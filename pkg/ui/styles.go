package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Campfire Color Palette
	ColorAmber   = lipgloss.Color("#F59E0B") // Primary brand amber/fire
	ColorFlame   = lipgloss.Color("#EF4444") // Red / alert
	ColorEmber   = lipgloss.Color("#EA580C") // Orange-red
	ColorSmoke   = lipgloss.Color("#94A3B8") // Slate gray / muted
	ColorAsh     = lipgloss.Color("#64748B") // Darker gray
	ColorCharcoal= lipgloss.Color("#1E293B") // Deep background
	ColorForest  = lipgloss.Color("#10B981") // Success green
	ColorSky     = lipgloss.Color("#38BDF8") // Cyan / info

	// Lipgloss Styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorAmber)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSmoke)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(ColorForest)

	WarningStyle = lipgloss.NewStyle().
			Foreground(ColorAmber)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ColorFlame).
			Bold(true)

	MutedStyle = lipgloss.NewStyle().
			Foreground(ColorSmoke)

	BadgeReady = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ColorForest).
			Padding(0, 1).
			Bold(true)

	BadgePending = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ColorAmber).
			Padding(0, 1).
			Bold(true)

	BadgeTerminating = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ColorSmoke).
			Padding(0, 1)

	BadgeFailed = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ColorFlame).
			Padding(0, 1).
			Bold(true)

	BadgeStopped = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ColorSmoke).
			Padding(0, 1).
			Bold(true)
)

// StatusBadge formats a state into a colorful badge
func StatusBadge(status string) string {
	switch strings.ToLower(status) {
	case "ready", "running":
		return BadgeReady.Render("RUNNING")
	case "pending", "starting":
		return BadgePending.Render("STARTING")
	case "stopped", "suspended":
		return BadgeStopped.Render("STOPPED")
	case "terminating":
		return BadgeTerminating.Render("TERMINATING")
	default:
		return BadgeFailed.Render(strings.ToUpper(status))
	}
}

// Banner returns the Campfire ASCII art / title
func Banner() string {
	return TitleStyle.Render("🔥 campfire")
}

// Info prints a campfire info line
func Info(format string, a ...interface{}) {
	fmt.Printf("%s %s\n", TitleStyle.Render("•"), fmt.Sprintf(format, a...))
}

// Success prints a success message
func Success(format string, a ...interface{}) {
	fmt.Printf("%s %s\n", SuccessStyle.Render("✓"), fmt.Sprintf(format, a...))
}

// Error prints an error message
func Error(format string, a ...interface{}) {
	fmt.Printf("%s %s\n", ErrorStyle.Render("✗"), fmt.Sprintf(format, a...))
}
