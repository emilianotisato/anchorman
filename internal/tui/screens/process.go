package screens

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/emilianohg/anchorman/internal/agent"
	"github.com/emilianohg/anchorman/internal/config"
	"github.com/emilianohg/anchorman/internal/models"
	"github.com/emilianohg/anchorman/internal/repository"
)

type processMode int

const (
	processModeSelectRange processMode = iota
	processModeConfirm
	processModeProcessing
	processModeComplete
)

type processRange int

const (
	processRangeAll processRange = iota
	processRangeLast7Days
	processRangeLast30Days
)

var processRangeLabels = []string{
	"All unprocessed",
	"Last 7 days",
	"Last 30 days",
}

type Process struct {
	db     *sql.DB
	cfg    *config.Config
	width  int
	height int

	mode           processMode
	rangeCursor    int
	selectedRange  processRange
	unprocessed    int
	commitsToProcess []models.RawCommit
	tasksCreated   int
	currentProject string
	loading        bool
	err            error
}

func NewProcess(db *sql.DB, cfg *config.Config) *Process {
	return &Process{
		db:  db,
		cfg: cfg,
	}
}

func (p *Process) SetSize(width, height int) {
	p.width = width
	p.height = height
}

type processCountMsg struct {
	count int
	err   error
}

type processCommitsMsg struct {
	commits []models.RawCommit
	err     error
}

type processCompleteMsg struct {
	tasksCreated int
	err          error
}

type processProgressMsg struct {
	project string
}

func (p *Process) Init() tea.Cmd {
	p.mode = processModeSelectRange
	p.loading = true
	p.err = nil
	p.tasksCreated = 0
	p.currentProject = ""
	return p.loadCount
}

func (p *Process) loadCount() tea.Msg {
	commitRepo := repository.NewCommitRepo(p.db)
	count, err := commitRepo.CountUnprocessed()
	return processCountMsg{count: count, err: err}
}

func (p *Process) loadCommits() tea.Msg {
	commitRepo := repository.NewCommitRepo(p.db)

	var commits []models.RawCommit
	var err error

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	switch p.selectedRange {
	case processRangeAll:
		commits, err = commitRepo.GetUnprocessed()
	case processRangeLast7Days:
		from := today.AddDate(0, 0, -7)
		to := today.Add(24*time.Hour - time.Second)
		commits, err = commitRepo.GetUnprocessedInDateRange(from, to)
	case processRangeLast30Days:
		from := today.AddDate(0, 0, -30)
		to := today.Add(24*time.Hour - time.Second)
		commits, err = commitRepo.GetUnprocessedInDateRange(from, to)
	}

	return processCommitsMsg{commits: commits, err: err}
}

func (p *Process) runProcessing() tea.Msg {
	if len(p.commitsToProcess) == 0 {
		return processCompleteMsg{tasksCreated: 0}
	}

	// Group commits by project
	commitsByProject := make(map[int64][]models.RawCommit)
	projectNames := make(map[int64]string)

	repoRepo := repository.NewRepoRepo(p.db)

	for _, c := range p.commitsToProcess {
		repo, err := repoRepo.GetByID(c.RepoID)
		if err != nil || repo == nil {
			continue
		}

		if repo.ProjectID == nil {
			// Skip orphan repos
			continue
		}

		commitsByProject[*repo.ProjectID] = append(commitsByProject[*repo.ProjectID], c)
		projectNames[*repo.ProjectID] = repo.ProjectName
	}

	// Get agent
	ag, err := agent.New(p.cfg.DefaultAgent)
	if err != nil {
		return processCompleteMsg{err: err}
	}

	taskRepo := repository.NewTaskRepo(p.db)
	commitRepo := repository.NewCommitRepo(p.db)
	totalTasks := 0

	// Process each project
	for projectID, commits := range commitsByProject {
		projectName := projectNames[projectID]
		if projectName == "" {
			projectName = "Unknown"
		}

		// Call agent
		tasks, err := ag.Process(projectName, commits)
		if err != nil {
			return processCompleteMsg{err: fmt.Errorf("failed to process %s: %w", projectName, err)}
		}

		// Get commit IDs
		var commitIDs []int64
		for _, c := range commits {
			commitIDs = append(commitIDs, c.ID)
		}

		// Determine task date (use the most recent commit date)
		var taskDate time.Time
		for _, c := range commits {
			if c.CommittedAt.After(taskDate) {
				taskDate = c.CommittedAt
			}
		}

		// Create tasks
		for _, task := range tasks {
			_, err := taskRepo.Create(projectID, task.Description, commitIDs, taskDate, task.EstimatedHours)
			if err != nil {
				return processCompleteMsg{err: fmt.Errorf("failed to create task: %w", err)}
			}
			totalTasks++
		}

		// Mark commits as processed
		if err := commitRepo.MarkProcessed(commitIDs); err != nil {
			return processCompleteMsg{err: fmt.Errorf("failed to mark commits processed: %w", err)}
		}
	}

	return processCompleteMsg{tasksCreated: totalTasks}
}

func (p *Process) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case processCountMsg:
		p.loading = false
		p.err = msg.err
		p.unprocessed = msg.count
		return nil

	case processCommitsMsg:
		p.loading = false
		p.err = msg.err
		p.commitsToProcess = msg.commits
		p.mode = processModeConfirm
		return nil

	case processCompleteMsg:
		p.loading = false
		p.err = msg.err
		p.tasksCreated = msg.tasksCreated
		if msg.err == nil {
			p.mode = processModeComplete
		}
		return nil

	case processProgressMsg:
		p.currentProject = msg.project
		return nil

	case RefreshMsg:
		return p.Init()

	case tea.KeyMsg:
		return p.handleKey(msg)
	}

	return nil
}

func (p *Process) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch p.mode {
	case processModeSelectRange:
		return p.handleRangeKey(msg)
	case processModeConfirm:
		return p.handleConfirmKey(msg)
	case processModeComplete:
		return p.handleCompleteKey(msg)
	}
	return nil
}

func (p *Process) handleRangeKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		if p.rangeCursor > 0 {
			p.rangeCursor--
		}
	case "down", "j":
		if p.rangeCursor < len(processRangeLabels)-1 {
			p.rangeCursor++
		}
	case "enter":
		p.selectedRange = processRange(p.rangeCursor)
		p.loading = true
		return p.loadCommits
	case "q", "esc":
		return Navigate("dashboard")
	}
	return nil
}

func (p *Process) handleConfirmKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter", "y":
		p.mode = processModeProcessing
		p.loading = true
		return p.runProcessing
	case "esc", "n":
		p.mode = processModeSelectRange
	case "q":
		return Navigate("dashboard")
	}
	return nil
}

func (p *Process) handleCompleteKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter", "q", "esc":
		return Navigate("dashboard")
	}
	return nil
}

func (p *Process) View() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("PROCESS COMMITS"))
	b.WriteString("\n\n")

	if p.loading && p.mode == processModeProcessing {
		b.WriteString(fmt.Sprintf("Processing with %s...\n", p.cfg.DefaultAgent))
		if p.currentProject != "" {
			b.WriteString(fmt.Sprintf("Current: %s\n", p.currentProject))
		}
		return b.String()
	}

	if p.loading {
		b.WriteString("Loading...\n")
		return b.String()
	}

	if p.err != nil {
		b.WriteString(ErrorStyle.Render(fmt.Sprintf("Error: %v", p.err)))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("[enter] Try again  [q] Back"))
		return b.String()
	}

	switch p.mode {
	case processModeSelectRange:
		return p.viewSelectRange(&b)
	case processModeConfirm:
		return p.viewConfirm(&b)
	case processModeComplete:
		return p.viewComplete(&b)
	}

	return b.String()
}

func (p *Process) viewSelectRange(b *strings.Builder) string {
	b.WriteString(fmt.Sprintf("Unprocessed commits: %s\n\n", WarningStyle.Render(fmt.Sprintf("%d", p.unprocessed))))

	if p.unprocessed == 0 {
		b.WriteString(SuccessStyle.Render("All commits are processed!"))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("[q] Back"))
		return b.String()
	}

	b.WriteString("Select commits to process:\n\n")

	for i, label := range processRangeLabels {
		cursor := "  "
		style := NormalStyle
		if i == p.rangeCursor {
			cursor = "> "
			style = SelectedStyle
		}
		b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, label)))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("Agent: %s\n\n", DimStyle.Render(p.cfg.DefaultAgent)))
	b.WriteString(HelpStyle.Render("[enter] Select  [q] Back"))

	return b.String()
}

func (p *Process) viewConfirm(b *strings.Builder) string {
	// Count commits per project
	projectCounts := make(map[string]int)
	for _, c := range p.commitsToProcess {
		projectCounts[c.RepoPath]++
	}

	orphanCount := 0
	for _, c := range p.commitsToProcess {
		repoRepo := repository.NewRepoRepo(p.db)
		repo, _ := repoRepo.GetByPath(c.RepoPath)
		if repo != nil && repo.ProjectID == nil {
			orphanCount++
		}
	}

	b.WriteString(fmt.Sprintf("Ready to process %d commits\n\n", len(p.commitsToProcess)))

	if orphanCount > 0 {
		b.WriteString(WarningStyle.Render(fmt.Sprintf("Note: %d commits are from orphan repos and will be skipped.\n", orphanCount)))
		b.WriteString(DimStyle.Render("Assign repos to projects first to include them.\n"))
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("Agent: %s\n\n", p.cfg.DefaultAgent))
	b.WriteString("Proceed? (y/n)\n\n")
	b.WriteString(HelpStyle.Render("[y/enter] Process  [n/esc] Cancel"))

	return b.String()
}

func (p *Process) viewComplete(b *strings.Builder) string {
	b.WriteString(SuccessStyle.Render("Processing complete!"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Created %d tasks\n\n", p.tasksCreated))
	b.WriteString(HelpStyle.Render("[enter] Done"))

	return b.String()
}
