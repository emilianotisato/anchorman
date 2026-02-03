package screens

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/emilianohg/anchorman/internal/repository"
)

type projectsMode int

const (
	projectsModeList projectsMode = iota
	projectsModeAdd
	projectsModeEdit
	projectsModeDelete
	projectsModeMove
)

type Projects struct {
	db     *sql.DB
	width  int
	height int

	projects        []repository.ProjectWithStats
	companies       []repository.CompanyWithStats
	companyFilter   *int64
	cursor          int
	companyCursor   int
	mode            projectsMode
	input           textinput.Model
	loading         bool
	err             error
	message         string
}

func NewProjects(db *sql.DB) *Projects {
	ti := textinput.New()
	ti.Placeholder = "Project name"
	ti.CharLimit = 100
	ti.Width = 40

	return &Projects{
		db:    db,
		input: ti,
	}
}

func (p *Projects) SetSize(width, height int) {
	p.width = width
	p.height = height
}

func (p *Projects) SetCompanyFilter(companyID *int64) {
	p.companyFilter = companyID
}

type projectsDataMsg struct {
	projects  []repository.ProjectWithStats
	companies []repository.CompanyWithStats
	err       error
}

func (p *Projects) Init() tea.Cmd {
	p.loading = true
	p.mode = projectsModeList
	p.message = ""
	return p.loadData
}

func (p *Projects) loadData() tea.Msg {
	projectRepo := repository.NewProjectRepo(p.db)
	companyRepo := repository.NewCompanyRepo(p.db)

	projects, err := projectRepo.GetAllWithStats()
	if err != nil {
		return projectsDataMsg{err: err}
	}

	companies, err := companyRepo.GetAllWithStats()
	if err != nil {
		return projectsDataMsg{err: err}
	}

	return projectsDataMsg{projects: projects, companies: companies}
}

func (p *Projects) Update(msg tea.Msg) tea.Cmd {
	// In input mode, pass messages to text input first
	if p.mode == projectsModeAdd || p.mode == projectsModeEdit {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "enter":
				return p.handleInputKey()
			case "esc":
				p.mode = projectsModeList
				p.input.Blur()
				return nil
			}
		}
		// Pass all other messages to text input
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		return cmd
	}

	switch msg := msg.(type) {
	case projectsDataMsg:
		p.loading = false
		p.err = msg.err
		p.projects = msg.projects
		p.companies = msg.companies

		// Filter by company if set
		if p.companyFilter != nil {
			var filtered []repository.ProjectWithStats
			for _, proj := range p.projects {
				if proj.CompanyID != nil && *proj.CompanyID == *p.companyFilter {
					filtered = append(filtered, proj)
				}
			}
			p.projects = filtered
		}

		if p.cursor >= len(p.projects) {
			p.cursor = max(0, len(p.projects)-1)
		}
		return nil

	case RefreshMsg:
		return p.Init()

	case tea.KeyMsg:
		return p.handleKey(msg)
	}

	return nil
}

func (p *Projects) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch p.mode {
	case projectsModeList:
		return p.handleListKey(msg)
	case projectsModeDelete:
		return p.handleDeleteKey(msg)
	case projectsModeMove:
		return p.handleMoveKey(msg)
	}
	return nil
}

func (p *Projects) handleListKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		if p.cursor > 0 {
			p.cursor--
		}
	case "down", "j":
		if p.cursor < len(p.projects)-1 {
			p.cursor++
		}
	case "a":
		p.mode = projectsModeAdd
		p.input.SetValue("")
		p.input.Focus()
	case "e":
		if len(p.projects) > 0 {
			p.mode = projectsModeEdit
			p.input.SetValue(p.projects[p.cursor].Name)
			p.input.Focus()
		}
	case "d":
		if len(p.projects) > 0 {
			p.mode = projectsModeDelete
		}
	case "m":
		if len(p.projects) > 0 && len(p.companies) > 0 {
			p.mode = projectsModeMove
			p.companyCursor = 0
		}
	case "enter":
		if len(p.projects) > 0 {
			return NavigateWithProject("repos", p.projects[p.cursor].ID)
		}
	case "q", "esc":
		if p.companyFilter != nil {
			return Navigate("companies")
		}
		return Navigate("dashboard")
	}
	return nil
}

func (p *Projects) handleInputKey() tea.Cmd {
	name := strings.TrimSpace(p.input.Value())
	if name == "" {
		p.mode = projectsModeList
		p.input.Blur()
		return nil
	}

	repo := repository.NewProjectRepo(p.db)
	if p.mode == projectsModeAdd {
		_, err := repo.Create(name, p.companyFilter)
		if err != nil {
			p.err = err
		} else {
			p.message = fmt.Sprintf("Created project: %s", name)
		}
	} else {
		err := repo.Update(p.projects[p.cursor].ID, name)
		if err != nil {
			p.err = err
		} else {
			p.message = fmt.Sprintf("Updated project: %s", name)
		}
	}
	p.mode = projectsModeList
	p.input.Blur()
	return p.loadData
}

func (p *Projects) handleDeleteKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "Y":
		repo := repository.NewProjectRepo(p.db)
		name := p.projects[p.cursor].Name
		err := repo.Delete(p.projects[p.cursor].ID)
		if err != nil {
			p.err = err
		} else {
			p.message = fmt.Sprintf("Deleted project: %s", name)
		}
		p.mode = projectsModeList
		return p.loadData

	case "n", "N", "esc":
		p.mode = projectsModeList
	}
	return nil
}

func (p *Projects) handleMoveKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		if p.companyCursor > 0 {
			p.companyCursor--
		}
	case "down", "j":
		if p.companyCursor < len(p.companies)-1 {
			p.companyCursor++
		}
	case "enter":
		repo := repository.NewProjectRepo(p.db)
		companyID := p.companies[p.companyCursor].ID
		err := repo.SetCompany(p.projects[p.cursor].ID, &companyID)
		if err != nil {
			p.err = err
		} else {
			p.message = fmt.Sprintf("Moved to %s", p.companies[p.companyCursor].Name)
		}
		p.mode = projectsModeList
		return p.loadData

	case "esc":
		p.mode = projectsModeList
	}
	return nil
}

func (p *Projects) View() string {
	var b strings.Builder

	title := "PROJECTS"
	if p.companyFilter != nil {
		for _, c := range p.companies {
			if c.ID == *p.companyFilter {
				title = fmt.Sprintf("PROJECTS - %s", c.Name)
				break
			}
		}
	}
	b.WriteString(TitleStyle.Render(title))
	b.WriteString("\n\n")

	if p.loading {
		b.WriteString("Loading...\n")
		return b.String()
	}

	if p.err != nil {
		b.WriteString(ErrorStyle.Render(fmt.Sprintf("Error: %v", p.err)))
		b.WriteString("\n\n")
		p.err = nil
	}

	if p.message != "" {
		b.WriteString(SuccessStyle.Render(p.message))
		b.WriteString("\n\n")
	}

	// Input modes
	if p.mode == projectsModeAdd {
		b.WriteString("New project name:\n")
		b.WriteString(p.input.View())
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("[enter] Save  [esc] Cancel"))
		return b.String()
	}

	if p.mode == projectsModeEdit {
		b.WriteString("Edit project name:\n")
		b.WriteString(p.input.View())
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("[enter] Save  [esc] Cancel"))
		return b.String()
	}

	if p.mode == projectsModeDelete && len(p.projects) > 0 {
		b.WriteString(WarningStyle.Render(fmt.Sprintf(
			"Delete project '%s'? This will orphan its repos. (y/n)",
			p.projects[p.cursor].Name,
		)))
		b.WriteString("\n")
		return b.String()
	}

	if p.mode == projectsModeMove {
		b.WriteString("Move to company:\n\n")
		for i, c := range p.companies {
			cursor := "  "
			style := NormalStyle
			if i == p.companyCursor {
				cursor = "> "
				style = SelectedStyle
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s", cursor, c.Name)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(HelpStyle.Render("[enter] Select  [esc] Cancel"))
		return b.String()
	}

	// List mode
	if len(p.projects) == 0 {
		b.WriteString(DimStyle.Render("No projects yet."))
		b.WriteString("\n\n")
	} else {
		for i, proj := range p.projects {
			cursor := "  "
			style := NormalStyle
			if i == p.cursor {
				cursor = "> "
				style = SelectedStyle
			}

			company := DimStyle.Render("(no company)")
			if proj.CompanyName != "" {
				company = DimStyle.Render(fmt.Sprintf("(%s)", proj.CompanyName))
			}

			line := fmt.Sprintf("%s%s %s - %d repos",
				cursor,
				proj.Name,
				company,
				proj.RepoCount,
			)
			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	help := "[a] Add  [e] Edit  [d] Delete  [m] Move  [enter] View repos  [q] Back"
	b.WriteString(HelpStyle.Render(help))

	return b.String()
}
