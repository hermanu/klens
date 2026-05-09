package resources

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

// LogSvc implements port.LogService against client-go.
type LogSvc struct {
	kube kubernetes.Interface
}

func NewLogSvc(kube kubernetes.Interface) LogSvc {
	return LogSvc{kube: kube}
}

// scanBufMax caps a single log line at 1 MiB; container runtimes already
// bound stdout lines but JSON-encoded payloads can be long, so we lift the
// default 64 KiB Scanner limit.
const scanBufMax = 1024 * 1024

// fallbackTailLines caps the backlog when no since-window is supplied. 2000
// is plenty of context for "show me everything you have buffered" while
// staying well under the typical kubelet log retention size.
const fallbackTailLines int64 = 2000

// StreamPodLogs streams container logs to `out`. `sinceSeconds` is the
// lookback window — pass 1800 for the last 30 min. Pass 0 for "no since",
// in which case we fall back to a tail-line cap so we don't replay days of
// logs on a busy pod.
func (s LogSvc) StreamPodLogs(ctx context.Context, namespace, pod, container string, sinceSeconds int64, out chan<- LogLine) error {
	opts := &corev1.PodLogOptions{
		Container:  container,
		Follow:     true,
		Timestamps: true,
	}
	switch {
	case sinceSeconds > 0:
		s := sinceSeconds
		opts.SinceSeconds = &s
	default:
		opts.TailLines = ptr.To(fallbackTailLines)
	}
	req := s.kube.CoreV1().Pods(namespace).GetLogs(pod, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = stream.Close() }()

	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, 64*1024), scanBufMax)

	for scanner.Scan() {
		line := parseLogLine(scanner.Text(), time.Now())
		line.Pod = pod
		line.Container = container
		select {
		case out <- line:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	// scanner.Err() returns nil at EOF; ctx cancellation usually surfaces here too.
	return scanner.Err()
}

// StreamPodLogsMulti fans out one stream per pod into the shared `out` channel.
// Per-pod errors are logged to stderr but do not terminate sibling streams; the
// first non-nil error is surfaced once ctx ends. Returns nil when no pods.
func (s LogSvc) StreamPodLogsMulti(ctx context.Context, ns string, pods []string, since int64, out chan<- LogLine) error {
	if len(pods) == 0 {
		return nil
	}
	if len(pods) == 1 {
		return s.StreamPodLogs(ctx, ns, pods[0], "", since, out)
	}
	var wg sync.WaitGroup
	errCh := make(chan error, len(pods))
	for _, p := range pods {
		wg.Add(1)
		go func(podName string) {
			defer wg.Done()
			if err := s.StreamPodLogs(ctx, ns, podName, "", since, out); err != nil && ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "log stream for pod %s/%s ended: %v\n", ns, podName, err)
				errCh <- err
			}
		}(p)
	}
	wg.Wait()
	close(errCh)
	for e := range errCh {
		return e
	}
	return ctx.Err()
}

// parseLogLine splits a kubelet `Timestamps: true` log line into its
// timestamp prefix and message, then makes a best-effort guess at log level.
// Exposed for direct unit testing because the fake clientset's GetLogs body
// does not let us drive realistic input through the streaming path.
func parseLogLine(raw string, fallback time.Time) LogLine {
	ts := fallback
	msg := raw

	// kubelet emits "<RFC3339Nano> <message>". If the prefix parses, peel it off.
	if idx := strings.IndexByte(raw, ' '); idx > 0 {
		if parsed, err := time.Parse(time.RFC3339Nano, raw[:idx]); err == nil {
			ts = parsed
			msg = raw[idx+1:]
		}
	}

	return LogLine{
		Time:    ts,
		Level:   detectLevel(msg),
		Message: msg,
	}
}

// detectLevel scans the first 32 bytes for a common level token. Anything
// past 32 bytes is almost always payload, not a level prefix, so bounding
// the search avoids matching the word "ERROR" inside a stack-trace tail.
func detectLevel(msg string) string {
	head := msg
	if len(head) > 32 {
		head = head[:32]
	}
	upper := strings.ToUpper(head)
	for _, lvl := range []string{"ERROR", "WARN", "INFO", "DEBUG"} {
		if strings.Contains(upper, lvl) {
			return lvl
		}
	}
	return ""
}
