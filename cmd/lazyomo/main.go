package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/itokun99/lazyomo/internal/mcpfile"
	"github.com/itokun99/lazyomo/internal/tui"
	"github.com/itokun99/lazyomo/internal/workspace"
)

// Compile-time wiring contracts: the real registry is the TUI's workspace,
// and the mcpfile session plugs into the registry's session seam.
var (
	_ tui.Workspace     = (*workspace.Registry)(nil)
	_ workspace.Session = (*mcpfile.Session)(nil)
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
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolving home directory: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving working directory: %w", err)
	}
	agentDir, err := mcpfile.ResolveAgentDir()
	if err != nil {
		return err
	}
	reg := workspace.NewRegistry(workspace.RegistryConfig{HomeDir: home, Cwd: cwd, AgentDir: agentDir})
	if err := requireUserConfig(reg); err != nil {
		return err
	}
	if err := attachMCPSession(reg); err != nil {
		return err
	}
	p := tea.NewProgram(tui.New(reg), tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// requireUserConfig refuses to start without an editable user config: a
// missing or invalid ~/.omo/omo.jsonc is fatal, exactly as before the
// workspace layer. Every other layer is optional and stays a diagnostic.
func requireUserConfig(reg *workspace.Registry) error {
	for _, diagnostic := range reg.Diagnostics() {
		if diagnostic.SourceID == workspace.IDUser {
			return fmt.Errorf("loading user config: %s", diagnostic.Reason)
		}
	}
	return nil
}

// attachMCPSession wires the registered mcp.json source to its editing
// session. A malformed mcp.json leaves the source registered without a
// session for this run; the operator sees the warning on stderr.
func attachMCPSession(reg *workspace.Registry) error {
	source, ok := reg.Source(workspace.IDMCP)
	if !ok {
		return nil
	}
	session, err := mcpfile.Load(source.Path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v; MCP surface disabled\n", err)
		return nil
	}
	return reg.AttachSession(workspace.IDMCP, session)
}
