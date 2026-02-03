package tui

import (
	"database/sql"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/emilianohg/anchorman/internal/config"
	"github.com/emilianohg/anchorman/internal/tui/screens"
)

type Screen int

const (
	ScreenDashboard Screen = iota
	ScreenCompanies
	ScreenProjects
	ScreenRepos
	ScreenReports
	ScreenProcess
)

type App struct {
	db            *sql.DB
	cfg           *config.Config
	currentScreen Screen
	width         int
	height        int

	// Screen models
	dashboard *screens.Dashboard
	companies *screens.Companies
	projects  *screens.Projects
	repos     *screens.Repos
	reports   *screens.Reports
	process   *screens.Process

	// Navigation context
	selectedCompanyID *int64
	selectedProjectID *int64
}

func NewApp(db *sql.DB, cfg *config.Config) *App {
	return &App{
		db:            db,
		cfg:           cfg,
		currentScreen: ScreenDashboard,
	}
}

func (a *App) Init() tea.Cmd {
	a.dashboard = screens.NewDashboard(a.db)
	a.companies = screens.NewCompanies(a.db)
	a.projects = screens.NewProjects(a.db)
	a.repos = screens.NewRepos(a.db)
	a.reports = screens.NewReports(a.db, a.cfg)
	a.process = screens.NewProcess(a.db, a.cfg)

	return a.dashboard.Init()
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "q":
			if a.currentScreen == ScreenDashboard {
				return a, tea.Quit
			}
			// Let individual screens handle 'q' for going back
		}

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.dashboard.SetSize(msg.Width, msg.Height)
		a.companies.SetSize(msg.Width, msg.Height)
		a.projects.SetSize(msg.Width, msg.Height)
		a.repos.SetSize(msg.Width, msg.Height)
		a.reports.SetSize(msg.Width, msg.Height)
		a.process.SetSize(msg.Width, msg.Height)

	case screens.NavigateMsg:
		return a.handleNavigation(msg)
	}

	// Update current screen
	var cmd tea.Cmd
	switch a.currentScreen {
	case ScreenDashboard:
		cmd = a.dashboard.Update(msg)
	case ScreenCompanies:
		cmd = a.companies.Update(msg)
	case ScreenProjects:
		cmd = a.projects.Update(msg)
	case ScreenRepos:
		cmd = a.repos.Update(msg)
	case ScreenReports:
		cmd = a.reports.Update(msg)
	case ScreenProcess:
		cmd = a.process.Update(msg)
	}

	return a, cmd
}

func (a *App) handleNavigation(msg screens.NavigateMsg) (tea.Model, tea.Cmd) {
	switch msg.Screen {
	case "dashboard":
		a.currentScreen = ScreenDashboard
		a.selectedCompanyID = nil
		a.selectedProjectID = nil
		return a, a.dashboard.Init()
	case "companies":
		a.currentScreen = ScreenCompanies
		return a, a.companies.Init()
	case "projects":
		a.currentScreen = ScreenProjects
		a.selectedCompanyID = msg.CompanyID
		a.projects.SetCompanyFilter(msg.CompanyID)
		return a, a.projects.Init()
	case "repos":
		a.currentScreen = ScreenRepos
		a.selectedProjectID = msg.ProjectID
		a.repos.SetProjectFilter(msg.ProjectID)
		return a, a.repos.Init()
	case "reports":
		a.currentScreen = ScreenReports
		a.selectedCompanyID = msg.CompanyID
		a.reports.SetCompanyFilter(msg.CompanyID)
		return a, a.reports.Init()
	case "process":
		a.currentScreen = ScreenProcess
		return a, a.process.Init()
	}
	return a, nil
}

func (a *App) View() string {
	var content string

	switch a.currentScreen {
	case ScreenDashboard:
		content = a.dashboard.View()
	case ScreenCompanies:
		content = a.companies.View()
	case ScreenProjects:
		content = a.projects.View()
	case ScreenRepos:
		content = a.repos.View()
	case ScreenReports:
		content = a.reports.View()
	case ScreenProcess:
		content = a.process.View()
	}

	return lipgloss.NewStyle().
		Width(a.width).
		Height(a.height).
		Render(content)
}

func Run(db *sql.DB, cfg *config.Config) error {
	app := NewApp(db, cfg)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
