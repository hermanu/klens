package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hermanu/klens/app"
	k8sclient "github.com/hermanu/klens/k8s"
)

// Set by GoReleaser at build time via -ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		kubeconfig  = flag.String("kubeconfig", "", "Path to kubeconfig (overrides config file and KUBECONFIG env var)")
		namespace   = flag.String("namespace", "", "Default namespace (overrides config file)")
		showVersion = flag.Bool("version", false, "Print version and exit")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("klens %s (commit %s, built %s)\n", version, commit, date)
		return
	}

	m, err := app.New(*kubeconfig, *namespace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if m.Client() != nil {
		w := k8sclient.NewWatcher(m.Client(), m.Namespace(), p)
		w.Start()
		defer w.Stop()
	}
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
