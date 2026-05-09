package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/manu/klens/app"
	k8sclient "github.com/manu/klens/k8s"
)

func main() {
	m, err := app.New()
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
