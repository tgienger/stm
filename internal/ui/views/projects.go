package views

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tgienger/stm/internal/db"
	"github.com/tgienger/stm/internal/models"
	"github.com/tgienger/stm/internal/ui/keys"
	"github.com/tgienger/stm/internal/ui/styles"
)

type projectItem struct {
	project models.Project
}

func (i projectItem) Title() string       { return i.project.Title }
func (i projectItem) Description() string { return i.project.Description }
func (i projectItem) FilterValue() string { return i.project.Title }

type projectDelegate struct {
	styles *styles.Styles
	width  int
}

func (d projectDelegate) Height() int                               { return 2 }
func (d projectDelegate) Spacing() int                              { return 1 }
func (d projectDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d projectDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	p, ok := item.(projectItem)
	if !ok {
		return
	}

	selected := index == m.Index()
	width := max(d.width-4, 20)

	var titleStyle, descStyle lipgloss.Style
	if selected {
		titleStyle = d.styles.ListSelected.Width(width)
		descStyle = d.styles.ListSelected.Foreground(styles.Current.ForegroundDim).Width(width)
	} else {
		titleStyle = d.styles.ListItem.Width(width)
		descStyle = d.styles.ListItem.Foreground(styles.Current.ForegroundDim).Width(width)
	}

	title := titleStyle.Render(p.Title())
	desc := descStyle.Render(p.Description())

	fmt.Fprintf(w, "%s\n%s", title, desc)
}

type ProjectListView struct {
	db               *db.DB
	list             list.Model
	delegate         *projectDelegate
	filter           textinput.Model
	styles           *styles.Styles
	keys             keys.KeyMap
	width            int
	height           int
	creating         bool
	loaded           bool
	confirmingDelete bool
	deleteTargetID   int64
	deleteTargetName string
	newName          textinput.Model
	newDesc          textinput.Model
	focusIdx         int // 0=name, 1=desc, 2=confirm

	// Help popup (shown with ? at narrow widths)
	showHelpPopup bool
}

func NewProjectListView(database *db.DB) *ProjectListView {
	s := styles.NewStyles()

	filter := textinput.New()
	filter.Placeholder = "Filter projects..."
	filter.CharLimit = 100

	newName := textinput.New()
	newName.Placeholder = "Project name"
	newName.CharLimit = 100

	newDesc := textinput.New()
	newDesc.Placeholder = "Description (optional)"
	newDesc.CharLimit = 100
	// newDesc.CharLimit = 500
	// newDesc.SetWidth(50)
	// newDesc.SetHeight(3)
	// newDesc.ShowLineNumbers = false

	// Setup custom delegate
	delegate := &projectDelegate{styles: s, width: 80}

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Projects"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = s.Title
	l.SetShowHelp(false)

	return &ProjectListView{
		db:       database,
		list:     l,
		delegate: delegate,
		filter:   filter,
		styles:   s,
		keys:     keys.DefaultKeyMap(),
		newName:  newName,
		newDesc:  newDesc,
	}
}

func (v *ProjectListView) Init() tea.Cmd {
	return v.loadProjects
}

func (v *ProjectListView) loadProjects() tea.Msg {
	projects, err := v.db.ListProjects()
	if err != nil {
		return err
	}
	return projectsLoadedMsg{projects: projects}
}

type projectsLoadedMsg struct {
	projects []models.Project
}

type SelectedProject struct {
	Project models.Project
}

func (v *ProjectListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		// Use content width (capped at MaxWidth) for internal layout
		contentWidth := styles.ContentWidth(msg.Width)
		v.delegate.width = contentWidth
		v.list.SetSize(contentWidth-4, msg.Height-6)
		return v, nil

	case projectsLoadedMsg:
		items := make([]list.Item, len(msg.projects))
		for i, p := range msg.projects {
			items[i] = projectItem{project: p}
		}
		v.list.SetItems(items)
		v.loaded = true
		return v, nil

	case tea.KeyMsg:
		// Handle help popup first - any key closes it
		if v.showHelpPopup {
			v.showHelpPopup = false
			return v, nil
		}

		if v.confirmingDelete {
			return v.updateConfirmDelete(msg)
		}

		if v.creating {
			return v.updateCreating(msg)
		}

		switch {
		case key.Matches(msg, v.keys.Quit):
			return v, tea.Quit
		case key.Matches(msg, v.keys.Back):
			// Don't quit on escape/backspace in project list - only q quits
			return v, nil
		case key.Matches(msg, v.keys.New):
			v.creating = true
			v.focusIdx = 0
			v.newName.Reset()
			v.newDesc.Reset()
			v.newName.Focus()
			return v, textinput.Blink
		case msg.String() == "?":
			v.showHelpPopup = true
			return v, nil
		case key.Matches(msg, v.keys.Enter):
			if item, ok := v.list.SelectedItem().(projectItem); ok {
				return v, func() tea.Msg {
					return SelectedProject{Project: item.project}
				}
			}
		case key.Matches(msg, v.keys.Delete):
			if item, ok := v.list.SelectedItem().(projectItem); ok {
				v.confirmingDelete = true
				v.deleteTargetID = item.project.ID
				v.deleteTargetName = item.project.Title
				return v, nil
			}
		}
	}

	var cmd tea.Cmd
	v.list, cmd = v.list.Update(msg)
	return v, cmd
}

func (v *ProjectListView) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if err := v.db.DeleteProject(v.deleteTargetID); err == nil {
			v.confirmingDelete = false
			return v, v.loadProjects
		}
		v.confirmingDelete = false
		return v, nil
	case "n", "N", "esc":
		v.confirmingDelete = false
		return v, nil
	}
	return v, nil
}

func (v *ProjectListView) updateCreating(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, v.keys.Back):
		v.creating = false
		return v, nil

	case msg.String() == "ctrl+s":
		name := strings.TrimSpace(v.newName.Value())
		if name != "" {
			project, err := v.db.CreateProject(name, strings.TrimSpace(v.newDesc.Value()))
			if err == nil {
				v.creating = false
				return v, func() tea.Msg {
					return SelectedProject{Project: *project}
				}
			}
		}
		return v, nil

	case msg.String() == "shift+tab":
		v.focusIdx = (v.focusIdx + 2) % 3
		v.updateFocus()
		return v, nil

	case key.Matches(msg, v.keys.Tab):
		v.focusIdx = (v.focusIdx + 1) % 3
		v.updateFocus()
		return v, nil

	case key.Matches(msg, v.keys.Enter):
		if v.focusIdx == 0 || v.focusIdx == 1 {
			v.focusIdx++
			v.updateFocus()
			return v, nil
		}

		if v.focusIdx == 2 {
			name := strings.TrimSpace(v.newName.Value())
			if name != "" {
				project, err := v.db.CreateProject(name, strings.TrimSpace(v.newDesc.Value()))
				if err == nil {
					v.creating = false
					return v, func() tea.Msg {
						return SelectedProject{Project: *project}
					}
				}
			}
			return v, nil
		}
	}

	var cmd tea.Cmd
	switch v.focusIdx {
	case 0:
		v.newName, cmd = v.newName.Update(msg)
	case 1:
		v.newDesc, cmd = v.newDesc.Update(msg)
	}
	return v, cmd
}

func (v *ProjectListView) updateFocus() {
	v.newName.Blur()
	v.newDesc.Blur()
	switch v.focusIdx {
	case 0:
		v.newName.Focus()
	case 1:
		v.newDesc.Focus()
	}
}

// View renders the view
func (v *ProjectListView) View() string {
	if v.showHelpPopup {
		return v.renderHelpPopup()
	}

	if v.confirmingDelete {
		return v.renderDeleteConfirm()
	}

	if v.creating {
		return v.renderCreateForm()
	}

	if !v.loaded {
		return v.styles.TitleMuted.Render("Loading...")
	}

	if len(v.list.Items()) == 0 {
		return v.renderEmpty()
	}

	content := v.list.View() + "\n" + v.renderHelp()
	return styles.CenterView(content, v.width, v.height)
}

func (v *ProjectListView) renderEmpty() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	content := lipgloss.JoinVertical(lipgloss.Center,
		s.Title.Render("No Projects"),
		"",
		s.TitleMuted.Render("Press 'n' to create your first project"),
		"",
		s.ButtonPrimary.Render(" New Project "),
	)

	// Center within content width, then center that in terminal
	centered := lipgloss.Place(contentWidth, v.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
	return styles.CenterView(centered, v.width, v.height)
}

func (v *ProjectListView) renderCreateForm() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	nameStyle := s.Input
	descStyle := s.Input
	btnStyle := s.Button

	switch v.focusIdx {
	case 0:
		nameStyle = s.InputFocused
	case 1:
		descStyle = s.InputFocused
	case 2:
		btnStyle = s.ButtonFocused
	}

	// Dynamic input width based on content width
	inputWidth := clamp(contentWidth-6, 20, 50)

	form := lipgloss.JoinVertical(lipgloss.Left,
		s.Title.Render("New Project"),
		"",
		"Name:",
		nameStyle.Width(inputWidth).Render(v.newName.View()),
		"",
		"Description:",
		descStyle.Width(inputWidth).Render(v.newDesc.View()),
		"",
		btnStyle.Render(" Create "),
		"",
		s.TitleMuted.Render("Tab: next • Ctrl+S: save • Esc: cancel"),
	)

	// Center within content width, then center that in terminal
	centered := lipgloss.Place(contentWidth, v.height,
		lipgloss.Center, lipgloss.Center,
		form,
	)
	return styles.CenterView(centered, v.width, v.height)
}

func (v *ProjectListView) renderHelp() string {
	contentWidth := styles.ContentWidth(v.width)
	// At narrow widths, show hint to press ? for help
	if contentWidth > 0 && contentWidth < 50 {
		return v.styles.Help.Render(v.styles.HelpKey.Render("?") + " help")
	}
	return v.styles.Help.Render(
		fmt.Sprintf("%s select • %s new • %s del • %s quit",
			v.styles.HelpKey.Render("↵"),
			v.styles.HelpKey.Render("n"),
			v.styles.HelpKey.Render("d"),
			v.styles.HelpKey.Render("q"),
		),
	)
}

func (v *ProjectListView) renderHelpPopup() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	helpItems := []string{
		s.HelpKey.Render("↵") + "      select project",
		s.HelpKey.Render("n") + "      new project",
		s.HelpKey.Render("d") + "      delete project",
		s.HelpKey.Render("q") + "      quit",
		"",
		s.TitleMuted.Render("Press any key to close"),
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		append([]string{s.Title.Render("Keyboard Shortcuts"), ""}, helpItems...)...,
	)

	centered := lipgloss.Place(contentWidth, v.height,
		lipgloss.Center, lipgloss.Center,
		s.FilterBar.Render(content),
	)
	return styles.CenterView(centered, v.width, v.height)
}

func (v *ProjectListView) renderDeleteConfirm() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	content := lipgloss.JoinVertical(lipgloss.Center,
		s.Title.Foreground(styles.Current.Error).Render("Delete Project?"),
		"",
		// s.TitleMuted.Render(fmt.Sprintf("Are you sure you want to delete \"%s\"?", v.deleteTargetName)),
		// s.TitleMuted.Render("This will also delete all tasks in this project."),
		"",
		lipgloss.JoinHorizontal(lipgloss.Center,
			s.ButtonPrimary.Render(" Y - Yes "),
			"  ",
			s.Button.Render(" N - No "),
		),
	)

	centered := lipgloss.Place(contentWidth, v.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
	return styles.CenterView(centered, v.width, v.height)
}
