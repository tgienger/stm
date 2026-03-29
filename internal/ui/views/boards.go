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
	"github.com/tgienger/stm/internal/fizzy"
	"github.com/tgienger/stm/internal/models"
	"github.com/tgienger/stm/internal/ui/keys"
	"github.com/tgienger/stm/internal/ui/styles"
)

type boardItem struct {
	board models.Board
}

func (i boardItem) Title() string       { return i.board.Name }
func (i boardItem) Description() string { return "" }
func (i boardItem) FilterValue() string { return i.board.Name }

type boardDelegate struct {
	styles *styles.Styles
	width  int
}

func (d boardDelegate) Height() int                               { return 2 }
func (d boardDelegate) Spacing() int                              { return 1 }
func (d boardDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }

func (d boardDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	b, ok := item.(boardItem)
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

	title := titleStyle.Render(b.Title())
	desc := descStyle.Render(b.Description())

	fmt.Fprintf(w, "%s\n%s", title, desc)
}

type BoardListView struct {
	fizzy            *fizzy.Fizzy
	list             list.Model
	delegate         *boardDelegate
	styles           *styles.Styles
	keys             keys.KeyMap
	width            int
	height           int
	creating         bool
	loaded           bool
	confirmingDelete bool
	deleteTargetID   string
	deleteTargetName string
	newName          textinput.Model
	focusIdx         int

	confirmingDiscard bool
	originalName      string

	showHelpPopup bool
}

func NewBoardListView(f *fizzy.Fizzy) *BoardListView {
	s := styles.NewStyles()

	newName := textinput.New()
	newName.Placeholder = "Board name"
	newName.CharLimit = 100

	delegate := &boardDelegate{styles: s, width: 80}

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Boards"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = s.Title
	l.SetShowHelp(false)

	return &BoardListView{
		fizzy:    f,
		list:     l,
		delegate: delegate,
		styles:   s,
		keys:     keys.DefaultKeyMap(),
		newName:  newName,
	}
}

func (v *BoardListView) Init() tea.Cmd {
	return v.loadBoards
}

func (v *BoardListView) loadBoards() tea.Msg {
	boards, err := v.fizzy.ListBoards()
	if err != nil {
		return err
	}
	return boardsLoadedMsg{boards: boards}
}

func (v *BoardListView) SetBoards(boards []models.Board) {
	items := make([]list.Item, len(boards))
	for i, b := range boards {
		items[i] = boardItem{board: b}
	}
	v.list.SetItems(items)
	v.loaded = true
}

type boardsLoadedMsg struct {
	boards []models.Board
}

type SelectedBoard struct {
	Board models.Board
}

func (v *BoardListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		contentWidth := styles.ContentWidth(msg.Width)
		v.delegate.width = contentWidth
		v.list.SetSize(contentWidth-4, msg.Height-6)
		return v, nil

	case boardsLoadedMsg:
		v.SetBoards(msg.boards)
		return v, nil

	case tea.KeyMsg:
		if v.showHelpPopup {
			v.showHelpPopup = false
			return v, nil
		}

		if v.confirmingDelete {
			return v.updateConfirmDelete(msg)
		}

		if v.confirmingDiscard {
			return v.updateConfirmDiscard(msg)
		}

		if v.creating {
			return v.updateCreating(msg)
		}

		switch {
		case key.Matches(msg, v.keys.Quit):
			return v, tea.Quit
		case key.Matches(msg, v.keys.Back):
			return v, nil
		case key.Matches(msg, v.keys.New):
			v.creating = true
			v.focusIdx = 0
			v.newName.Reset()
			v.newName.Focus()
			v.originalName = ""
			return v, textinput.Blink
		case msg.String() == "?":
			v.showHelpPopup = true
			return v, nil
		case key.Matches(msg, v.keys.Enter):
			if item, ok := v.list.SelectedItem().(boardItem); ok {
				return v, func() tea.Msg {
					return SelectedBoard{Board: item.board}
				}
			}
		case key.Matches(msg, v.keys.Delete):
			if item, ok := v.list.SelectedItem().(boardItem); ok {
				v.confirmingDelete = true
				v.deleteTargetID = item.board.ID
				v.deleteTargetName = item.board.Name
				return v, nil
			}
		}
	}

	var cmd tea.Cmd
	v.list, cmd = v.list.Update(msg)
	return v, cmd
}

func (v *BoardListView) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if err := v.fizzy.DeleteBoard(v.deleteTargetID); err == nil {
			v.confirmingDelete = false
			return v, v.loadBoards
		}
		v.confirmingDelete = false
		return v, nil
	case "n", "N", "esc":
		v.confirmingDelete = false
		return v, nil
	}
	return v, nil
}

func (v *BoardListView) updateConfirmDiscard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		v.confirmingDiscard = false
		v.creating = false
		return v, nil
	case "s", "S":
		v.confirmingDiscard = false
		name := strings.TrimSpace(v.newName.Value())
		if name != "" {
			board, err := v.fizzy.CreateBoard(name)
			if err == nil {
				v.creating = false
				return v, func() tea.Msg {
					return SelectedBoard{Board: *board}
				}
			}
		}
		return v, nil
	case "n", "N", "esc":
		v.confirmingDiscard = false
		return v, nil
	}
	return v, nil
}

func (v *BoardListView) updateCreating(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, v.keys.Back):
		if v.hasUnsavedChanges() {
			v.confirmingDiscard = true
			return v, nil
		}
		v.creating = false
		return v, nil

	case msg.String() == "ctrl+s":
		name := strings.TrimSpace(v.newName.Value())
		if name != "" {
			board, err := v.fizzy.CreateBoard(name)
			if err == nil {
				v.creating = false
				return v, func() tea.Msg {
					return SelectedBoard{Board: *board}
				}
			}
		}
		return v, nil

	case key.Matches(msg, v.keys.Enter):
		name := strings.TrimSpace(v.newName.Value())
		if name != "" {
			board, err := v.fizzy.CreateBoard(name)
			if err == nil {
				v.creating = false
				return v, func() tea.Msg {
					return SelectedBoard{Board: *board}
				}
			}
		}
		return v, nil
	}

	var cmd tea.Cmd
	v.newName, cmd = v.newName.Update(msg)
	return v, cmd
}

func (v *BoardListView) hasUnsavedChanges() bool {
	return v.newName.Value() != v.originalName
}

func (v *BoardListView) View() string {
	if v.showHelpPopup {
		return v.renderHelpPopup()
	}

	if v.confirmingDelete {
		return v.renderDeleteConfirm()
	}

	if v.confirmingDiscard {
		return v.renderDiscardConfirm()
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

func (v *BoardListView) renderEmpty() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	content := lipgloss.JoinVertical(lipgloss.Center,
		s.Title.Render("No Boards"),
		"",
		s.TitleMuted.Render("Press 'n' to create your first board"),
		"",
		s.ButtonPrimary.Render(" New Board "),
	)

	centered := lipgloss.Place(contentWidth, v.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
	return styles.CenterView(centered, v.width, v.height)
}

func (v *BoardListView) renderCreateForm() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	nameStyle := s.Input
	btnStyle := s.Button

	switch v.focusIdx {
	case 0:
		nameStyle = s.InputFocused
	case 1:
		btnStyle = s.ButtonFocused
	}

	inputWidth := clamp(contentWidth-6, 20, 50)

	form := lipgloss.JoinVertical(lipgloss.Left,
		s.Title.Render("New Board"),
		"",
		"Name:",
		nameStyle.Width(inputWidth).Render(v.newName.View()),
		"",
		btnStyle.Render(" Create "),
		"",
		s.TitleMuted.Render("↵: create • Esc: cancel"),
	)

	centered := lipgloss.Place(contentWidth, v.height,
		lipgloss.Center, lipgloss.Center,
		form,
	)
	return styles.CenterView(centered, v.width, v.height)
}

func (v *BoardListView) renderDiscardConfirm() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	content := lipgloss.JoinVertical(lipgloss.Center,
		s.Title.Foreground(styles.Current.Warning).Render("Discard unsaved changes?"),
		"",
		"",
		lipgloss.JoinHorizontal(lipgloss.Center,
			s.ButtonPrimary.Render(" Y - Discard "),
			"  ",
			s.Button.Render(" S - Save "),
			"  ",
			s.Button.Render(" N - Cancel "),
		),
	)

	centered := lipgloss.Place(contentWidth, v.height,
		lipgloss.Center, lipgloss.Center,
		content,
	)
	return styles.CenterView(centered, v.width, v.height)
}

func (v *BoardListView) renderHelp() string {
	contentWidth := styles.ContentWidth(v.width)
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

func (v *BoardListView) renderHelpPopup() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	helpItems := []string{
		s.HelpKey.Render("↵") + "      select board",
		s.HelpKey.Render("n") + "      new board",
		s.HelpKey.Render("d") + "      delete board",
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

func (v *BoardListView) renderDeleteConfirm() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	content := lipgloss.JoinVertical(lipgloss.Center,
		s.Title.Foreground(styles.Current.Error).Render("Delete Board?"),
		"",
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
