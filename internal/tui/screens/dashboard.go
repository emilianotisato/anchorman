package screens

import (
	"database/sql"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/emilianohg/anchorman/internal/repository"
)

type Dashboard struct {
	db     *sql.DB
	width  int
	height int

	unprocessedCount int
	lastProcessed    string
	companies        []repository.CompanyWithStats
	loading          bool
	err              error
}

func NewDashboard(db *sql.DB) *Dashboard {
	return &Dashboard{
		db:      db,
		loading: true,
	}
}

func (d *Dashboard) SetSize(width, height int) {
	d.width = width
	d.height = height
}

type dashboardDataMsg struct {
	unprocessedCount int
	lastProcessed    string
	companies        []repository.CompanyWithStats
	err              error
}

func (d *Dashboard) Init() tea.Cmd {
	d.loading = true
	return d.loadData
}

func (d *Dashboard) loadData() tea.Msg {
	commitRepo := repository.NewCommitRepo(d.db)
	companyRepo := repository.NewCompanyRepo(d.db)

	unprocessed, err := commitRepo.CountUnprocessed()
	if err != nil {
		return dashboardDataMsg{err: err}
	}

	lastTime, err := commitRepo.GetLastProcessedTime()
	if err != nil {
		return dashboardDataMsg{err: err}
	}

	lastProcessed := "Never"
	if lastTime != nil {
		lastProcessed = lastTime.Format("Jan 02, 2006 15:04")
	}

	companies, err := companyRepo.GetAllWithStats()
	if err != nil {
		return dashboardDataMsg{err: err}
	}

	return dashboardDataMsg{
		unprocessedCount: unprocessed,
		lastProcessed:    lastProcessed,
		companies:        companies,
	}
}

func (d *Dashboard) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case dashboardDataMsg:
		d.loading = false
		d.err = msg.err
		d.unprocessedCount = msg.unprocessedCount
		d.lastProcessed = msg.lastProcessed
		d.companies = msg.companies
		return nil

	case RefreshMsg:
		return d.Init()

	case tea.KeyMsg:
		switch msg.String() {
		case "p":
			return Navigate("process")
		case "c":
			return Navigate("companies")
		case "r":
			return Navigate("reports")
		case "o":
			return Navigate("repos")
		}
	}

	return nil
}

func (d *Dashboard) View() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("ANCHORMAN"))
	b.WriteString("\n")
	b.WriteString(SubtitleStyle.Render("Git Activity Tracker"))
	b.WriteString("\n\n")

	if d.loading {
		b.WriteString("Loading...\n")
		return b.String()
	}

	if d.err != nil {
		b.WriteString(ErrorStyle.Render(fmt.Sprintf("Error: %v", d.err)))
		b.WriteString("\n")
		return b.String()
	}

	// Stats box
	statsContent := fmt.Sprintf(
		"Unprocessed commits: %s\nLast processed: %s",
		d.formatUnprocessed(),
		d.lastProcessed,
	)
	b.WriteString(BoxStyle.Render(statsContent))
	b.WriteString("\n\n")

	// Companies summary
	if len(d.companies) > 0 {
		b.WriteString(SubtitleStyle.Render("Companies"))
		b.WriteString("\n")
		for _, c := range d.companies {
			b.WriteString(fmt.Sprintf("  %s - %d projects, %d repos, %d tasks\n",
				NormalStyle.Render(c.Name),
				c.ProjectCount,
				c.RepoCount,
				c.TaskCount,
			))
		}
	} else {
		b.WriteString(DimStyle.Render("No companies yet. Press 'c' to create one."))
	}

	b.WriteString("\n")

	// Help
	help := "[p] Process commits  [c] Companies  [o] Repos  [r] Reports  [q] Quit"
	b.WriteString(HelpStyle.Render(help))

	return b.String()
}

func (d *Dashboard) formatUnprocessed() string {
	if d.unprocessedCount == 0 {
		return SuccessStyle.Render("0")
	}
	return WarningStyle.Render(fmt.Sprintf("%d", d.unprocessedCount))
}
