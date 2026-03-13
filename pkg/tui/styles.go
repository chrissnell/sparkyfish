package tui

import "github.com/charmbracelet/lipgloss"

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")) // white

	bannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("9")) // red

	latencyLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("10")) // green

	latencyStatsStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")) // white

	chartLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")) // green

	chartBorderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("7")) // white/gray

	summaryHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")) // white

	summaryValueStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("15")) // white

	progressLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("10")) // green

	progressBarActive = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("1")) // red bg

	progressBarDone = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("2")) // green bg

	progressBarEmpty = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(lipgloss.Color("0")) // dark

	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")) // dim gray

	helpStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")). // white
			Background(lipgloss.Color("1"))   // red
)
