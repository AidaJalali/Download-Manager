package styles

import "github.com/charmbracelet/lipgloss"

var (
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Background(lipgloss.Color("#1E1E1E")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF0000"))

	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Background(lipgloss.Color("#1E1E1E")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00FF00"))

	InfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FFFF")).
			Background(lipgloss.Color("#1E1E1E")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#00FFFF"))
)
