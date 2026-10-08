package common

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/mikeryanboss/vineyard/internal/session"
)

// Status icons, one visible cell wide.
const (
	IconRunning = "◑"
	IconReady   = "●"
	IconLoading = "◌"
	IconPaused  = "○"
	IconStopped = "×"
	IconBranch  = "⑂"
)

// palette is the set of colours a theme is built from. The base colours match
// grapes, so the two tools look like one family side by side.
type palette struct {
	Accent, AccentBg, Border, Text, Muted, Faint, Surface string
	Running, Ready, Loading, Paused, Stopped              string
	Error                                                 string
	AddFg, AddBg, AddEmph, DelFg, DelBg, DelEmph          string
	// SyntaxStyle names the Chroma style for code in diffs.
	SyntaxStyle string
}

var darkPalette = palette{
	Accent: "#a371f7", AccentBg: "#2d1b69", Border: "#30363d", Text: "#e6edf3",
	Muted: "#8b949e", Faint: "#484f58", Surface: "#161b22",
	Running: "#d29922", Ready: "#3fb950", Loading: "#388bfd", Paused: "#8b949e", Stopped: "#6e7681",
	Error: "#f85149",
	AddFg: "#3fb950", AddBg: "#12261e", AddEmph: "#1f5130",
	DelFg: "#f85149", DelBg: "#2d1517", DelEmph: "#6b2a2c",
	SyntaxStyle: "github-dark",
}

var lightPalette = palette{
	Accent: "#8250df", AccentBg: "#eddeff", Border: "#d0d7de", Text: "#1f2328",
	Muted: "#656d76", Faint: "#afb8c1", Surface: "#f6f8fa",
	Running: "#9a6700", Ready: "#1a7f37", Loading: "#0969da", Paused: "#656d76", Stopped: "#8c959f",
	Error: "#cf222e",
	AddFg: "#1a7f37", AddBg: "#e6ffec", AddEmph: "#abf2bc",
	DelFg: "#cf222e", DelBg: "#ffebe9", DelEmph: "#ffc1bc",
	SyntaxStyle: "github",
}

// Theme holds the colours and prebuilt styles for the TUI.
type Theme struct {
	IsDark bool

	ColorAccent  color.Color
	ColorText    color.Color
	ColorMuted   color.Color
	ColorFaint   color.Color
	ColorBorder  color.Color
	ColorSurface color.Color
	ColorError   color.Color

	ColorAddFg   color.Color
	ColorAddBg   color.Color
	ColorAddEmph color.Color
	ColorDelFg   color.Color
	ColorDelBg   color.Color
	ColorDelEmph color.Color
	SyntaxStyle  string

	statusColors map[session.Status]color.Color

	StyleAppTitle    lipgloss.Style
	StyleTabActive   lipgloss.Style
	StyleTabInactive lipgloss.Style
	StyleSeparator   lipgloss.Style
	StyleStatusBar   lipgloss.Style
	StyleStatusKey   lipgloss.Style
	StyleTitle       lipgloss.Style
	StyleSubtitle    lipgloss.Style
	StyleFaint       lipgloss.Style
	StyleSelected    lipgloss.Style
	StylePane        lipgloss.Style
	StylePaneFocused lipgloss.Style
	StyleDialog      lipgloss.Style
	StyleError       lipgloss.Style
	StyleFileHeader  lipgloss.Style
	StyleHunkHeader  lipgloss.Style
	StyleBadge       lipgloss.Style
}

// NewTheme builds the theme for a dark or light terminal background.
func NewTheme(isDark bool) Theme {
	p := lightPalette
	if isDark {
		p = darkPalette
	}
	c := func(hex string) color.Color { return lipgloss.Color(hex) }

	t := Theme{
		IsDark:       isDark,
		ColorAccent:  c(p.Accent),
		ColorText:    c(p.Text),
		ColorMuted:   c(p.Muted),
		ColorFaint:   c(p.Faint),
		ColorBorder:  c(p.Border),
		ColorSurface: c(p.Surface),
		ColorError:   c(p.Error),
		ColorAddFg:   c(p.AddFg),
		ColorAddBg:   c(p.AddBg),
		ColorAddEmph: c(p.AddEmph),
		ColorDelFg:   c(p.DelFg),
		ColorDelBg:   c(p.DelBg),
		ColorDelEmph: c(p.DelEmph),
		SyntaxStyle:  p.SyntaxStyle,
		statusColors: map[session.Status]color.Color{
			session.StatusRunning: c(p.Running),
			session.StatusReady:   c(p.Ready),
			session.StatusLoading: c(p.Loading),
			session.StatusPaused:  c(p.Paused),
			session.StatusStopped: c(p.Stopped),
		},
	}

	t.StyleAppTitle = lipgloss.NewStyle().Bold(true).Foreground(t.ColorAccent).Padding(0, 1, 0, 2)
	t.StyleTabActive = lipgloss.NewStyle().Bold(true).Foreground(t.ColorAccent).Background(c(p.AccentBg)).Padding(0, 1)
	t.StyleTabInactive = lipgloss.NewStyle().Foreground(t.ColorMuted).Padding(0, 1)
	t.StyleSeparator = lipgloss.NewStyle().Foreground(t.ColorBorder)
	t.StyleStatusBar = lipgloss.NewStyle().Background(t.ColorSurface).Foreground(t.ColorMuted).Padding(0, 1)
	t.StyleStatusKey = lipgloss.NewStyle().Foreground(t.ColorText).Bold(true)
	t.StyleTitle = lipgloss.NewStyle().Bold(true).Foreground(t.ColorText)
	t.StyleSubtitle = lipgloss.NewStyle().Foreground(t.ColorMuted)
	t.StyleFaint = lipgloss.NewStyle().Foreground(t.ColorFaint)
	t.StyleSelected = lipgloss.NewStyle().Background(c(p.AccentBg))
	t.StylePane = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorBorder)
	t.StylePaneFocused = t.StylePane.BorderForeground(t.ColorAccent)
	t.StyleDialog = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(t.ColorAccent).Padding(1, 2)
	t.StyleError = lipgloss.NewStyle().Foreground(t.ColorError)
	t.StyleFileHeader = lipgloss.NewStyle().Bold(true).Foreground(t.ColorText).Background(t.ColorSurface)
	t.StyleHunkHeader = lipgloss.NewStyle().Foreground(t.ColorAccent)
	t.StyleBadge = lipgloss.NewStyle().Foreground(t.ColorMuted).Background(t.ColorSurface).Padding(0, 1)
	return t
}

// StatusColor returns the colour for a session status.
func (t Theme) StatusColor(s session.Status) color.Color {
	if c, ok := t.statusColors[s]; ok {
		return c
	}
	return t.ColorMuted
}

// StatusIcon returns the icon for a session status.
func StatusIcon(s session.Status) string {
	switch s {
	case session.StatusRunning:
		return IconRunning
	case session.StatusReady:
		return IconReady
	case session.StatusLoading:
		return IconLoading
	case session.StatusPaused:
		return IconPaused
	default:
		return IconStopped
	}
}

// FormatKeyHint renders a key and its action for the status bar.
func (t Theme) FormatKeyHint(k, action string) string {
	return t.StyleStatusKey.Render(k) + " " + action
}
