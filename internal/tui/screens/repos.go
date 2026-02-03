package screens

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/emilianohg/anchorman/internal/repository"
)

type reposMode int

const (
	reposModeList reposMode = iota
	reposModeAssign
	reposModeDelete
)

type Repos struct {
	db     *sql.DB
	width  int
	height int

	repos         []repository.RepoWithStats
	projects      []repository.ProjectWithStats
	projectFilter *int64
	cursor        int
	projectCursor int
	mode          reposMode
	showOrphans   bool
	loading       bool
	err           error
	message       string
}

func NewRepos(db *sql.DB) *Repos {
	return &Repos{
		db: db,
	}
}

func (r *Repos) SetSize(width, height int) {
	r.width = width
	r.height = height
}

func (r *Repos) SetProjectFilter(projectID *int64) {
	r.projectFilter = projectID
}

type reposDataMsg struct {
	repos    []repository.RepoWithStats
	projects []repository.ProjectWithStats
	err      error
}

func (r *Repos) Init() tea.Cmd {
	r.loading = true
	r.mode = reposModeList
	r.message = ""
	return r.loadData
}

func (r *Repos) loadData() tea.Msg {
	repoRepo := repository.NewRepoRepo(r.db)
	projectRepo := repository.NewProjectRepo(r.db)

	repos, err := repoRepo.GetAllWithStats()
	if err != nil {
		return reposDataMsg{err: err}
	}

	projects, err := projectRepo.GetAllWithStats()
	if err != nil {
		return reposDataMsg{err: err}
	}

	return reposDataMsg{repos: repos, projects: projects}
}

func (r *Repos) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case reposDataMsg:
		r.loading = false
		r.err = msg.err
		r.repos = msg.repos
		r.projects = msg.projects
		r.filterRepos()
		return nil

	case RefreshMsg:
		return r.Init()

	case tea.KeyMsg:
		return r.handleKey(msg)
	}

	return nil
}

func (r *Repos) filterRepos() {
	if r.projectFilter != nil {
		var filtered []repository.RepoWithStats
		for _, repo := range r.repos {
			if repo.ProjectID != nil && *repo.ProjectID == *r.projectFilter {
				filtered = append(filtered, repo)
			}
		}
		r.repos = filtered
	} else if r.showOrphans {
		var filtered []repository.RepoWithStats
		for _, repo := range r.repos {
			if repo.ProjectID == nil {
				filtered = append(filtered, repo)
			}
		}
		r.repos = filtered
	}

	if r.cursor >= len(r.repos) {
		r.cursor = max(0, len(r.repos)-1)
	}
}

func (r *Repos) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch r.mode {
	case reposModeList:
		return r.handleListKey(msg)
	case reposModeAssign:
		return r.handleAssignKey(msg)
	case reposModeDelete:
		return r.handleDeleteKey(msg)
	}
	return nil
}

func (r *Repos) handleListKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		if r.cursor > 0 {
			r.cursor--
		}
	case "down", "j":
		if r.cursor < len(r.repos)-1 {
			r.cursor++
		}
	case "a":
		if len(r.repos) > 0 && len(r.projects) > 0 {
			r.mode = reposModeAssign
			r.projectCursor = 0
		}
	case "d":
		if len(r.repos) > 0 {
			r.mode = reposModeDelete
		}
	case "f":
		r.showOrphans = !r.showOrphans
		return r.loadData
	case "q", "esc":
		if r.projectFilter != nil {
			return Navigate("projects")
		}
		return Navigate("dashboard")
	}
	return nil
}

func (r *Repos) handleAssignKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		if r.projectCursor > 0 {
			r.projectCursor--
		}
	case "down", "j":
		if r.projectCursor < len(r.projects)-1 {
			r.projectCursor++
		}
	case "enter":
		repo := repository.NewRepoRepo(r.db)
		projectID := r.projects[r.projectCursor].ID
		err := repo.SetProject(r.repos[r.cursor].ID, &projectID)
		if err != nil {
			r.err = err
		} else {
			r.message = fmt.Sprintf("Assigned to %s", r.projects[r.projectCursor].Name)
		}
		r.mode = reposModeList
		return r.loadData

	case "esc":
		r.mode = reposModeList
	}
	return nil
}

func (r *Repos) handleDeleteKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "Y":
		repo := repository.NewRepoRepo(r.db)
		path := r.repos[r.cursor].Path
		err := repo.Delete(r.repos[r.cursor].ID)
		if err != nil {
			r.err = err
		} else {
			r.message = fmt.Sprintf("Removed: %s", filepath.Base(path))
		}
		r.mode = reposModeList
		return r.loadData

	case "n", "N", "esc":
		r.mode = reposModeList
	}
	return nil
}

func (r *Repos) View() string {
	var b strings.Builder

	title := "REPOSITORIES"
	if r.showOrphans {
		title = "REPOSITORIES (Orphans)"
	}
	b.WriteString(TitleStyle.Render(title))
	b.WriteString("\n\n")

	if r.loading {
		b.WriteString("Loading...\n")
		return b.String()
	}

	if r.err != nil {
		b.WriteString(ErrorStyle.Render(fmt.Sprintf("Error: %v", r.err)))
		b.WriteString("\n\n")
		r.err = nil
	}

	if r.message != "" {
		b.WriteString(SuccessStyle.Render(r.message))
		b.WriteString("\n\n")
	}

	// Assign mode
	if r.mode == reposModeAssign {
		b.WriteString("Assign to project:\n\n")
		for i, p := range r.projects {
			cursor := "  "
			style := NormalStyle
			if i == r.projectCursor {
				cursor = "> "
				style = SelectedStyle
			}
			company := ""
			if p.CompanyName != "" {
				company = DimStyle.Render(fmt.Sprintf(" (%s)", p.CompanyName))
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, p.Name)))
			b.WriteString(company)
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(HelpStyle.Render("[enter] Select  [esc] Cancel"))
		return b.String()
	}

	// Delete mode
	if r.mode == reposModeDelete && len(r.repos) > 0 {
		b.WriteString(WarningStyle.Render(fmt.Sprintf(
			"Remove repo '%s' from tracking? (y/n)\nNote: This only removes from anchorman, not the actual repo.",
			filepath.Base(r.repos[r.cursor].Path),
		)))
		b.WriteString("\n")
		return b.String()
	}

	// List mode
	if len(r.repos) == 0 {
		if r.showOrphans {
			b.WriteString(SuccessStyle.Render("No orphan repos!"))
		} else {
			b.WriteString(DimStyle.Render("No repos tracked yet. Commits will appear after you install hooks."))
		}
		b.WriteString("\n\n")
	} else {
		for i, repo := range r.repos {
			cursor := "  "
			style := NormalStyle
			if i == r.cursor {
				cursor = "> "
				style = SelectedStyle
			}

			name := filepath.Base(repo.Path)
			project := DimStyle.Render("(orphan)")
			if repo.ProjectName != "" {
				project = DimStyle.Render(fmt.Sprintf("(%s)", repo.ProjectName))
			}

			unprocessed := ""
			if repo.UnprocessedCommitCount > 0 {
				unprocessed = WarningStyle.Render(fmt.Sprintf(" [%d unprocessed]", repo.UnprocessedCommitCount))
			}

			line := fmt.Sprintf("%s%s %s%s",
				cursor,
				name,
				project,
				unprocessed,
			)
			b.WriteString(style.Render(line))
			b.WriteString("\n")

			// Show path on second line when selected
			if i == r.cursor {
				b.WriteString(DimStyle.Render(fmt.Sprintf("     %s", repo.Path)))
				b.WriteString("\n")
			}
		}
		b.WriteString("\n")
	}

	help := "[a] Assign to project  [d] Remove  [f] Toggle orphans  [q] Back"
	b.WriteString(HelpStyle.Render(help))

	return b.String()
}
