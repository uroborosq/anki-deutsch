// Package tui is the Bubble Tea terminal UI for the deckbuilder. It depends only
// on the usecase.Service inbound port and never imports the adapters.
package tui

import "github.com/charmbracelet/lipgloss"

// Styles centralizes every lipgloss style used by the UI. View pulls colors from
// here exclusively — there are no inline colors anywhere in model.go. Every color
// is an AdaptiveColor so the palette stays legible on dark and light terminals.
type Styles struct {
	// Semantic feedback styles.
	OK      lipgloss.Style // green: success
	Err     lipgloss.Style // red: failure
	Warn    lipgloss.Style // yellow: skipped/warning
	Skipped lipgloss.Style // yellow: a skipped suggestion

	// Grammatical-gender colors for German articles.
	Der lipgloss.Style // masculine
	Die lipgloss.Style // feminine
	Das lipgloss.Style // neuter

	// Accents.
	Title       lipgloss.Style // screen titles
	Selected    lipgloss.Style // selected list item
	Item        lipgloss.Style // unselected list item
	ActiveInput lipgloss.Style // focused text input frame
	KeyHint     lipgloss.Style // key/binding hints in the footer
	Faint       lipgloss.Style // de-emphasized helper text
	Checkbox    lipgloss.Style // [x]/[ ] checkbox marker
}

// Palette of AdaptiveColors. Each is {dark, light} tuned for contrast.
var (
	colGreen  = lipgloss.AdaptiveColor{Dark: "#73F59F", Light: "#08872B"}
	colRed    = lipgloss.AdaptiveColor{Dark: "#FF6B6B", Light: "#C8102E"}
	colYellow = lipgloss.AdaptiveColor{Dark: "#F5D76E", Light: "#9A6700"}
	colBlue   = lipgloss.AdaptiveColor{Dark: "#7AA2F7", Light: "#1F6FEB"} // der
	colPink   = lipgloss.AdaptiveColor{Dark: "#FF8FD9", Light: "#BF2C8C"} // die
	colTeal   = lipgloss.AdaptiveColor{Dark: "#5BD6C8", Light: "#0B7A6E"} // das
	colPurple = lipgloss.AdaptiveColor{Dark: "#C792EA", Light: "#6F42C1"} // title accent
	colFaint  = lipgloss.AdaptiveColor{Dark: "#8A8FA3", Light: "#6E7178"}
)

// DefaultStyles builds the colorful theme.
func DefaultStyles() Styles {
	return Styles{
		OK:      lipgloss.NewStyle().Foreground(colGreen).Bold(true),
		Err:     lipgloss.NewStyle().Foreground(colRed).Bold(true),
		Warn:    lipgloss.NewStyle().Foreground(colYellow),
		Skipped: lipgloss.NewStyle().Foreground(colYellow).Italic(true),

		Der: lipgloss.NewStyle().Foreground(colBlue).Bold(true),
		Die: lipgloss.NewStyle().Foreground(colPink).Bold(true),
		Das: lipgloss.NewStyle().Foreground(colTeal).Bold(true),

		Title: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(colPurple).
			Bold(true).
			Padding(0, 1),
		Selected: lipgloss.NewStyle().
			Foreground(colPurple).
			Bold(true),
		Item: lipgloss.NewStyle().
			Foreground(lipgloss.AdaptiveColor{Dark: "#D5D8E0", Light: "#3A3D45"}),
		ActiveInput: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colPurple).
			Padding(0, 1),
		KeyHint: lipgloss.NewStyle().Foreground(colFaint).Bold(true),
		Faint:   lipgloss.NewStyle().Foreground(colFaint),
		Checkbox: lipgloss.NewStyle().
			Foreground(colGreen).
			Bold(true),
	}
}

// Gender returns the style for a German article ("der"/"die"/"das"), falling
// back to the neutral item style for anything else.
func (s Styles) Gender(article string) lipgloss.Style {
	switch article {
	case "der":
		return s.Der
	case "die":
		return s.Die
	case "das":
		return s.Das
	default:
		return s.Item
	}
}
