package styles

import (
	"github.com/charmbracelet/lipgloss"
)

// Theme represents a color scheme for the application
type Theme struct {
	Name string

	// Base colors
	Background    lipgloss.Color
	Foreground    lipgloss.Color
	ForegroundDim lipgloss.Color

	// Accent colors
	Primary   lipgloss.Color
	Secondary lipgloss.Color
	Accent    lipgloss.Color

	// Semantic colors
	Success lipgloss.Color
	Warning lipgloss.Color
	Error   lipgloss.Color
	Info    lipgloss.Color

	// UI element colors
	Border      lipgloss.Color
	BorderFocus lipgloss.Color
	Selection   lipgloss.Color
	Cursor      lipgloss.Color
}

// TokyoNight is the default color theme
var TokyoNight = Theme{
	Name: "Tokyo Night",

	Background:    lipgloss.Color("#1a1b26"),
	Foreground:    lipgloss.Color("#c0caf5"),
	ForegroundDim: lipgloss.Color("#565f89"),

	Primary:   lipgloss.Color("#7aa2f7"),
	Secondary: lipgloss.Color("#bb9af7"),
	Accent:    lipgloss.Color("#7dcfff"),

	Success: lipgloss.Color("#9ece6a"),
	Warning: lipgloss.Color("#e0af68"),
	Error:   lipgloss.Color("#f7768e"),
	Info:    lipgloss.Color("#7aa2f7"),

	Border:      lipgloss.Color("#3b4261"),
	BorderFocus: lipgloss.Color("#7aa2f7"),
	Selection:   lipgloss.Color("#33467c"),
	Cursor:      lipgloss.Color("#c0caf5"),
}

// Current holds the active theme
var Current = TokyoNight

// MaxWidth is the maximum content width for the app (classic terminal width)
const MaxWidth = 80

// ContentWidth returns the actual content width to use (min of terminal width and MaxWidth)
func ContentWidth(terminalWidth int) int {
	if terminalWidth > MaxWidth {
		return MaxWidth
	}
	return terminalWidth
}

// CenterView wraps content and centers it horizontally if terminal is wider than MaxWidth
func CenterView(content string, terminalWidth, terminalHeight int) string {
	if terminalWidth <= MaxWidth {
		return content
	}
	return lipgloss.Place(terminalWidth, terminalHeight,
		lipgloss.Center, lipgloss.Top,
		content,
	)
}

// Styles holds all the pre-computed styles for the UI
type Styles struct {
	// App container
	App lipgloss.Style

	// Title bar
	TitleBar   lipgloss.Style
	Title      lipgloss.Style
	TitleMuted lipgloss.Style

	// Lists
	List         lipgloss.Style
	ListItem     lipgloss.Style
	ListSelected lipgloss.Style

	// Filter bar
	FilterBar    lipgloss.Style
	FilterInput  lipgloss.Style
	FilterButton lipgloss.Style

	// Buttons
	Button        lipgloss.Style
	ButtonFocused lipgloss.Style
	ButtonPrimary lipgloss.Style

	// Tags
	Tag lipgloss.Style

	// Task item
	TaskItem     lipgloss.Style
	TaskTitle    lipgloss.Style
	TaskPriority lipgloss.Style

	// Input fields
	Input        lipgloss.Style
	InputFocused lipgloss.Style

	// Help text
	Help     lipgloss.Style
	HelpKey  lipgloss.Style
	HelpDesc lipgloss.Style

	// Status bar
	StatusBar lipgloss.Style
}

// NewStyles creates styles based on the current theme
func NewStyles() *Styles {
	t := Current

	return &Styles{
		App: lipgloss.NewStyle().
			Background(t.Background).
			Foreground(t.Foreground),

		TitleBar: lipgloss.NewStyle().
			Foreground(t.Foreground).
			Background(t.Background).
			Padding(0, 1).
			Bold(true),

		Title: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		TitleMuted: lipgloss.NewStyle().
			Foreground(t.ForegroundDim),

		List: lipgloss.NewStyle().
			Padding(1, 2),

		ListItem: lipgloss.NewStyle().
			Foreground(t.Foreground).
			Padding(0, 2),

		ListSelected: lipgloss.NewStyle().
			Foreground(t.Primary).
			Background(t.Selection).
			Padding(0, 2).
			Bold(true),

		FilterBar: lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border),

		FilterInput: lipgloss.NewStyle().
			Foreground(t.Foreground).
			Padding(0, 1),

		FilterButton: lipgloss.NewStyle().
			Foreground(t.ForegroundDim).
			Padding(0, 1),

		Button: lipgloss.NewStyle().
			Foreground(t.Foreground).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(0, 2),

		ButtonFocused: lipgloss.NewStyle().
			Foreground(t.Primary).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderFocus).
			Padding(0, 2).
			Bold(true),

		ButtonPrimary: lipgloss.NewStyle().
			Foreground(t.Background).
			Background(t.Primary).
			Padding(0, 2).
			Bold(true),

		Tag: lipgloss.NewStyle().
			Padding(0, 1).
			MarginRight(1),

		TaskItem: lipgloss.NewStyle().
			Padding(0, 1),

		TaskTitle: lipgloss.NewStyle().
			Foreground(t.Foreground),

		TaskPriority: lipgloss.NewStyle().
			Foreground(t.Warning).
			Bold(true),

		Input: lipgloss.NewStyle().
			Foreground(t.Foreground).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(0, 1),

		InputFocused: lipgloss.NewStyle().
			Foreground(t.Foreground).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderFocus).
			Padding(0, 1),

		Help: lipgloss.NewStyle().
			Foreground(t.ForegroundDim).
			Padding(1, 2),

		HelpKey: lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true),

		HelpDesc: lipgloss.NewStyle().
			Foreground(t.ForegroundDim),

		StatusBar: lipgloss.NewStyle().
			Foreground(t.ForegroundDim).
			Padding(0, 1),
	}
}
