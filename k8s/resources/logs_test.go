package resources

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestLogSvc_StreamPodLogs_DeliversAtLeastOneLine(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"},
	})

	out := make(chan LogLine, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- NewLogSvc(cs).StreamPodLogs(ctx, "ns", "p", "", 0, out)
	}()

	select {
	case line := <-out:
		if line.Pod != "p" {
			t.Errorf("want Pod=p, got %q", line.Pod)
		}
		if line.Message == "" {
			t.Errorf("want non-empty message")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no log line received within 2s")
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("StreamPodLogs did not return after cancel")
	}
}

func TestParseLogLine_StripsTimestampAndDetectsLevel(t *testing.T) {
	ts := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	raw := ts.Format(time.RFC3339Nano) + " ERROR something exploded"

	got := parseLogLine(raw, time.Now())

	if !got.Time.Equal(ts) {
		t.Errorf("want Time=%v, got %v", ts, got.Time)
	}
	if got.Level != "ERROR" {
		t.Errorf("want Level=ERROR, got %q", got.Level)
	}
	if got.Message != "ERROR something exploded" {
		t.Errorf("unexpected message: %q", got.Message)
	}
}

func TestParseLogLine_NoTimestampUsesFallback(t *testing.T) {
	fallback := time.Date(2030, 5, 1, 12, 0, 0, 0, time.UTC)

	got := parseLogLine("plain line with no prefix", fallback)

	if !got.Time.Equal(fallback) {
		t.Errorf("want fallback Time=%v, got %v", fallback, got.Time)
	}
	if got.Level != "" {
		t.Errorf("want empty level, got %q", got.Level)
	}
	if got.Message != "plain line with no prefix" {
		t.Errorf("unexpected message: %q", got.Message)
	}
}

func TestParseLogLine_LevelDetectionIsBounded(t *testing.T) {
	// "ERROR" appearing past byte 32 must not register as a level — that
	// avoids classifying stack-trace tails as ERROR rows.
	raw := "queued job ok with status code 200, ERROR was reported elsewhere"
	got := parseLogLine(raw, time.Now())
	if got.Level != "" {
		t.Errorf("want empty level for late ERROR token, got %q", got.Level)
	}
}

func TestParseLogLine_PrefersHigherSeverity(t *testing.T) {
	// "INFO" appears first, but the head-window scan iterates ERROR first
	// to surface the most actionable level when both fit in 32 bytes.
	raw := "INFO ERROR mixed case sample"
	got := parseLogLine(raw, time.Now())
	if got.Level != "ERROR" {
		t.Errorf("want ERROR (highest severity), got %q", got.Level)
	}
}

func TestLogSvc_StreamPodLogsMulti_FanOut(t *testing.T) {
	// Two pods, one fan-out call → both pods' lines must arrive on the same
	// channel. The fake clientset's GetLogs body returns a single canned line
	// per pod, so we expect exactly two LogLine values, one per source pod.
	cs := fake.NewSimpleClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "ns"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p2", Namespace: "ns"}},
	)

	out := make(chan LogLine, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- NewLogSvc(cs).StreamPodLogsMulti(ctx, "ns", []string{"p1", "p2"}, 0, out)
	}()

	seen := map[string]bool{}
	deadline := time.After(2 * time.Second)
	for len(seen) < 2 {
		select {
		case line := <-out:
			seen[line.Pod] = true
		case <-deadline:
			t.Fatalf("only saw lines from %v within 2s", seen)
		}
	}
	if !seen["p1"] || !seen["p2"] {
		t.Errorf("want lines from p1 and p2, got %v", seen)
	}

	cancel()
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("StreamPodLogsMulti did not return after cancel")
	}
}
