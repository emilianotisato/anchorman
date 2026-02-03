package screens

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/emilianohg/anchorman/internal/repository"
)

type companiesMode int

const (
	companiesModeList companiesMode = iota
	companiesModeAdd
	companiesModeEdit
	companiesModeDelete
)

type Companies struct {
	db     *sql.DB
	width  int
	height int

	companies []repository.CompanyWithStats
	cursor    int
	mode      companiesMode
	input     textinput.Model
	loading   bool
	err       error
	message   string
}

func NewCompanies(db *sql.DB) *Companies {
	ti := textinput.New()
	ti.Placeholder = "Company name"
	ti.CharLimit = 100
	ti.Width = 40

	return &Companies{
		db:    db,
		input: ti,
	}
}

func (c *Companies) SetSize(width, height int) {
	c.width = width
	c.height = height
}

type companiesDataMsg struct {
	companies []repository.CompanyWithStats
	err       error
}

func (c *Companies) Init() tea.Cmd {
	c.loading = true
	c.mode = companiesModeList
	c.message = ""
	return c.loadData
}

func (c *Companies) loadData() tea.Msg {
	repo := repository.NewCompanyRepo(c.db)
	companies, err := repo.GetAllWithStats()
	return companiesDataMsg{companies: companies, err: err}
}

func (c *Companies) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case companiesDataMsg:
		c.loading = false
		c.err = msg.err
		c.companies = msg.companies
		if c.cursor >= len(c.companies) {
			c.cursor = max(0, len(c.companies)-1)
		}
		return nil

	case RefreshMsg:
		return c.Init()

	case tea.KeyMsg:
		return c.handleKey(msg)
	}

	if c.mode == companiesModeAdd || c.mode == companiesModeEdit {
		var cmd tea.Cmd
		c.input, cmd = c.input.Update(msg)
		return cmd
	}

	return nil
}

func (c *Companies) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch c.mode {
	case companiesModeList:
		return c.handleListKey(msg)
	case companiesModeAdd, companiesModeEdit:
		return c.handleInputKey(msg)
	case companiesModeDelete:
		return c.handleDeleteKey(msg)
	}
	return nil
}

func (c *Companies) handleListKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up", "k":
		if c.cursor > 0 {
			c.cursor--
		}
	case "down", "j":
		if c.cursor < len(c.companies)-1 {
			c.cursor++
		}
	case "a":
		c.mode = companiesModeAdd
		c.input.SetValue("")
		c.input.Focus()
		return nil
	case "e":
		if len(c.companies) > 0 {
			c.mode = companiesModeEdit
			c.input.SetValue(c.companies[c.cursor].Name)
			c.input.Focus()
		}
	case "d":
		if len(c.companies) > 0 {
			c.mode = companiesModeDelete
		}
	case "enter":
		if len(c.companies) > 0 {
			return NavigateWithCompany("projects", c.companies[c.cursor].ID)
		}
	case "q", "esc":
		return Navigate("dashboard")
	}
	return nil
}

func (c *Companies) handleInputKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		name := strings.TrimSpace(c.input.Value())
		if name == "" {
			c.mode = companiesModeList
			return nil
		}

		repo := repository.NewCompanyRepo(c.db)
		if c.mode == companiesModeAdd {
			_, err := repo.Create(name)
			if err != nil {
				c.err = err
			} else {
				c.message = fmt.Sprintf("Created company: %s", name)
			}
		} else {
			err := repo.Update(c.companies[c.cursor].ID, name)
			if err != nil {
				c.err = err
			} else {
				c.message = fmt.Sprintf("Updated company: %s", name)
			}
		}
		c.mode = companiesModeList
		c.input.Blur()
		return c.loadData

	case "esc":
		c.mode = companiesModeList
		c.input.Blur()
	}
	return nil
}

func (c *Companies) handleDeleteKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "Y":
		repo := repository.NewCompanyRepo(c.db)
		name := c.companies[c.cursor].Name
		err := repo.Delete(c.companies[c.cursor].ID)
		if err != nil {
			c.err = err
		} else {
			c.message = fmt.Sprintf("Deleted company: %s", name)
		}
		c.mode = companiesModeList
		return c.loadData

	case "n", "N", "esc":
		c.mode = companiesModeList
	}
	return nil
}

func (c *Companies) View() string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("COMPANIES"))
	b.WriteString("\n\n")

	if c.loading {
		b.WriteString("Loading...\n")
		return b.String()
	}

	if c.err != nil {
		b.WriteString(ErrorStyle.Render(fmt.Sprintf("Error: %v", c.err)))
		b.WriteString("\n\n")
		c.err = nil
	}

	if c.message != "" {
		b.WriteString(SuccessStyle.Render(c.message))
		b.WriteString("\n\n")
	}

	// Input mode
	if c.mode == companiesModeAdd {
		b.WriteString("New company name:\n")
		b.WriteString(c.input.View())
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("[enter] Save  [esc] Cancel"))
		return b.String()
	}

	if c.mode == companiesModeEdit {
		b.WriteString("Edit company name:\n")
		b.WriteString(c.input.View())
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("[enter] Save  [esc] Cancel"))
		return b.String()
	}

	if c.mode == companiesModeDelete && len(c.companies) > 0 {
		b.WriteString(WarningStyle.Render(fmt.Sprintf(
			"Delete company '%s'? This will orphan its projects. (y/n)",
			c.companies[c.cursor].Name,
		)))
		b.WriteString("\n")
		return b.String()
	}

	// List mode
	if len(c.companies) == 0 {
		b.WriteString(DimStyle.Render("No companies yet."))
		b.WriteString("\n\n")
	} else {
		for i, company := range c.companies {
			cursor := "  "
			style := NormalStyle
			if i == c.cursor {
				cursor = "> "
				style = SelectedStyle
			}

			line := fmt.Sprintf("%s%s (%d projects, %d repos)",
				cursor,
				company.Name,
				company.ProjectCount,
				company.RepoCount,
			)
			b.WriteString(style.Render(line))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	help := "[a] Add  [e] Edit  [d] Delete  [enter] View projects  [q] Back"
	b.WriteString(HelpStyle.Render(help))

	return b.String()
}
