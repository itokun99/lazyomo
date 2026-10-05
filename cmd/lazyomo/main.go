package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/itokun99/lazyomo/internal/editor"
	"github.com/itokun99/lazyomo/internal/tui"
)

const usage = `lazyomo - editor for ~/.omo/omo.jsonc

Usage:
  lazyomo           open the interactive editor
  lazyomo --help    print this help
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 {
		if args[0] == "--help" || args[0] == "-h" {
			fmt.Fprint(os.Stdout, usage)
			return nil
		}
		return fmt.Errorf("unknown args %q\n\n%s", args, usage)
	}
	ed, err := editor.LoadDefault()
	if err != nil {
		return err
	}
	p := tea.NewProgram(tui.New(ed), tea.WithAltScreen())
	_, err = p.Run()
	return err
}
