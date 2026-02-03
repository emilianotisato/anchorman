package screens

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NavigateMsg is sent when navigation to another screen is requested
type NavigateMsg struct {
	Screen    string
	CompanyID *int64
	ProjectID *int64
}

func Navigate(screen string) tea.Cmd {
	return func() tea.Msg {
		return NavigateMsg{Screen: screen}
	}
}

func NavigateWithCompany(screen string, companyID int64) tea.Cmd {
	return func() tea.Msg {
		return NavigateMsg{Screen: screen, CompanyID: &companyID}
	}
}

func NavigateWithProject(screen string, projectID int64) tea.Cmd {
	return func() tea.Msg {
		return NavigateMsg{Screen: screen, ProjectID: &projectID}
	}
}

// RefreshMsg is sent when data should be refreshed
type RefreshMsg struct{}

func Refresh() tea.Cmd {
	return func() tea.Msg {
		return RefreshMsg{}
	}
}

// Styles
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginBottom(1)

	HelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			MarginTop(1)

	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212"))

	NormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	DimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42"))

	WarningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("214"))

	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196"))

	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(1, 2)
)
