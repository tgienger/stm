package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tgienger/stm/internal/fizzy"
	"github.com/tgienger/stm/internal/ui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("stm %s (commit: %s, built: %s)\n", version, commit, date)
		os.Exit(0)
	}

	client, err := fizzy.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	settings, err := fizzy.NewSettings()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading settings: %v\n", err)
		os.Exit(1)
	}

	app := ui.NewApp(client, settings)
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running application: %v\n", err)
		os.Exit(1)
	}
}
