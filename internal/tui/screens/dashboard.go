package screens

import (
	"database/sql"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/emilianohg/anchorman/internal/db"
	"github.com/emilianohg/anchorman/internal/repository"
)

type Dashboard struct {
	database *sql.DB
	width    int
	height   int

	unprocessedCount  int
	orphanReposCount  int
	lastProcessed     string
	companies         []repository.CompanyWithStats
	migrationPending  bool
	migrationCurrent  uint
	migrationLatest   uint
	migrationDirty    bool
	loading           bool
	migrating         bool
	err               error
	message           string
}

func NewDashboard(database *sql.DB) *Dashboard {
	return &Dashboard{
		database: database,
		loading:  true,
	}
}

func (d *Dashboard) SetSize(width, height int) {
	d.width = width
	d.height = height
}

type dashboardDataMsg struct {
	unprocessedCount int
	orphanReposCount int
	lastProcessed    string
	companies        []repository.CompanyWithStats
	migrationPending bool
	migrationCurrent uint
	migrationLatest  uint
	migrationDirty   bool
	err              error
}

type migrationCompleteMsg struct {
	err error
}

func (d *Dashboard) Init() tea.Cmd {
	d.loading = true
	d.message = ""
	return d.loadData
}

func (d *Dashboard) loadData() tea.Msg {
	// Check migration status first
	status, err := db.GetMigrationStatus()
	if err != nil {
		return dashboardDataMsg{err: err}
	}

	// If migrations are pending, return early with just migration info
	if status.Pending || status.Dirty {
		return dashboardDataMsg{
			migrationPending: status.Pending,
			migrationCurrent: status.CurrentVersion,
			migrationLatest:  status.LatestVersion,
			migrationDirty:   status.Dirty,
		}
	}

	// Load normal dashboard data
	commitRepo := repository.NewCommitRepo(d.database)
	companyRepo := repository.NewCompanyRepo(d.database)
	repoRepo := repository.NewRepoRepo(d.database)

	unprocessed, err := commitRepo.CountUnprocessed()
	if err != nil {
		return dashboardDataMsg{err: err}
	}

	orphanRepos, err := repoRepo.GetOrphans()
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
		orphanReposCount: len(orphanRepos),
		lastProcessed:    lastProcessed,
		companies:        companies,
		migrationPending: false,
		migrationCurrent: status.CurrentVersion,
		migrationLatest:  status.LatestVersion,
	}
}

func (d *Dashboard) runMigrations() tea.Msg {
	err := db.RunMigrations()
	return migrationCompleteMsg{err: err}
}

func (d *Dashboard) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case dashboardDataMsg:
		d.loading = false
		d.err = msg.err
		d.unprocessedCount = msg.unprocessedCount
		d.orphanReposCount = msg.orphanReposCount
		d.lastProcessed = msg.lastProcessed
		d.companies = msg.companies
		d.migrationPending = msg.migrationPending
		d.migrationCurrent = msg.migrationCurrent
		d.migrationLatest = msg.migrationLatest
		d.migrationDirty = msg.migrationDirty
		return nil

	case migrationCompleteMsg:
		d.migrating = false
		if msg.err != nil {
			d.err = msg.err
			return nil
		}
		d.message = "Migrations completed successfully!"
		return d.loadData

	case RefreshMsg:
		return d.Init()

	case tea.KeyMsg:
		// Handle migration mode
		if d.migrationPending || d.migrationDirty {
			switch msg.String() {
			case "m":
				d.migrating = true
				return d.runMigrations
			case "q":
				return tea.Quit
			}
			return nil
		}

		// Normal mode
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

	if d.migrating {
		b.WriteString("Running migrations...\n")
		return b.String()
	}

	if d.loading {
		b.WriteString("Loading...\n")
		return b.String()
	}

	if d.err != nil {
		b.WriteString(ErrorStyle.Render(fmt.Sprintf("Error: %v", d.err)))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("[q] Quit"))
		return b.String()
	}

	// Show migration warning if pending
	if d.migrationPending || d.migrationDirty {
		return d.viewMigrationPending(&b)
	}

	if d.message != "" {
		b.WriteString(SuccessStyle.Render(d.message))
		b.WriteString("\n\n")
	}

	// Stats box
	statsContent := fmt.Sprintf(
		"Unprocessed commits: %s\nOrphan repos: %s\nLast processed: %s",
		d.formatUnprocessed(),
		d.formatOrphanRepos(),
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

func (d *Dashboard) viewMigrationPending(b *strings.Builder) string {
	b.WriteString(WarningStyle.Render("DATABASE UPDATE REQUIRED"))
	b.WriteString("\n\n")

	if d.migrationDirty {
		b.WriteString(ErrorStyle.Render("Warning: Database is in a dirty state from a failed migration."))
		b.WriteString("\n")
		b.WriteString("You may need to manually fix this before proceeding.\n\n")
	}

	b.WriteString(fmt.Sprintf("Current schema version: %d\n", d.migrationCurrent))
	b.WriteString(fmt.Sprintf("Latest schema version:  %d\n", d.migrationLatest))
	b.WriteString(fmt.Sprintf("Pending migrations: %d\n\n", d.migrationLatest-d.migrationCurrent))

	b.WriteString("A new version of anchorman includes database changes.\n")
	b.WriteString("Press 'm' to run migrations and update your database.\n\n")

	b.WriteString(HelpStyle.Render("[m] Run migrations  [q] Quit"))

	return b.String()
}

func (d *Dashboard) formatUnprocessed() string {
	if d.unprocessedCount == 0 {
		return SuccessStyle.Render("0")
	}
	return WarningStyle.Render(fmt.Sprintf("%d", d.unprocessedCount))
}

func (d *Dashboard) formatOrphanRepos() string {
	if d.orphanReposCount == 0 {
		return SuccessStyle.Render("0")
	}
	return WarningStyle.Render(fmt.Sprintf("%d (press 'o' to assign)", d.orphanReposCount))
}
