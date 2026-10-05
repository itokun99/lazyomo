package tui

import "github.com/charmbracelet/lipgloss"

// Styles holds every lipgloss style for the TUI. Colors extend the
// previous palette: green marks focus, blue marks selection, gray
// marks unfocused chrome, red marks errors and dirty dots.
type Styles struct {
	Title    lipgloss.Style
	Help     lipgloss.Style
	ErrorMsg lipgloss.Style
	Status   lipgloss.Style
	Keybar   lipgloss.Style

	FocusBorder lipgloss.Style
	BlurBorder  lipgloss.Style
	PaneTitle   lipgloss.Style
	Selected    lipgloss.Style
	Normal      lipgloss.Style
	DirtyDot    lipgloss.Style
	OverlayBox  lipgloss.Style
}

// DefaultStyles returns the default styles.
func DefaultStyles() Styles {
	focused := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("42")).
		Bold(true)
	blurred := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("241"))
	return Styles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("60")),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")),
		ErrorMsg: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196")),
		Status: lipgloss.NewStyle().
			Padding(0, 1).
			Background(lipgloss.Color("60")).
			Foreground(lipgloss.Color("230")),
		Keybar: lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")),
		FocusBorder: focused,
		BlurBorder:  blurred,
		PaneTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("229")),
		Selected: lipgloss.NewStyle().
			Background(lipgloss.Color("27")).
			Foreground(lipgloss.Color("230")),
		Normal: lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")),
		DirtyDot: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196")),
		OverlayBox: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("42")).
			Padding(1, 2),
	}
}
