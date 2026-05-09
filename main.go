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

	if m.Client() != nil {
		w := k8sclient.NewWatcher(m.Client(), m.Namespace(), p, m.Metrics(), m.Logs())
		w.Start()
		defer w.Stop()
		// Hand the watcher's log-streaming entry point to the model so the
		// pods view can request a tail when the user presses `l` / Enter
		// without importing client-go from views.
		m.SetLogTailStarter(w.StartPodLogTails)
	}
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	_ = version
	_ = commit
	_ = date
}
