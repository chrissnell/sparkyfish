package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	sf "github.com/chrissnell/sparkyfish/pkg/backend/sparkyfish"
	"github.com/chrissnell/sparkyfish/pkg/tui"
)

var version = "dev"

const defaultPort = "7121"

func main() {
	if len(os.Args) >= 2 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		fmt.Println("sparkyfish", version)
		os.Exit(0)
	}

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <hostname>[:port]\n", os.Args[0])
		os.Exit(1)
	}

	addr := os.Args[1]
	if !strings.Contains(addr, ":") {
		addr = addr + ":" + defaultPort
	}

	client := sf.New()
	model := tui.New(client, addr)

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
