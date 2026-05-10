package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-logr/logr"
	"github.com/hermanu/klens/app"
	k8sclient "github.com/hermanu/klens/k8s"
	"github.com/hermanu/klens/port"
	"k8s.io/klog/v2"
)

// Set by GoReleaser at build time via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Silence klog — client-go's informer and trace packages otherwise dump
	// "Reflector ListAndWatch (total time: 24s)" traces to stderr at I-level
	// during slow list operations, which corrupts the Bubble Tea alt-screen.
	// Three layers, all needed:
	//  1. SetLogger(Discard) — silences klog.Background() / contextual logger
	//     which is what the `trace` package uses;
	//  2. SetOutput(io.Discard) — silences direct klog.Info / Errorf calls;
	//  3. -logtostderr=false / -v=0 — flag-driven defaults still applied for
	//     any code that re-reads them.
	klog.SetLogger(logr.Discard())
	klog.SetOutput(io.Discard)
	klog.LogToStderr(false)
	klogFlags := flag.NewFlagSet("klog", flag.ContinueOnError)
	klog.InitFlags(klogFlags)
	_ = klogFlags.Parse([]string{
		"-logtostderr=false",
		"-alsologtostderr=false",
		"-stderrthreshold=FATAL",
		"-v=0",
	})

	m, err := app.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Hard-kill safety net — Bubble Tea translates the first SIGINT to a
	// QuitMsg, but if Update is wedged on a slow client-go ListAndWatch the
	// QuitMsg never gets processed. A second ctrl+c (or 5s elapsed) force-
	// exits unconditionally so the user is never trapped in the alt-screen.
	go func() {
		ch := make(chan os.Signal, 2)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch // first signal — let bubbletea try to clean up
		select {
		case <-ch:
		case <-time.After(5 * time.Second):
		}
		// Restore the main screen before bailing so the terminal isn't left
		// in alt-screen mode.
		fmt.Fprint(os.Stderr, "\x1b[?1049l")
		os.Exit(130)
	}()

	// Watcher lifecycle is owned by main.go so context switches can swap it
	// without leaking informers. The restarter closure below captures `w` by
	// reference; both `defer Stop()` and runtime tear-downs read the latest
	// pointer, and Watcher.Stop is sync.Once-guarded against double-close.
	var w *k8sclient.Watcher
	startWatcher := func(client *k8sclient.Client, ns string, metrics port.MetricsService, logs port.LogService) {
		nw := k8sclient.NewWatcher(client, ns, p, metrics, logs)
		nw.Start()
		w = nw
		// Re-wire the log-tail entry point so `l` continues to find a live
		// streamer after the watcher is replaced.
		m.SetLogTailStarter(nw.StartPodLogTails)
	}
	if m.Client() != nil {
		startWatcher(m.Client(), m.Namespace(), m.Metrics(), m.Logs())
	}
	defer func() {
		if w != nil {
			w.Stop()
		}
	}()
	m.SetWatcherRestarter(func(client *k8sclient.Client, ns string, metrics port.MetricsService, logs port.LogService) {
		if w != nil {
			w.Stop()
		}
		startWatcher(client, ns, metrics, logs)
	})
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	_ = version
	_ = commit
	_ = date
}
