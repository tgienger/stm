package ui

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/tgienger/stm/internal/db"
	"github.com/tgienger/stm/internal/models"
	"github.com/tgienger/stm/internal/ui/views"
)

// Currently active view
type View int

const (
	ViewProjects View = iota
	ViewTasks
)

type App struct {
	db          *db.DB
	currentView View
	projectList *views.ProjectListView
	taskList    *views.TaskListView
	width       int
	height      int
}

func NewApp(database *db.DB) *App {
	return &App{
		db:          database,
		currentView: ViewProjects,
		projectList: views.NewProjectListView(database),
	}
}

func (a *App) Init() tea.Cmd {
	lastProjectID, err := a.db.GetSetting("last_project_id")
	if err == nil && lastProjectID != "" {
		id, err := strconv.ParseInt(lastProjectID, 10, 64)
		if err == nil {
			project, err := a.db.GetProject(id)
			if err == nil {
				return a.openProject(*project)
			}
		}
	}

	return a.projectList.Init()
}

func (a *App) openProject(project models.Project) tea.Cmd {
	a.currentView = ViewTasks
	a.taskList = views.NewTaskListView(a.db, project)

	a.db.SetSetting("last_project_id", strconv.FormatInt(project.ID, 10))

	return tea.Batch(
		a.taskList.Init(),
		func() tea.Msg {
			return tea.WindowSizeMsg{Width: a.width, Height: a.height}
		},
	)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.projectList.Update(msg)

	case views.SelectedProject:
		return a, a.openProject(msg.Project)

	case views.BackToProjects:
		a.currentView = ViewProjects
		a.db.SetSetting("last_project_id", "")
		return a, tea.Batch(
			a.projectList.Init(),
			func() tea.Msg {
				return tea.WindowSizeMsg{Width: a.width, Height: a.height}
			},
		)
	}

	var cmd tea.Cmd
	switch a.currentView {
	case ViewProjects:
		_, cmd = a.projectList.Update(msg)
	case ViewTasks:
		_, cmd = a.taskList.Update(msg)
	}

	return a, cmd
}

func (a *App) View() string {
	switch a.currentView {
	case ViewTasks:
		if a.taskList != nil {
			return a.taskList.View()
		}
	}
	return a.projectList.View()
}
