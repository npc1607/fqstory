package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Catppuccin / Tokyo Night inspired rich palette
	colorTomato    = lipgloss.Color("#FF5252") // vibrant tomato red
	colorGold      = lipgloss.Color("#FFA116") // warm amber/gold
	colorCyan      = lipgloss.Color("#00CEC9") // cyber cyan
	colorMint      = lipgloss.Color("#2ECC71") // emerald mint
	colorPurple    = lipgloss.Color("#9C88FF") // neon lavender
	colorPink      = lipgloss.Color("#FF7675") // coral pink
	colorBlue      = lipgloss.Color("#487EB0") // slate blue
	colorWhite     = lipgloss.Color("#F5F6FA") // bright fg
	colorGray      = lipgloss.Color("#A4B0BE") // muted fg
	colorDarkGray  = lipgloss.Color("#57606F") // subtle dim
	colorCardBg    = lipgloss.Color("#1E1E2E") // panel background
	colorActiveBg  = lipgloss.Color("#2F3542") // active row background
	colorBorder    = lipgloss.Color("#3F4556") // inactive border
	colorBorderAct = lipgloss.Color("#FF5252") // active border

	// Aliases & shared styles for compatibility
	colorAccent    = colorCyan
	dimStyle       = lipgloss.NewStyle().Foreground(colorDarkGray)
	headerStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorWhite)
	highlightStyle = lipgloss.NewStyle().Bold(true).Foreground(colorWhite).Background(colorActiveBg).Padding(0, 1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorGray).
			Background(lipgloss.Color("#181825")).
			Padding(0, 1)

	statusKeyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorCyan)

	itemSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorCyan).
				PaddingLeft(1)

	itemNormalStyle = lipgloss.NewStyle().
			Foreground(colorGray).
			PaddingLeft(2)

	dialogBoxStyle = cardActiveBoxStyle

	// Brand Header styles
	badgeBrand = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorTomato).
			Padding(0, 1).
			MarginRight(1)

	headerTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorWhite)

	headerSubStyle = lipgloss.NewStyle().
			Foreground(colorDarkGray).
			MarginLeft(1)

	headerPillStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Background(lipgloss.Color("#262938")).
			Padding(0, 1).
			MarginLeft(1)

	// Container Card Box styles (Rounded borders)
	cardBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Background(colorCardBg).
			Padding(1, 1)

	cardActiveBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorCyan).
				Background(colorCardBg).
				Padding(1, 1)

	// Panel Titles
	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Padding(0, 1).
			MarginBottom(1)

	// Book Item Rows
	bookRowSelected = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorActiveBg).
			Padding(0, 1)

	bookRowNormal = lipgloss.NewStyle().
			Foreground(colorGray).
			Padding(0, 1)

	// Chips / Tags
	tagStyle = lipgloss.NewStyle().
			Foreground(colorWhite).
			Background(colorPurple).
			Padding(0, 1).
			MarginRight(1)

	scoreBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1e1e2e")).
			Background(colorGold).
			Padding(0, 1)

	readCountBadgeStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Background(lipgloss.Color("#182C38")).
				Padding(0, 1)

	// Progress bar styles
	progressFilled = lipgloss.NewStyle().Foreground(colorCyan)
	progressEmpty  = lipgloss.NewStyle().Foreground(colorDarkGray)

	// Bottom Key Caps Dock
	keyCapKey = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(lipgloss.Color("#373E4D")).
			Padding(0, 1)

	keyCapDesc = lipgloss.NewStyle().
			Foreground(colorGray).
			MarginRight(2)

	bossKeyBadge = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorTomato).
			Background(lipgloss.Color("#3B1D22")).
			Padding(0, 1)

	// Tab styles
	tabActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#1e1e2e")).
			Background(colorCyan).
			Padding(0, 1).
			MarginRight(1)

	tabInactive = lipgloss.NewStyle().
			Foreground(colorGray).
			Background(lipgloss.Color("#2b2d3c")).
			Padding(0, 1).
			MarginRight(1)
)

// renderProgressBar draws a smooth mini progress bar.
func renderProgressBar(ratio float64, width int) string {
	if width <= 0 {
		width = 10
	}
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}

	filledLen := int(ratio * float64(width))
	if filledLen > width {
		filledLen = width
	}
	emptyLen := width - filledLen

	filled := strings.Repeat("█", filledLen)
	empty := strings.Repeat("░", emptyLen)

	pctStr := fmt.Sprintf(" %4.1f%%", ratio*100)
	return progressFilled.Render(filled) + progressEmpty.Render(empty) + lipgloss.NewStyle().Foreground(colorCyan).Render(pctStr)
}
