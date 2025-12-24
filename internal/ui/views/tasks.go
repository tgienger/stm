package views

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tgienger/stm/internal/db"
	"github.com/tgienger/stm/internal/models"
	"github.com/tgienger/stm/internal/ui/keys"
	"github.com/tgienger/stm/internal/ui/styles"
)

// clamp returns val clamped between minVal and maxVal
func clamp(val, minVal, maxVal int) int {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

// FocusArea represents which part of the UI has focus
type FocusArea int

const (
	FocusBackButton FocusArea = iota
	FocusSearchInput
	FocusTagDropdown
	FocusTaskList
)

// TaskListView shows tasks for a project
type TaskListView struct {
	db      *db.DB
	project models.Project
	tasks   []models.Task
	tags    []models.Tag
	styles  *styles.Styles
	keys    keys.KeyMap

	width  int
	height int

	// UI state
	focus       FocusArea
	cursor      int
	scrollY     int
	searchInput textinput.Model
	selectedTag *int64 // nil = no filter

	// Tag dropdown state
	tagDropdownOpen bool
	tagCursor       int

	// Task creation/editing
	editing       bool
	editingNew    bool
	editTitle     textinput.Model
	editDesc      textarea.Model
	editNotes     textarea.Model
	editPriority  textinput.Model
	editFocusIdx  int     // 0=title, 1=desc, 2=notes, 3=priority, 4=tags, 5=save
	editTags      []int64 // IDs of tags selected for this task
	editTagCursor int     // cursor position in tag list when focused

	// Tag assignment mode
	assigningTags   bool
	assignTagCursor int
	assigningTaskID int64 // ID of task being edited in tag assignment mode

	// Task view mode (read-only detail view)
	viewingTask         bool
	viewTaskComments    []models.Comment // comments for the currently viewed task
	commentInput        textarea.Model   // textarea for adding new comments
	commentInputFocused bool             // whether the comment input is focused

	// Delete confirmation
	confirmingDelete bool
	deleteTargetID   int64
	deleteTargetName string

	// Show completed tasks mode
	showingCompleted bool   // true when viewing only completed tasks
	preCompletedTag  *int64 // saved tag filter before toggling to completed view
	completeTagID    *int64 // cached ID of the 'complete' tag

	// Help popup (shown with ? at narrow widths)
	showHelpPopup bool
}

// NewTaskListView creates a new task list view
func NewTaskListView(database *db.DB, project models.Project) *TaskListView {
	s := styles.NewStyles()

	search := textinput.New()
	search.Placeholder = "Search tasks..."
	search.CharLimit = 100

	editTitle := textinput.New()
	editTitle.Placeholder = "Task title"
	editTitle.CharLimit = 200

	editDesc := textarea.New()
	editDesc.Placeholder = "Description"
	editDesc.CharLimit = 1000
	editDesc.SetWidth(50)
	editDesc.SetHeight(3)
	editDesc.ShowLineNumbers = false

	editNotes := textarea.New()
	editNotes.Placeholder = "Notes"
	editNotes.CharLimit = 5000
	editNotes.SetWidth(50)
	editNotes.SetHeight(5)
	editNotes.ShowLineNumbers = false

	editPriority := textinput.New()
	editPriority.Placeholder = "0-10"
	editPriority.CharLimit = 2

	commentInput := textarea.New()
	commentInput.Placeholder = "Add a comment..."
	commentInput.CharLimit = 2000
	commentInput.SetWidth(50)
	commentInput.SetHeight(3)
	commentInput.ShowLineNumbers = false

	return &TaskListView{
		db:           database,
		project:      project,
		styles:       s,
		keys:         keys.DefaultKeyMap(),
		focus:        FocusTaskList,
		searchInput:  search,
		editTitle:    editTitle,
		editDesc:     editDesc,
		editNotes:    editNotes,
		editPriority: editPriority,
		commentInput: commentInput,
	}
}

// BackToProjects signals to go back to project list
type BackToProjects struct{}

// Init initializes the view
func (v *TaskListView) Init() tea.Cmd {
	return tea.Batch(v.loadTasks, v.loadTags)
}

type tasksLoadedMsg struct {
	tasks []models.Task
}

type tagsLoadedMsg struct {
	tags []models.Tag
}

func (v *TaskListView) loadTasks() tea.Msg {
	var tasks []models.Task
	var err error

	// Cache the complete tag ID if we haven't yet
	if v.completeTagID == nil {
		if completeTag, err := v.db.GetTagByName("complete"); err == nil && completeTag != nil {
			v.completeTagID = &completeTag.ID
		}
	}

	search := strings.TrimSpace(v.searchInput.Value())

	// Determine exclusion: hide completed tasks unless we're showing only completed
	var excludeTagID *int64
	if !v.showingCompleted && v.completeTagID != nil {
		excludeTagID = v.completeTagID
	}

	tasks, err = v.db.ListTasksFiltered(v.project.ID, search, v.selectedTag, excludeTagID)
	if err != nil {
		return err
	}
	return tasksLoadedMsg{tasks: tasks}
}

func (v *TaskListView) loadTags() tea.Msg {
	tags, err := v.db.ListTags()
	if err != nil {
		return err
	}
	return tagsLoadedMsg{tags: tags}
}

// Update handles messages
func (v *TaskListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		// Update textarea widths dynamically based on content width
		contentWidth := styles.ContentWidth(v.width)
		inputWidth := clamp(contentWidth-10, 20, 50)
		v.editDesc.SetWidth(inputWidth)
		v.editNotes.SetWidth(inputWidth)
		v.commentInput.SetWidth(inputWidth)
		return v, nil

	case tasksLoadedMsg:
		v.tasks = msg.tasks
		if v.cursor >= len(v.tasks) {
			v.cursor = max(0, len(v.tasks)-1)
		}
		// If we're in tag assignment mode, check if the task we're editing is still in the list
		if v.assigningTags && v.assigningTaskID != 0 {
			found := false
			for _, t := range v.tasks {
				if t.ID == v.assigningTaskID {
					found = true
					break
				}
			}
			if !found {
				// Task was filtered out (e.g., marked complete), close tag assignment
				v.assigningTags = false
				v.assigningTaskID = 0
			}
		}
		return v, nil

	case tagsLoadedMsg:
		v.tags = msg.tags
		return v, nil

	case commentsLoadedMsg:
		v.viewTaskComments = msg.comments
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

		if v.editing {
			return v.updateEditing(msg)
		}

		if v.viewingTask {
			return v.updateViewingTask(msg)
		}

		if v.assigningTags {
			return v.updateAssigningTags(msg)
		}

		if v.tagDropdownOpen {
			return v.updateTagDropdown(msg)
		}

		return v.updateNormal(msg)
	}

	return v, nil
}

func (v *TaskListView) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search input typing first - don't process hotkeys while typing
	if v.focus == FocusSearchInput {
		switch {
		case key.Matches(msg, v.keys.Back):
			v.searchInput.Blur()
			v.focus = FocusTaskList
			return v, nil
		case key.Matches(msg, v.keys.Enter):
			v.searchInput.Blur()
			v.focus = FocusTaskList
			return v, v.loadTasks
		default:
			var cmd tea.Cmd
			v.searchInput, cmd = v.searchInput.Update(msg)
			return v, tea.Batch(cmd, v.loadTasks)
		}
	}

	switch {
	case key.Matches(msg, v.keys.Quit):
		return v, tea.Quit

	case key.Matches(msg, v.keys.Back):
		return v, func() tea.Msg { return BackToProjects{} }

	case key.Matches(msg, v.keys.Tab):
		v.cycleFocus(1)
		return v, nil

	case msg.String() == "shift+tab":
		v.cycleFocus(-1)
		return v, nil

	case key.Matches(msg, v.keys.Up):
		if v.focus == FocusTaskList && v.cursor > 0 {
			v.cursor--
			v.ensureVisible()
		}
		return v, nil

	case key.Matches(msg, v.keys.Down):
		if v.focus == FocusTaskList && v.cursor < len(v.tasks)-1 {
			v.cursor++
			v.ensureVisible()
		}
		return v, nil

	case key.Matches(msg, v.keys.Enter):
		switch v.focus {
		case FocusBackButton:
			return v, func() tea.Msg { return BackToProjects{} }
		case FocusTagDropdown:
			v.tagDropdownOpen = true
			v.tagCursor = 0
			return v, nil
		case FocusTaskList:
			if len(v.tasks) > 0 {
				v.viewingTask = true
				return v, v.loadTaskComments
			}
		}
		return v, nil

	case key.Matches(msg, v.keys.Edit):
		if v.focus == FocusTaskList && len(v.tasks) > 0 {
			v.startEditTask(v.tasks[v.cursor])
			return v, textinput.Blink
		}
		return v, nil

	case key.Matches(msg, v.keys.New):
		v.startNewTask()
		return v, textinput.Blink

	case key.Matches(msg, v.keys.Delete):
		if v.focus == FocusTaskList && len(v.tasks) > 0 {
			v.confirmingDelete = true
			v.deleteTargetID = v.tasks[v.cursor].ID
			v.deleteTargetName = v.tasks[v.cursor].Title
			return v, nil
		}
		return v, nil

	case key.Matches(msg, v.keys.Search):
		v.focus = FocusSearchInput
		v.searchInput.Focus()
		return v, textinput.Blink

	case key.Matches(msg, v.keys.Filter):
		v.focus = FocusTagDropdown
		v.tagDropdownOpen = true
		v.tagCursor = 0
		return v, nil

	case msg.String() == "t":
		// Toggle tag assignment mode for selected task
		if v.focus == FocusTaskList && len(v.tasks) > 0 {
			v.assigningTags = true
			v.assignTagCursor = 0
			v.assigningTaskID = v.tasks[v.cursor].ID
			return v, nil
		}

	case msg.String() == "?":
		// Show help popup (useful at narrow widths)
		v.showHelpPopup = true
		return v, nil

	case key.Matches(msg, v.keys.ShowCompleted):
		// Toggle showing completed tasks
		if v.showingCompleted {
			// Going back to normal view - restore previous tag filter
			v.showingCompleted = false
			v.selectedTag = v.preCompletedTag
			v.preCompletedTag = nil
		} else {
			// Switching to completed view - save current tag filter and show only completed
			v.preCompletedTag = v.selectedTag
			v.showingCompleted = true
			v.selectedTag = v.completeTagID
		}
		v.cursor = 0
		v.scrollY = 0
		return v, v.loadTasks
	}

	return v, nil
}

func (v *TaskListView) updateTagDropdown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, v.keys.Back):
		v.tagDropdownOpen = false
		return v, nil

	case key.Matches(msg, v.keys.Up):
		if v.tagCursor > 0 {
			v.tagCursor--
		}
		return v, nil

	case key.Matches(msg, v.keys.Down):
		if v.tagCursor < len(v.tags) { // +1 for "None" option
			v.tagCursor++
		}
		return v, nil

	case key.Matches(msg, v.keys.Enter):
		if v.tagCursor == 0 {
			v.selectedTag = nil
		} else {
			tagID := v.tags[v.tagCursor-1].ID
			v.selectedTag = &tagID
		}
		v.tagDropdownOpen = false
		return v, v.loadTasks
	}

	return v, nil
}

func (v *TaskListView) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if err := v.db.DeleteTask(v.deleteTargetID); err == nil {
			v.confirmingDelete = false
			return v, v.loadTasks
		}
		v.confirmingDelete = false
		return v, nil
	case "n", "N", "esc":
		v.confirmingDelete = false
		return v, nil
	}
	return v, nil
}

func (v *TaskListView) updateViewingTask(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle comment input mode
	if v.commentInputFocused {
		switch {
		case key.Matches(msg, v.keys.Back):
			v.commentInputFocused = false
			v.commentInput.Blur()
			return v, nil
		case msg.String() == "ctrl+s":
			// Submit comment
			return v, v.submitComment()
		default:
			var cmd tea.Cmd
			v.commentInput, cmd = v.commentInput.Update(msg)
			return v, cmd
		}
	}

	switch {
	case key.Matches(msg, v.keys.Back):
		v.viewingTask = false
		v.viewTaskComments = nil
		return v, nil
	case key.Matches(msg, v.keys.Edit):
		v.viewingTask = false
		v.viewTaskComments = nil
		v.startEditTask(v.tasks[v.cursor])
		return v, textinput.Blink
	case key.Matches(msg, v.keys.Delete):
		v.confirmingDelete = true
		v.deleteTargetID = v.tasks[v.cursor].ID
		v.deleteTargetName = v.tasks[v.cursor].Title
		return v, nil
	case msg.String() == "t":
		v.viewingTask = false
		v.viewTaskComments = nil
		v.assigningTags = true
		v.assignTagCursor = 0
		v.assigningTaskID = v.tasks[v.cursor].ID
		return v, nil
	case msg.String() == "c" || msg.String() == "a":
		// Focus comment input (c for comment, a for add)
		v.commentInputFocused = true
		v.commentInput.Focus()
		return v, textarea.Blink
	case key.Matches(msg, v.keys.Quit):
		return v, tea.Quit
	}
	return v, nil
}

func (v *TaskListView) updateAssigningTags(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, v.keys.Back):
		v.assigningTags = false
		return v, nil

	case key.Matches(msg, v.keys.Up):
		if v.assignTagCursor > 0 {
			v.assignTagCursor--
		}
		return v, nil

	case key.Matches(msg, v.keys.Down):
		if v.assignTagCursor < len(v.tags)-1 {
			v.assignTagCursor++
		}
		return v, nil

	case key.Matches(msg, v.keys.Enter), msg.String() == " ":
		// Toggle the selected tag on the current task
		if len(v.tasks) > 0 && v.assignTagCursor < len(v.tags) {
			task := v.tasks[v.cursor]
			tag := v.tags[v.assignTagCursor]

			// Check if task already has this tag
			hasTag := false
			for _, t := range task.Tags {
				if t.ID == tag.ID {
					hasTag = true
					break
				}
			}

			if hasTag {
				v.db.RemoveTagFromTask(task.ID, tag.ID)
			} else {
				v.db.AddTagToTask(task.ID, tag.ID)
			}
			return v, v.loadTasks
		}
	}

	return v, nil
}

func (v *TaskListView) updateEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, v.keys.Back):
		v.editing = false
		return v, nil

	case msg.String() == "ctrl+s":
		return v, v.saveTask()

	case key.Matches(msg, v.keys.Tab):
		v.editFocusIdx = (v.editFocusIdx + 1) % 6 // 0-5: title, desc, notes, priority, tags, save
		v.updateEditFocus()
		return v, nil

	case msg.String() == "shift+tab":
		v.editFocusIdx = (v.editFocusIdx + 5) % 6
		v.updateEditFocus()
		return v, nil

	case key.Matches(msg, v.keys.Enter):
		// Enter on title or priority moves to next field
		if v.editFocusIdx == 0 || v.editFocusIdx == 3 {
			v.editFocusIdx++
			v.updateEditFocus()
			return v, nil
		}
		// Enter on tags toggles the selected tag
		if v.editFocusIdx == 4 {
			v.toggleEditTag()
			return v, nil
		}
		// Enter on save button saves
		if v.editFocusIdx == 5 {
			return v, v.saveTask()
		}
		// For textareas (desc/notes), let enter pass through for newlines

	case msg.String() == " ":
		// Space also toggles tags when in tag selector
		if v.editFocusIdx == 4 {
			v.toggleEditTag()
			return v, nil
		}

	case key.Matches(msg, v.keys.Up):
		// Navigate tag list when focused on tags
		if v.editFocusIdx == 4 && v.editTagCursor > 0 {
			v.editTagCursor--
			return v, nil
		}

	case key.Matches(msg, v.keys.Down):
		// Navigate tag list when focused on tags
		if v.editFocusIdx == 4 && v.editTagCursor < len(v.tags)-1 {
			v.editTagCursor++
			return v, nil
		}
	}

	var cmd tea.Cmd
	switch v.editFocusIdx {
	case 0:
		v.editTitle, cmd = v.editTitle.Update(msg)
	case 1:
		v.editDesc, cmd = v.editDesc.Update(msg)
	case 2:
		v.editNotes, cmd = v.editNotes.Update(msg)
	case 3:
		v.editPriority, cmd = v.editPriority.Update(msg)
	}
	return v, cmd
}

// toggleEditTag toggles the currently selected tag in the edit form
func (v *TaskListView) toggleEditTag() {
	if v.editTagCursor >= len(v.tags) {
		return
	}
	tagID := v.tags[v.editTagCursor].ID

	// Check if already selected
	for i, id := range v.editTags {
		if id == tagID {
			// Remove it
			v.editTags = append(v.editTags[:i], v.editTags[i+1:]...)
			return
		}
	}
	// Add it
	v.editTags = append(v.editTags, tagID)
}

func (v *TaskListView) cycleFocus(dir int) {
	// Blur current
	v.searchInput.Blur()

	// Cycle
	v.focus = FocusArea((int(v.focus) + dir + 4) % 4)

	// Focus search if needed
	if v.focus == FocusSearchInput {
		v.searchInput.Focus()
	}
}

func (v *TaskListView) ensureVisible() {
	// Each task item is 1 line + 1 margin = 2 lines
	availableHeight := v.height - 10
	if availableHeight < 2 {
		availableHeight = 2
	}
	visibleItems := availableHeight / 2
	if visibleItems < 1 {
		visibleItems = 1
	}

	if v.cursor < v.scrollY {
		v.scrollY = v.cursor
	} else if v.cursor >= v.scrollY+visibleItems {
		v.scrollY = v.cursor - visibleItems + 1
	}
}

func (v *TaskListView) startNewTask() {
	v.editing = true
	v.editingNew = true
	v.editFocusIdx = 0
	v.editTagCursor = 0
	v.editTags = []int64{} // No tags for new task
	v.editTitle.Reset()
	v.editDesc.Reset()
	v.editNotes.Reset()
	v.editPriority.SetValue("0")
	v.updateEditFocus()
}

func (v *TaskListView) startEditTask(task models.Task) {
	v.editing = true
	v.editingNew = false
	v.editFocusIdx = 0
	v.editTagCursor = 0
	// Copy existing tags
	v.editTags = make([]int64, len(task.Tags))
	for i, t := range task.Tags {
		v.editTags[i] = t.ID
	}
	v.editTitle.SetValue(task.Title)
	v.editDesc.SetValue(task.Description)
	v.editNotes.SetValue(task.Notes)
	v.editPriority.SetValue(fmt.Sprintf("%d", task.Priority))
	v.updateEditFocus()
}

func (v *TaskListView) updateEditFocus() {
	v.editTitle.Blur()
	v.editDesc.Blur()
	v.editNotes.Blur()
	v.editPriority.Blur()

	switch v.editFocusIdx {
	case 0:
		v.editTitle.Focus()
	case 1:
		v.editDesc.Focus()
	case 2:
		v.editNotes.Focus()
	case 3:
		v.editPriority.Focus()
	}
}

func (v *TaskListView) saveTask() tea.Cmd {
	title := strings.TrimSpace(v.editTitle.Value())
	if title == "" {
		v.editing = false
		return nil
	}

	desc := strings.TrimSpace(v.editDesc.Value())
	notes := strings.TrimSpace(v.editNotes.Value())
	priority, _ := strconv.Atoi(v.editPriority.Value())
	if priority < 0 {
		priority = 0
	}
	if priority > 10 {
		priority = 10
	}

	var taskID int64

	if v.editingNew {
		task, err := v.db.CreateTask(v.project.ID, title, desc, priority)
		if err != nil {
			v.editing = false
			return nil
		}
		taskID = task.ID
		// Update notes if any
		if notes != "" {
			v.db.UpdateTask(taskID, title, desc, notes, priority)
		}
	} else if len(v.tasks) > 0 {
		taskID = v.tasks[v.cursor].ID
		v.db.UpdateTask(taskID, title, desc, notes, priority)
	}

	// Sync tags - remove old ones and add new ones
	if taskID > 0 {
		// Get current tags on task
		currentTask, err := v.db.GetTask(taskID)
		if err == nil && currentTask != nil {
			// Remove tags that are no longer selected
			for _, existingTag := range currentTask.Tags {
				found := false
				for _, selectedID := range v.editTags {
					if existingTag.ID == selectedID {
						found = true
						break
					}
				}
				if !found {
					v.db.RemoveTagFromTask(taskID, existingTag.ID)
				}
			}
			// Add newly selected tags
			for _, selectedID := range v.editTags {
				found := false
				for _, existingTag := range currentTask.Tags {
					if existingTag.ID == selectedID {
						found = true
						break
					}
				}
				if !found {
					v.db.AddTagToTask(taskID, selectedID)
				}
			}
		}
	}

	v.editing = false
	return v.loadTasks
}

// submitComment adds a new comment to the current task
func (v *TaskListView) submitComment() tea.Cmd {
	content := strings.TrimSpace(v.commentInput.Value())
	if content == "" {
		return nil
	}

	if len(v.tasks) == 0 || v.cursor >= len(v.tasks) {
		return nil
	}

	taskID := v.tasks[v.cursor].ID
	_, err := v.db.CreateComment(taskID, content)
	if err != nil {
		return nil
	}

	// Clear the input and reload comments
	v.commentInput.Reset()
	v.commentInputFocused = false
	v.commentInput.Blur()

	return v.loadTaskComments
}

// loadTaskComments loads comments for the currently viewed task
func (v *TaskListView) loadTaskComments() tea.Msg {
	if len(v.tasks) == 0 || v.cursor >= len(v.tasks) {
		return nil
	}

	taskID := v.tasks[v.cursor].ID
	comments, err := v.db.GetTaskComments(taskID)
	if err != nil {
		return nil
	}
	return commentsLoadedMsg{comments: comments}
}

type commentsLoadedMsg struct {
	comments []models.Comment
}

// View renders the view
func (v *TaskListView) View() string {
	if v.showHelpPopup {
		return v.renderHelpPopup()
	}

	if v.confirmingDelete {
		return v.renderDeleteConfirm()
	}

	if v.editing {
		return v.renderEditForm()
	}

	if v.viewingTask {
		return v.renderTaskView()
	}

	if v.assigningTags {
		return v.renderTagAssignment()
	}

	var b strings.Builder

	// Header with back button, search, and tag filter
	b.WriteString(v.renderHeader())
	b.WriteString("\n\n")

	// Task list
	b.WriteString(v.renderTaskList())

	// Help
	b.WriteString("\n")
	b.WriteString(v.renderHelp())

	return styles.CenterView(b.String(), v.width, v.height)
}

func (v *TaskListView) renderHeader() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)
	isNarrow := contentWidth < 60

	// Search input - dynamic width
	searchStyle := s.Input
	if v.focus == FocusSearchInput {
		searchStyle = s.InputFocused
	}
	searchWidth := clamp(contentWidth-8, 10, 30)
	v.searchInput.Placeholder = "Search..."
	searchBox := searchStyle.Width(searchWidth).Render(v.searchInput.View())

	// Tag filter dropdown - show just tag name at narrow widths
	tagStyle := s.Button
	if v.focus == FocusTagDropdown {
		tagStyle = s.ButtonFocused
	}
	tagLabel := "All"
	if v.selectedTag != nil {
		for _, t := range v.tags {
			if t.ID == *v.selectedTag {
				tagLabel = t.Name
				break
			}
		}
	}
	if !isNarrow {
		tagLabel = "Tags: " + tagLabel
	}
	tagBtn := tagStyle.Render(tagLabel + " ▼")

	// Title - add indicator when viewing completed tasks
	titleText := v.project.Title
	if v.showingCompleted {
		titleText = v.project.Title + " (Completed)"
	}
	title := s.Title.Render(titleText)

	var header string
	if isNarrow {
		// Narrow: stack vertically, no back button (esc still works)
		header = lipgloss.JoinVertical(lipgloss.Left,
			searchBox,
			tagBtn,
		)
	} else {
		// Wide: horizontal with back button
		backStyle := s.Button
		if v.focus == FocusBackButton {
			backStyle = s.ButtonFocused
		}
		backBtn := backStyle.Render("← Projects")

		header = lipgloss.JoinHorizontal(lipgloss.Center,
			backBtn, "  ", searchBox, "  ", tagBtn,
		)
	}

	// Tag dropdown if open
	dropdown := ""
	if v.tagDropdownOpen {
		dropdown = "\n" + v.renderTagDropdown()
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, header+dropdown)
}

func (v *TaskListView) renderTagDropdown() string {
	s := v.styles
	var items []string

	// None option
	noneStyle := s.ListItem
	if v.tagCursor == 0 {
		noneStyle = s.ListSelected
	}
	items = append(items, noneStyle.Render("None"))

	// Tags
	for i, tag := range v.tags {
		itemStyle := s.ListItem
		if v.tagCursor == i+1 {
			itemStyle = s.ListSelected
		}
		tagColor := lipgloss.NewStyle().Foreground(lipgloss.Color(tag.Color))
		items = append(items, itemStyle.Render(tagColor.Render("●")+" "+tag.Name))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)
	return s.FilterBar.Render(content)
}

func (v *TaskListView) renderTaskList() string {
	s := v.styles

	if len(v.tasks) == 0 {
		return s.TitleMuted.Render("No tasks. Press 'n' to create one.")
	}

	// Each task item is 2 lines (title + tags) + 1 margin = 3 lines
	availableHeight := v.height - 12
	if availableHeight < 3 {
		availableHeight = 3
	}
	visibleItems := availableHeight / 3
	if visibleItems < 1 {
		visibleItems = 1
	}

	var items []string
	endIdx := min(v.scrollY+visibleItems, len(v.tasks))

	for i := v.scrollY; i < endIdx; i++ {
		task := v.tasks[i]
		items = append(items, v.renderTaskItem(task, i == v.cursor && v.focus == FocusTaskList))
	}

	return lipgloss.JoinVertical(lipgloss.Left, items...)
}

func (v *TaskListView) renderTaskItem(task models.Task, selected bool) string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)
	width := max(contentWidth-4, 20)

	// Priority indicator
	priorityStr := ""
	if task.Priority > 0 {
		priorityStr = fmt.Sprintf("[%d] ", task.Priority)
	}

	// Title line
	titleLine := priorityStr + task.Title

	// Tags line (below title, like description in project list)
	var tagsLine string
	if len(task.Tags) > 0 {
		var tagStrs []string
		for _, tag := range task.Tags {
			tagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tag.Color))//s.Tag.Background(lipgloss.Color(tag.Color)).Foreground(lipgloss.Color("#1a1b26"))
			tagStrs = append(tagStrs, tagStyle.Render(tag.Name))
		}
		tagsLine = strings.Join(tagStrs, " ")
	} else {
		tagsLine = s.TitleMuted.Render("no tags")
	}

	// Apply styling based on selection state
	var titleStyle, tagLineStyle lipgloss.Style
	if selected {
		titleStyle = s.ListSelected.Width(width)
		tagLineStyle = s.ListSelected.Width(width)
	} else {
		titleStyle = s.ListItem.Width(width)
		tagLineStyle = s.ListItem.Width(width)
	}

	title := titleStyle.Render(titleLine)
	tags := tagLineStyle.Render(tagsLine)

	// Return two-line item with margin
	return lipgloss.JoinVertical(lipgloss.Left, title, tags) + "\n"
}

func (v *TaskListView) renderEditForm() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	formTitle := "New Task"
	if !v.editingNew {
		formTitle = "Edit Task"
	}

	titleStyle := s.Input
	descStyle := s.Input
	notesStyle := s.Input
	priorityStyle := s.Input
	tagsStyle := s.Input
	btnStyle := s.Button

	switch v.editFocusIdx {
	case 0:
		titleStyle = s.InputFocused
	case 1:
		descStyle = s.InputFocused
	case 2:
		notesStyle = s.InputFocused
	case 3:
		priorityStyle = s.InputFocused
	case 4:
		tagsStyle = s.InputFocused
	case 5:
		btnStyle = s.ButtonFocused
	}

	// Dynamic input width based on content width
	inputWidth := clamp(contentWidth-6, 20, 50)

	// Build tag selector
	tagSelector := v.renderEditTagSelector(tagsStyle, inputWidth)

	form := lipgloss.JoinVertical(lipgloss.Left,
		s.Title.Render(formTitle),
		"",
		"Title:",
		titleStyle.Width(inputWidth).Render(v.editTitle.View()),
		"",
		"Description:",
		descStyle.Render(v.editDesc.View()),
		"",
		"Notes:",
		notesStyle.Render(v.editNotes.View()),
		"",
		"Priority (0-10):",
		priorityStyle.Width(10).Render(v.editPriority.View()),
		"",
		"Tags:",
		tagSelector,
		"",
		btnStyle.Render(" Save "),
		"",
		s.TitleMuted.Render("Tab: next • ↑↓: select tag • Space/↵: toggle • Ctrl+S: save • Esc: cancel"),
	)

	// Center within content width, then center that in terminal
	centered := lipgloss.Place(contentWidth, v.height,
		lipgloss.Center, lipgloss.Center,
		form,
	)
	return styles.CenterView(centered, v.width, v.height)
}

// renderEditTagSelector renders the inline tag selector for the edit form
func (v *TaskListView) renderEditTagSelector(containerStyle lipgloss.Style, width int) string {
	s := v.styles

	if len(v.tags) == 0 {
		return containerStyle.Width(width).Render(s.TitleMuted.Render("No tags available"))
	}

	var items []string
	for i, tag := range v.tags {
		// Check if this tag is selected
		isSelected := false
		for _, id := range v.editTags {
			if id == tag.ID {
				isSelected = true
				break
			}
		}

		checkbox := "[ ]"
		if isSelected {
			checkbox = "[x]"
		}

		tagColor := lipgloss.NewStyle().Foreground(lipgloss.Color(tag.Color))
		itemText := checkbox + " " + tagColor.Render("●") + " " + tag.Name

		// Highlight current cursor position when tag section is focused
		if v.editFocusIdx == 4 && i == v.editTagCursor {
			items = append(items, s.ListSelected.Render(itemText))
		} else {
			items = append(items, s.ListItem.Render(itemText))
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)
	return containerStyle.Width(width).Render(content)
}

func (v *TaskListView) renderHelp() string {
	contentWidth := styles.ContentWidth(v.width)
	// At narrow widths, show hint to press ? for help
	if contentWidth > 0 && contentWidth < 50 {
		return v.styles.Help.Render(v.styles.HelpKey.Render("?") + " help")
	}

	// Dynamic label for 'c' key based on current mode
	completedLabel := "done"
	if v.showingCompleted {
		completedLabel = "back"
	}

	return v.styles.Help.Render(
		fmt.Sprintf("%s view • %s edit • %s new • %s del • %s search • %s filter • %s tags • %s %s • %s back • %s quit",
			v.styles.HelpKey.Render("↵"),
			v.styles.HelpKey.Render("e"),
			v.styles.HelpKey.Render("n"),
			v.styles.HelpKey.Render("d"),
			v.styles.HelpKey.Render("/"),
			v.styles.HelpKey.Render("f"),
			v.styles.HelpKey.Render("t"),
			v.styles.HelpKey.Render("c"),
			completedLabel,
			v.styles.HelpKey.Render("esc"),
			v.styles.HelpKey.Render("q"),
		),
	)
}

func (v *TaskListView) renderHelpPopup() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	// Dynamic label for 'c' key based on current mode
	completedLabel := "show completed"
	if v.showingCompleted {
		completedLabel = "hide completed"
	}

	helpItems := []string{
		s.HelpKey.Render("↵") + "      view task",
		s.HelpKey.Render("e") + "      edit task",
		s.HelpKey.Render("n") + "      new task",
		s.HelpKey.Render("d") + "      delete task",
		s.HelpKey.Render("/") + "      search",
		s.HelpKey.Render("f") + "      filter by tag",
		s.HelpKey.Render("t") + "      assign tags",
		s.HelpKey.Render("c") + "      " + completedLabel,
		s.HelpKey.Render("esc") + "    back",
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

func (v *TaskListView) renderTagAssignment() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	if len(v.tasks) == 0 {
		return ""
	}

	task := v.tasks[v.cursor]

	// Build list of tags with checkmarks for assigned ones
	var items []string
	for i, tag := range v.tags {
		// Check if task has this tag
		hasTag := false
		for _, t := range task.Tags {
			if t.ID == tag.ID {
				hasTag = true
				break
			}
		}

		itemStyle := s.ListItem
		if i == v.assignTagCursor {
			itemStyle = s.ListSelected
		}

		checkbox := "[ ]"
		if hasTag {
			checkbox = "[x]"
		}

		tagColor := lipgloss.NewStyle().Foreground(lipgloss.Color(tag.Color))
		items = append(items, itemStyle.Render(checkbox+" "+tagColor.Render("●")+" "+tag.Name))
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		s.Title.Render("Assign Tags to: "+task.Title),
		"",
		lipgloss.JoinVertical(lipgloss.Left, items...),
		"",
		s.TitleMuted.Render("Enter/Space: toggle • Esc: done"),
	)

	centered := lipgloss.Place(contentWidth, v.height,
		lipgloss.Center, lipgloss.Center,
		s.FilterBar.Render(content),
	)
	return styles.CenterView(centered, v.width, v.height)
}

func (v *TaskListView) renderDeleteConfirm() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	content := lipgloss.JoinVertical(lipgloss.Center,
		s.Title.Foreground(styles.Current.Error).Render("Delete Task?"),
		"",
		// s.TitleMuted.Render(fmt.Sprintf("Are you sure you want to delete \"%s\"?", v.deleteTargetName)),
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

func (v *TaskListView) renderTaskView() string {
	if len(v.tasks) == 0 || v.cursor >= len(v.tasks) {
		return ""
	}

	s := v.styles
	task := v.tasks[v.cursor]
	maxContentWidth := styles.ContentWidth(v.width)

	// Priority display
	priorityText := "None"
	if task.Priority > 0 {
		priorityText = fmt.Sprintf("%d", task.Priority)
	}

	// Tags display
	var tagStrs []string
	for _, tag := range task.Tags {
		tagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(tag.Color))//s.Tag.Background(lipgloss.Color(tag.Color)).Foreground(lipgloss.Color("#1a1b26"))
		tagStrs = append(tagStrs, tagStyle.Render(tag.Name))
	}
	tagsLine := "None"
	if len(tagStrs) > 0 {
		tagsLine = strings.Join(tagStrs, " ")
	}

	// Description
	descText := task.Description
	if descText == "" {
		descText = s.TitleMuted.Render("No description")
	}

	// Notes
	notesText := task.Notes
	if notesText == "" {
		notesText = s.TitleMuted.Render("No notes")
	}

	// Build the view - use content width for text wrapping
	titleStyle := s.Title.MarginBottom(1)
	labelStyle := s.TitleMuted
	textWidth := clamp(maxContentWidth-10, 20, 70)

	// Update comment input width
	v.commentInput.SetWidth(clamp(textWidth, 20, 50))

	// Build comments section
	var commentsContent string
	if len(v.viewTaskComments) == 0 {
		commentsContent = s.TitleMuted.Render("No comments yet")
	} else {
		var commentLines []string
		for _, comment := range v.viewTaskComments {
			timestamp := comment.CreatedAt.Format("Jan 2, 2006 3:04 PM")
			timestampStyle := s.TitleMuted
			commentLine := lipgloss.JoinVertical(lipgloss.Left,
				timestampStyle.Render(timestamp),
				lipgloss.NewStyle().Width(textWidth).Render(comment.Content),
			)
			commentLines = append(commentLines, commentLine)
		}
		commentsContent = lipgloss.JoinVertical(lipgloss.Left, commentLines...)
	}

	// Comment input styling
	commentInputStyle := s.Input
	if v.commentInputFocused {
		commentInputStyle = s.InputFocused
	}

	// Help text changes based on whether comment input is focused
	var helpText string
	if v.commentInputFocused {
		helpText = s.Help.Render(
			fmt.Sprintf("%s submit • %s cancel",
				s.HelpKey.Render("ctrl+s"),
				s.HelpKey.Render("esc"),
			),
		)
	} else {
		helpText = s.Help.Render(
			fmt.Sprintf("%s edit • %s tags • %s delete • %s comment • %s back",
				s.HelpKey.Render("e"),
				s.HelpKey.Render("t"),
				s.HelpKey.Render("d"),
				s.HelpKey.Render("c"),
				s.HelpKey.Render("esc"),
			),
		)
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(task.Title),
		"",
		labelStyle.Render("Priority"),
		s.TaskPriority.Render(priorityText),
		"",
		labelStyle.Render("Tags"),
		tagsLine,
		"",
		labelStyle.Render("Description"),
		lipgloss.NewStyle().Width(textWidth).Render(descText),
		"",
		labelStyle.Render("Notes"),
		lipgloss.NewStyle().Width(textWidth).Render(notesText),
		"",
		labelStyle.Render("Comments"),
		commentsContent,
		"",
		commentInputStyle.Render(v.commentInput.View()),
		"",
		helpText,
	)

	// Return with padding, not centered vertically, but horizontally centered if wide
	padded := lipgloss.NewStyle().Padding(1, 2).Render(content)
	return styles.CenterView(padded, v.width, v.height)
}
