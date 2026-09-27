package ui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// RevKeen terminal brand tokens (aligned with product green / ink surfaces).
var (
	ColorBrand   = lipgloss.Color("#0FA968")
	ColorInk     = lipgloss.Color("#0B1220")
	ColorMuted   = lipgloss.Color("#6B7280")
	ColorDanger  = lipgloss.Color("#DC2626")
	ColorWarn    = lipgloss.Color("#D97706")
	ColorSuccess = lipgloss.Color("#059669")
	ColorBorder  = lipgloss.Color("#374151")
	ColorFg      = lipgloss.Color("#E5E7EB")
)

var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorBrand)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	WarnStyle = lipgloss.NewStyle().
			Foreground(ColorWarn).
			Bold(true)

	DangerStyle = lipgloss.NewStyle().
			Foreground(ColorDanger).
			Bold(true)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	BorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	NavActiveStyle = lipgloss.NewStyle().
			Foreground(ColorBrand).
			Bold(true)

	NavIdleStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorFg).
			Background(ColorInk).
			Padding(0, 1)
)

// NoColor reports whether color should be disabled.
func NoColor(cmd *cobra.Command) bool {
	if cmd != nil {
		if v, err := cmd.Root().PersistentFlags().GetBool("no-color"); err == nil && v {
			return true
		}
		if v, err := cmd.Root().PersistentFlags().GetBool("agent"); err == nil && v {
			return true
		}
	}
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	return false
}

// Interactive reports whether styled/interactive UI is allowed.
func Interactive(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	root := cmd.Root().PersistentFlags()
	if agent, _ := root.GetBool("agent"); agent {
		return false
	}
	if jsonFlag, _ := root.GetBool("json"); jsonFlag {
		return false
	}
	if root.Changed("output") {
		format, _ := root.GetString("output")
		if format != "" && format != "table" {
			return false
		}
	}
	return isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
}

// ApplyNoColor disables lipgloss colors when needed.
func ApplyNoColor(cmd *cobra.Command) {
	if NoColor(cmd) {
		lipgloss.SetColorProfile(0) // ascii / no color
	}
}
