package views

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tgienger/stm/internal/fizzy"
	"github.com/tgienger/stm/internal/models"
	"github.com/tgienger/stm/internal/ui/keys"
	"github.com/tgienger/stm/internal/ui/styles"
)

func clamp(val, minVal, maxVal int) int {
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

type FocusArea int

const (
	FocusBackButton FocusArea = iota
	FocusSearchInput
	FocusTagDropdown
	FocusCardList
)

type CardListView struct {
	fizzy    *fizzy.Fizzy
	settings *fizzy.Settings
	board    models.Board
	cards    []models.Card
	tags     []models.Tag
	styles   *styles.Styles
	keys     keys.KeyMap

	width  int
	height int

	// Columns
	columns                []models.Column
	currentColumn          int // 0 = All, 1..N = column index+1
	pendingRestoreColumnID string

	focus       FocusArea
	cursor      int
	scrollY     int
	searchInput textinput.Model
	selectedTag string // empty = no filter

	tagDropdownOpen bool
	tagCursor       int

	creatingColumn bool
	newColumnName  textinput.Model

	editing       bool
	editingNew    bool
	editTitle     textinput.Model
	editDesc      textarea.Model
	editFocusIdx  int // 0=title, 1=desc, 2=tags, 3=save
	editTags      []string
	editTagCursor int

	assigningTags   bool
	assignTagCursor int
	assigningCardID int

	viewingCard         bool
	viewCardComments    []models.Comment
	commentInput        textarea.Model
	commentInputFocused bool

	confirmingDelete bool
	deleteTargetID   int
	deleteTargetName string

	confirmingDeleteColumn bool
	deleteColumnID         string
	deleteColumnName       string

	confirmingDiscard bool
	originalTitle     string
	originalDesc      string
	originalTags      []string

	loadingCards bool

	showHelpPopup bool
}

func NewCardListView(f *fizzy.Fizzy, settings *fizzy.Settings, board models.Board) *CardListView {
	s := styles.NewStyles()

	search := textinput.New()
	search.Placeholder = "Search cards..."
	search.CharLimit = 100

	editTitle := textinput.New()
	editTitle.Placeholder = "Card title"
	editTitle.CharLimit = 200

	editDesc := textarea.New()
	editDesc.Placeholder = "Description"
	editDesc.CharLimit = 1000
	editDesc.SetWidth(50)
	editDesc.SetHeight(3)
	editDesc.ShowLineNumbers = false

	commentInput := textarea.New()
	commentInput.Placeholder = "Add a comment..."
	commentInput.CharLimit = 2000
	commentInput.SetWidth(50)
	commentInput.SetHeight(3)
	commentInput.ShowLineNumbers = false

	newColumnName := textinput.New()
	newColumnName.Placeholder = "Column name"
	newColumnName.CharLimit = 100

	return &CardListView{
		fizzy:                  f,
		settings:               settings,
		board:                  board,
		styles:                 s,
		keys:                   keys.DefaultKeyMap(),
		focus:                  FocusCardList,
		searchInput:            search,
		editTitle:              editTitle,
		editDesc:               editDesc,
		newColumnName:          newColumnName,
		commentInput:           commentInput,
		loadingCards:           true,
		pendingRestoreColumnID: settings.Get(lastColumnSettingKey(board.ID)),
	}
}

type BackToBoards struct{}

func (v *CardListView) Init() tea.Cmd {
	return tea.Batch(v.loadTags, v.loadColumns)
}

type cardsLoadedMsg struct {
	cards []models.Card
}

type cardsLoadErrorMsg struct {
	err error
}

type tagsLoadedMsg struct {
	tags []models.Tag
}

type columnsLoadedMsg struct {
	columns []models.Column
}

func (v *CardListView) loadCards() tea.Msg {
	v.loadingCards = true
	var cards []models.Card
	var err error

	if v.currentColumn > 0 && v.currentColumn <= len(v.columns) {
		col := v.columns[v.currentColumn-1]
		cards, err = v.fizzy.ListCardsByColumn(v.board.ID, col.ID, col.Pseudo)
	} else {
		cards, err = v.fizzy.ListCards(v.board.ID)
	}
	if err != nil {
		return cardsLoadErrorMsg{err: err}
	}
	return cardsLoadedMsg{cards: cards}
}

func (v *CardListView) loadTags() tea.Msg {
	tags, err := v.fizzy.ListTags()
	if err != nil {
		return err
	}
	return tagsLoadedMsg{tags: tags}
}

func (v *CardListView) loadColumns() tea.Msg {
	columns, err := v.fizzy.ListColumns(v.board.ID)
	if err != nil {
		return err
	}
	return columnsLoadedMsg{columns: columns}
}

func (v *CardListView) filteredCards() []models.Card {
	search := strings.ToLower(strings.TrimSpace(v.searchInput.Value()))
	var result []models.Card
	for _, c := range v.cards {
		if search != "" && !strings.Contains(strings.ToLower(c.Title), search) &&
			!strings.Contains(strings.ToLower(c.Description), search) {
			continue
		}
		if v.selectedTag != "" {
			found := false
			for _, t := range c.Tags {
				if t == v.selectedTag {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		result = append(result, c)
	}
	return result
}

func (v *CardListView) clampVisibleState() {
	filtered := v.filteredCards()
	if len(filtered) == 0 {
		v.cursor = 0
		v.scrollY = 0
		return
	}

	if v.cursor >= len(filtered) {
		v.cursor = len(filtered) - 1
	}
	if v.scrollY > v.cursor {
		v.scrollY = v.cursor
	}
	if v.scrollY >= len(filtered) {
		v.scrollY = len(filtered) - 1
	}
	if v.scrollY < 0 {
		v.scrollY = 0
	}
}

func (v *CardListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		contentWidth := styles.ContentWidth(v.width)
		inputWidth := clamp(contentWidth-10, 20, 50)
		v.editDesc.SetWidth(inputWidth)
		v.commentInput.SetWidth(inputWidth)
		return v, nil

	case cardsLoadedMsg:
		v.cards = msg.cards
		v.loadingCards = false
		v.clampVisibleState()
		if v.assigningTags && v.assigningCardID != 0 {
			found := false
			for _, c := range v.cards {
				if c.Number == v.assigningCardID {
					found = true
					break
				}
			}
			if !found {
				v.assigningTags = false
				v.assigningCardID = 0
			}
		}
		return v, nil

	case cardsLoadErrorMsg:
		v.loadingCards = false
		v.cards = nil
		return v, nil

	case tagsLoadedMsg:
		v.tags = msg.tags
		return v, nil

	case columnsLoadedMsg:
		v.columns = msg.columns
		v.restoreSavedColumn()
		return v, v.loadCards

	case commentsLoadedMsg:
		v.viewCardComments = msg.comments
		return v, nil

	case tea.KeyMsg:
		if v.showHelpPopup {
			v.showHelpPopup = false
			return v, nil
		}

		if v.confirmingDelete {
			return v.updateConfirmDelete(msg)
		}

		if v.confirmingDeleteColumn {
			return v.updateConfirmDeleteColumn(msg)
		}

		if v.confirmingDiscard {
			return v.updateConfirmDiscard(msg)
		}

		if v.creatingColumn {
			return v.updateCreatingColumn(msg)
		}

		if v.editing {
			return v.updateEditing(msg)
		}

		if v.viewingCard {
			return v.updateViewingCard(msg)
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

func (v *CardListView) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if v.focus == FocusSearchInput {
		switch {
		case key.Matches(msg, v.keys.Back):
			v.searchInput.Blur()
			v.focus = FocusCardList
			return v, nil
		case key.Matches(msg, v.keys.Enter):
			v.searchInput.Blur()
			v.focus = FocusCardList
			return v, v.loadCards
		default:
			var cmd tea.Cmd
			v.searchInput, cmd = v.searchInput.Update(msg)
			v.clampVisibleState()
			return v, tea.Batch(cmd, v.loadCards)
		}
	}

	switch {
	case key.Matches(msg, v.keys.Quit):
		return v, tea.Quit

	case key.Matches(msg, v.keys.Back):
		return v, func() tea.Msg { return BackToBoards{} }

	case key.Matches(msg, v.keys.Tab):
		v.cycleFocus(1)
		return v, nil

	case msg.String() == "shift+tab":
		v.cycleFocus(-1)
		return v, nil

	case key.Matches(msg, v.keys.Up):
		if v.focus == FocusCardList && v.cursor > 0 {
			v.cursor--
			v.ensureVisible()
		}
		return v, nil

	case key.Matches(msg, v.keys.Down):
		if v.focus == FocusCardList && v.cursor < len(v.cards)-1 {
			v.cursor++
			v.ensureVisible()
		}
		return v, nil

	case key.Matches(msg, v.keys.Enter):
		switch v.focus {
		case FocusBackButton:
			return v, func() tea.Msg { return BackToBoards{} }
		case FocusTagDropdown:
			v.tagDropdownOpen = true
			v.tagCursor = 0
			return v, nil
		case FocusCardList:
			if len(v.cards) > 0 {
				v.viewingCard = true
				return v, v.loadCardComments
			}
		}
		return v, nil

	case key.Matches(msg, v.keys.Edit):
		if v.focus == FocusCardList && len(v.cards) > 0 {
			v.startEditCard(v.cards[v.cursor])
			return v, textinput.Blink
		}
		return v, nil

	case key.Matches(msg, v.keys.New):
		v.startNewCard()
		return v, textinput.Blink

	case msg.String() == "C":
		v.creatingColumn = true
		v.newColumnName.Reset()
		v.newColumnName.Focus()
		return v, textinput.Blink

	case key.Matches(msg, v.keys.Delete):
		if v.focus == FocusCardList && len(v.cards) > 0 {
			v.confirmingDelete = true
			v.deleteTargetID = v.cards[v.cursor].Number
			v.deleteTargetName = v.cards[v.cursor].Title
			return v, nil
		}
		return v, nil

	case msg.String() == "X":
		if col := v.currentRealColumn(); col != nil {
			v.confirmingDeleteColumn = true
			v.deleteColumnID = col.ID
			v.deleteColumnName = col.Name
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
		if v.focus == FocusCardList && len(v.cards) > 0 {
			v.assigningTags = true
			v.assignTagCursor = 0
			v.assigningCardID = v.cards[v.cursor].Number
			return v, nil
		}

	case msg.String() == "?":
		v.showHelpPopup = true
		return v, nil

	case key.Matches(msg, v.keys.Left):
		if v.focus != FocusSearchInput && v.currentColumn > 0 {
			v.currentColumn--
			v.saveCurrentColumn()
			v.cards = nil
			v.loadingCards = true
			v.cursor = 0
			v.scrollY = 0
			return v, v.loadCards
		}

	case key.Matches(msg, v.keys.Right):
		if v.focus != FocusSearchInput && v.currentColumn < len(v.columns) {
			v.currentColumn++
			v.saveCurrentColumn()
			v.cards = nil
			v.loadingCards = true
			v.cursor = 0
			v.scrollY = 0
			return v, v.loadCards
		}
	}

	return v, nil
}

func (v *CardListView) updateTagDropdown(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		if v.tagCursor < len(v.tags) {
			v.tagCursor++
		}
		return v, nil

	case key.Matches(msg, v.keys.Enter):
		if v.tagCursor == 0 {
			v.selectedTag = ""
		} else {
			v.selectedTag = v.tags[v.tagCursor-1].Title
		}
		v.tagDropdownOpen = false
		v.clampVisibleState()
		return v, v.loadCards
	}

	return v, nil
}

func (v *CardListView) updateConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if err := v.fizzy.DeleteCard(v.deleteTargetID); err == nil {
			v.confirmingDelete = false
			v.viewingCard = false
			v.viewCardComments = nil
			return v, v.loadCards
		}
		v.confirmingDelete = false
		return v, nil
	case "n", "N", "esc":
		v.confirmingDelete = false
		return v, nil
	}
	return v, nil
}

func (v *CardListView) updateConfirmDeleteColumn(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if err := v.fizzy.DeleteColumn(v.board.ID, v.deleteColumnID); err == nil {
			v.confirmingDeleteColumn = false
			v.deleteColumnID = ""
			v.deleteColumnName = ""
			v.currentColumn = 0
			v.saveCurrentColumn()
			v.cards = nil
			v.loadingCards = true
			v.cursor = 0
			v.scrollY = 0
			return v, tea.Batch(v.loadColumns, v.loadCards)
		}
		v.confirmingDeleteColumn = false
		v.deleteColumnID = ""
		v.deleteColumnName = ""
		return v, nil
	case "n", "N", "esc":
		v.confirmingDeleteColumn = false
		v.deleteColumnID = ""
		v.deleteColumnName = ""
		return v, nil
	}
	return v, nil
}

func (v *CardListView) updateConfirmDiscard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		v.confirmingDiscard = false
		v.editing = false
		return v, nil
	case "s", "S":
		v.confirmingDiscard = false
		return v, v.saveCard()
	case "n", "N", "esc":
		v.confirmingDiscard = false
		return v, nil
	}
	return v, nil
}

func (v *CardListView) updateCreatingColumn(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, v.keys.Back):
		v.creatingColumn = false
		v.newColumnName.Reset()
		v.newColumnName.Blur()
		return v, nil

	case msg.String() == "ctrl+s":
		return v, v.createColumn()

	case key.Matches(msg, v.keys.Enter):
		return v, v.createColumn()
	}

	var cmd tea.Cmd
	v.newColumnName, cmd = v.newColumnName.Update(msg)
	return v, cmd
}

func (v *CardListView) updateViewingCard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if v.commentInputFocused {
		switch {
		case key.Matches(msg, v.keys.Back):
			v.commentInputFocused = false
			v.commentInput.Blur()
			return v, nil
		case msg.String() == "ctrl+s":
			return v, v.submitComment()
		default:
			var cmd tea.Cmd
			v.commentInput, cmd = v.commentInput.Update(msg)
			return v, cmd
		}
	}

	switch {
	case key.Matches(msg, v.keys.Back):
		v.viewingCard = false
		v.viewCardComments = nil
		return v, nil
	case key.Matches(msg, v.keys.Edit):
		v.viewingCard = false
		v.viewCardComments = nil
		v.startEditCard(v.cards[v.cursor])
		return v, textinput.Blink
	case key.Matches(msg, v.keys.Delete):
		v.confirmingDelete = true
		v.deleteTargetID = v.cards[v.cursor].Number
		v.deleteTargetName = v.cards[v.cursor].Title
		return v, nil
	case msg.String() == "t":
		v.viewingCard = false
		v.viewCardComments = nil
		v.assigningTags = true
		v.assignTagCursor = 0
		v.assigningCardID = v.cards[v.cursor].Number
		return v, nil
	case msg.String() == "c" || msg.String() == "a":
		v.commentInputFocused = true
		v.commentInput.Focus()
		return v, textarea.Blink
	case key.Matches(msg, v.keys.Quit):
		return v, tea.Quit
	}
	return v, nil
}

func (v *CardListView) updateAssigningTags(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		if len(v.cards) > 0 && v.assignTagCursor < len(v.tags) {
			card := v.cards[v.cursor]
			tag := v.tags[v.assignTagCursor]

			hasTag := false
			for _, t := range card.Tags {
				if t == tag.Title {
					hasTag = true
					break
				}
			}

			v.fizzy.TagCard(card.Number, tag.Title, hasTag)
			return v, v.loadCards
		}
	}

	return v, nil
}

func (v *CardListView) updateEditing(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, v.keys.Back):
		if v.hasUnsavedChanges() {
			v.confirmingDiscard = true
			return v, nil
		}
		v.editing = false
		return v, nil

	case msg.String() == "ctrl+s":
		return v, v.saveCard()

	case key.Matches(msg, v.keys.Tab):
		v.editFocusIdx = (v.editFocusIdx + 1) % 4 // 0-3: title, desc, tags, save
		v.updateEditFocus()
		return v, nil

	case msg.String() == "shift+tab":
		v.editFocusIdx = (v.editFocusIdx + 3) % 4
		v.updateEditFocus()
		return v, nil

	case key.Matches(msg, v.keys.Enter):
		if v.editFocusIdx == 0 {
			v.editFocusIdx++
			v.updateEditFocus()
			return v, nil
		}
		if v.editFocusIdx == 2 {
			v.toggleEditTag()
			return v, nil
		}
		if v.editFocusIdx == 3 {
			return v, v.saveCard()
		}

	case msg.String() == " ":
		if v.editFocusIdx == 2 {
			v.toggleEditTag()
			return v, nil
		}

	case key.Matches(msg, v.keys.Up):
		if v.editFocusIdx == 2 && v.editTagCursor > 0 {
			v.editTagCursor--
			return v, nil
		}

	case key.Matches(msg, v.keys.Down):
		if v.editFocusIdx == 2 && v.editTagCursor < len(v.tags)-1 {
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
	}
	return v, cmd
}

func (v *CardListView) toggleEditTag() {
	if v.editTagCursor >= len(v.tags) {
		return
	}
	tagTitle := v.tags[v.editTagCursor].Title

	for i, t := range v.editTags {
		if t == tagTitle {
			v.editTags = append(v.editTags[:i], v.editTags[i+1:]...)
			return
		}
	}
	v.editTags = append(v.editTags, tagTitle)
}

func (v *CardListView) cycleFocus(dir int) {
	v.searchInput.Blur()
	v.focus = FocusArea((int(v.focus) + dir + 4) % 4)
	if v.focus == FocusSearchInput {
		v.searchInput.Focus()
	}
}

func (v *CardListView) ensureVisible() {
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

func (v *CardListView) startNewCard() {
	v.editing = true
	v.editingNew = true
	v.editFocusIdx = 0
	v.editTagCursor = 0
	v.editTags = []string{}
	v.editTitle.Reset()
	v.editDesc.Reset()
	v.updateEditFocus()

	v.originalTitle = ""
	v.originalDesc = ""
	v.originalTags = []string{}
}

func (v *CardListView) startEditCard(card models.Card) {
	v.editing = true
	v.editingNew = false
	v.editFocusIdx = 0
	v.editTagCursor = 0
	v.editTags = make([]string, len(card.Tags))
	copy(v.editTags, card.Tags)
	v.editTitle.SetValue(card.Title)
	v.editDesc.SetValue(card.Description)
	v.updateEditFocus()

	v.originalTitle = card.Title
	v.originalDesc = card.Description
	v.originalTags = make([]string, len(card.Tags))
	copy(v.originalTags, card.Tags)
}

func (v *CardListView) hasUnsavedChanges() bool {
	if v.editTitle.Value() != v.originalTitle {
		return true
	}
	if v.editDesc.Value() != v.originalDesc {
		return true
	}
	if len(v.editTags) != len(v.originalTags) {
		return true
	}
	for i, t := range v.editTags {
		if t != v.originalTags[i] {
			return true
		}
	}
	return false
}

func (v *CardListView) updateEditFocus() {
	v.editTitle.Blur()
	v.editDesc.Blur()

	switch v.editFocusIdx {
	case 0:
		v.editTitle.Focus()
	case 1:
		v.editDesc.Focus()
	}
}

func (v *CardListView) saveCard() tea.Cmd {
	title := strings.TrimSpace(v.editTitle.Value())
	if title == "" {
		v.editing = false
		return nil
	}

	desc := strings.TrimSpace(v.editDesc.Value())

	if v.editingNew {
		card, err := v.fizzy.CreateCard(v.board.ID, title, desc)
		if err != nil {
			v.editing = false
			return nil
		}
		// Apply tags
		for _, tagTitle := range v.editTags {
			v.fizzy.TagCard(card.Number, tagTitle, false)
		}
	} else if len(v.cards) > 0 {
		card := v.cards[v.cursor]
		v.fizzy.UpdateCard(card.Number, title, desc)

		// Sync tags - remove old, add new
		for _, existingTag := range card.Tags {
			found := false
			for _, selected := range v.editTags {
				if existingTag == selected {
					found = true
					break
				}
			}
			if !found {
				v.fizzy.TagCard(card.Number, existingTag, true)
			}
		}
		for _, selected := range v.editTags {
			found := false
			for _, existingTag := range card.Tags {
				if existingTag == selected {
					found = true
					break
				}
			}
			if !found {
				v.fizzy.TagCard(card.Number, selected, false)
			}
		}
	}

	v.editing = false
	return v.loadCards
}

func (v *CardListView) createColumn() tea.Cmd {
	name := strings.TrimSpace(v.newColumnName.Value())
	if name == "" {
		return nil
	}

	column, err := v.fizzy.CreateColumn(v.board.ID, name)
	if err != nil {
		v.creatingColumn = false
		v.newColumnName.Reset()
		v.newColumnName.Blur()
		return nil
	}

	v.creatingColumn = false
	v.newColumnName.Reset()
	v.newColumnName.Blur()
	v.pendingRestoreColumnID = column.ID
	return v.loadColumns
}

func (v *CardListView) submitComment() tea.Cmd {
	content := strings.TrimSpace(v.commentInput.Value())
	if content == "" {
		return nil
	}

	if len(v.cards) == 0 || v.cursor >= len(v.cards) {
		return nil
	}

	cardNumber := v.cards[v.cursor].Number
	_, err := v.fizzy.CreateComment(cardNumber, content)
	if err != nil {
		return nil
	}

	v.commentInput.Reset()
	v.commentInputFocused = false
	v.commentInput.Blur()

	return v.loadCardComments
}

func (v *CardListView) loadCardComments() tea.Msg {
	if len(v.cards) == 0 || v.cursor >= len(v.cards) {
		return nil
	}

	cardNumber := v.cards[v.cursor].Number
	comments, err := v.fizzy.ListComments(cardNumber)
	if err != nil {
		return nil
	}
	return commentsLoadedMsg{comments: comments}
}

type commentsLoadedMsg struct {
	comments []models.Comment
}

// View renders the card list view
func (v *CardListView) View() string {
	if v.showHelpPopup {
		return v.renderHelpPopup()
	}

	if v.confirmingDelete {
		return v.renderDeleteConfirm()
	}

	if v.confirmingDeleteColumn {
		return v.renderDeleteColumnConfirm()
	}

	if v.confirmingDiscard {
		return v.renderDiscardConfirm()
	}

	if v.creatingColumn {
		return v.renderCreateColumnForm()
	}

	if v.editing {
		return v.renderEditForm()
	}

	if v.viewingCard {
		return v.renderCardView()
	}

	if v.assigningTags {
		return v.renderTagAssignment()
	}

	var b strings.Builder

	b.WriteString(v.renderHeader())
	b.WriteString("\n\n")

	b.WriteString(v.renderCardList())

	b.WriteString("\n")
	b.WriteString(v.renderHelp())

	return styles.CenterView(b.String(), v.width, v.height)
}

func (v *CardListView) renderHeader() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)
	isNarrow := contentWidth < 60

	searchStyle := s.Input
	if v.focus == FocusSearchInput {
		searchStyle = s.InputFocused
	}
	searchWidth := clamp(contentWidth-8, 10, 30)
	v.searchInput.Placeholder = "Search..."
	searchBox := searchStyle.Width(searchWidth).Render(v.searchInput.View())

	tagStyle := s.Button
	if v.focus == FocusTagDropdown {
		tagStyle = s.ButtonFocused
	}
	tagLabel := "All"
	if v.selectedTag != "" {
		tagLabel = v.selectedTag
	}
	if !isNarrow {
		tagLabel = "Tags: " + tagLabel
	}
	tagBtn := tagStyle.Render(tagLabel + " ▼")

	titleText := v.board.Name
	title := s.Title.Render(titleText)

	// Column indicator
	columnBar := v.renderColumnBar()

	var header string
	if isNarrow {
		header = lipgloss.JoinVertical(lipgloss.Left,
			searchBox,
			tagBtn,
		)
	} else {
		backStyle := s.Button
		if v.focus == FocusBackButton {
			backStyle = s.ButtonFocused
		}
		backBtn := backStyle.Render("← Boards")

		header = lipgloss.JoinHorizontal(lipgloss.Center,
			backBtn, "  ", searchBox, "  ", tagBtn,
		)
	}

	dropdown := ""
	if v.tagDropdownOpen {
		dropdown = "\n" + v.renderTagDropdown()
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, columnBar, header+dropdown)
}

func (v *CardListView) renderColumnBar() string {
	s := v.styles

	// Build column names: All | Col1 | Col2 | ...
	var items []string
	allName := "All"
	if v.currentColumn == 0 {
		items = append(items, s.ListSelected.Render(allName))
	} else {
		items = append(items, s.ListItem.Render(allName))
	}
	for i, col := range v.columns {
		if i+1 == v.currentColumn {
			items = append(items, s.ListSelected.Render(col.Name))
		} else {
			items = append(items, s.ListItem.Render(col.Name))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, items...)
}

func (v *CardListView) renderTagDropdown() string {
	s := v.styles
	var items []string

	noneStyle := s.ListItem
	if v.tagCursor == 0 {
		noneStyle = s.ListSelected
	}
	items = append(items, noneStyle.Render("None"))

	for i, tag := range v.tags {
		itemStyle := s.ListItem
		if v.tagCursor == i+1 {
			itemStyle = s.ListSelected
		}
		items = append(items, itemStyle.Render(tag.Title))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)
	return s.FilterBar.Render(content)
}

func (v *CardListView) renderCardList() string {
	s := v.styles

	if v.loadingCards {
		return s.TitleMuted.Render("Loading...")
	}

	filtered := v.filteredCards()
	if len(filtered) == 0 {
		return s.TitleMuted.Render("No cards. Press 'n' to create one.")
	}

	availableHeight := v.height - 12
	if availableHeight < 2 {
		availableHeight = 2
	}
	visibleItems := availableHeight / 2
	if visibleItems < 1 {
		visibleItems = 1
	}

	var items []string
	endIdx := min(v.scrollY+visibleItems, len(filtered))

	for i := v.scrollY; i < endIdx; i++ {
		card := filtered[i]
		items = append(items, v.renderCardItem(card, i == v.cursor && v.focus == FocusCardList))
	}

	return lipgloss.JoinVertical(lipgloss.Left, items...)
}

func (v *CardListView) renderCardItem(card models.Card, selected bool) string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)
	width := max(contentWidth-4, 20)

	// Title with card number
	titleLine := fmt.Sprintf("#%d %s", card.Number, card.Title)

	// Tags line
	var tagsLine string
	if len(card.Tags) > 0 {
		tagsLine = strings.Join(card.Tags, " ")
	} else {
		tagsLine = s.TitleMuted.Render("no tags")
	}

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

	return lipgloss.JoinVertical(lipgloss.Left, title, tags) + "\n"
}

func (v *CardListView) renderEditForm() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	formTitle := "New Card"
	if !v.editingNew {
		formTitle = "Edit Card"
	}

	titleStyle := s.Input
	descStyle := s.Input
	tagsStyle := s.Input
	btnStyle := s.Button

	switch v.editFocusIdx {
	case 0:
		titleStyle = s.InputFocused
	case 1:
		descStyle = s.InputFocused
	case 2:
		tagsStyle = s.InputFocused
	case 3:
		btnStyle = s.ButtonFocused
	}

	inputWidth := clamp(contentWidth-6, 20, 50)
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
		"Tags:",
		tagSelector,
		"",
		btnStyle.Render(" Save "),
		"",
		s.TitleMuted.Render("Tab: next • ↑↓: select tag • Space/↵: toggle • Ctrl+S: save • Esc: cancel"),
	)

	centered := lipgloss.Place(contentWidth, v.height,
		lipgloss.Center, lipgloss.Center,
		form,
	)
	return styles.CenterView(centered, v.width, v.height)
}

func (v *CardListView) renderEditTagSelector(containerStyle lipgloss.Style, width int) string {
	s := v.styles

	if len(v.tags) == 0 {
		return containerStyle.Width(width).Render(s.TitleMuted.Render("No tags available"))
	}

	var items []string
	for i, tag := range v.tags {
		isSelected := false
		for _, t := range v.editTags {
			if t == tag.Title {
				isSelected = true
				break
			}
		}

		checkbox := "[ ]"
		if isSelected {
			checkbox = "[x]"
		}

		itemText := checkbox + " " + tag.Title

		if v.editFocusIdx == 2 && i == v.editTagCursor {
			items = append(items, s.ListSelected.Render(itemText))
		} else {
			items = append(items, s.ListItem.Render(itemText))
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)
	return containerStyle.Width(width).Render(content)
}

func (v *CardListView) renderHelp() string {
	contentWidth := styles.ContentWidth(v.width)
	if contentWidth > 0 && contentWidth < 50 {
		return v.styles.Help.Render(v.styles.HelpKey.Render("?") + " help")
	}

	return v.styles.Help.Render(
		fmt.Sprintf("%s view • %s edit • %s new card • %s del card • %s new col • %s del col • %s search • %s filter • %s tags • %s←→ %s • %s back • %s quit",
			v.styles.HelpKey.Render("↵"),
			v.styles.HelpKey.Render("e"),
			v.styles.HelpKey.Render("n"),
			v.styles.HelpKey.Render("d"),
			v.styles.HelpKey.Render("C"),
			v.styles.HelpKey.Render("X"),
			v.styles.HelpKey.Render("/"),
			v.styles.HelpKey.Render("f"),
			v.styles.HelpKey.Render("t"),
			v.styles.HelpKey.Render("h"),
			v.currentColumnName(),
			v.styles.HelpKey.Render("esc"),
			v.styles.HelpKey.Render("q"),
		),
	)
}

func (v *CardListView) currentColumnName() string {
	if v.currentColumn == 0 {
		return "All"
	}
	if v.currentColumn <= len(v.columns) {
		return v.columns[v.currentColumn-1].Name
	}
	return "All"
}

func (v *CardListView) renderHelpPopup() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	helpItems := []string{
		s.HelpKey.Render("↵") + "      view card",
		s.HelpKey.Render("e") + "      edit card",
		s.HelpKey.Render("n") + "      new card",
		s.HelpKey.Render("d") + "      delete card",
		s.HelpKey.Render("C") + "      create column",
		s.HelpKey.Render("X") + "      delete column",
		s.HelpKey.Render("/") + "      search",
		s.HelpKey.Render("f") + "      filter by tag",
		s.HelpKey.Render("t") + "      assign tags",
		s.HelpKey.Render("h/l") + "     switch column",
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

func (v *CardListView) renderTagAssignment() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	if len(v.cards) == 0 {
		return ""
	}

	card := v.cards[v.cursor]

	var items []string
	for i, tag := range v.tags {
		hasTag := false
		for _, t := range card.Tags {
			if t == tag.Title {
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

		items = append(items, itemStyle.Render(checkbox+" "+tag.Title))
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		s.Title.Render("Assign Tags to: "+card.Title),
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

func (v *CardListView) renderDeleteConfirm() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	content := lipgloss.JoinVertical(lipgloss.Center,
		s.Title.Foreground(styles.Current.Error).Render("Delete Card?"),
		"",
		s.TitleMuted.Render(v.deleteTargetName),
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

func (v *CardListView) renderDeleteColumnConfirm() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)

	content := lipgloss.JoinVertical(lipgloss.Center,
		s.Title.Foreground(styles.Current.Error).Render("Delete Column?"),
		"",
		s.TitleMuted.Render(v.deleteColumnName),
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

func (v *CardListView) renderCreateColumnForm() string {
	s := v.styles
	contentWidth := styles.ContentWidth(v.width)
	inputWidth := clamp(contentWidth-6, 20, 50)

	form := lipgloss.JoinVertical(lipgloss.Left,
		s.Title.Render("New Column"),
		"",
		"Name:",
		s.InputFocused.Width(inputWidth).Render(v.newColumnName.View()),
		"",
		s.TitleMuted.Render("Enter/Ctrl+S: create • Esc: cancel"),
	)

	centered := lipgloss.Place(contentWidth, v.height,
		lipgloss.Center, lipgloss.Center,
		form,
	)
	return styles.CenterView(centered, v.width, v.height)
}

func (v *CardListView) renderDiscardConfirm() string {
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

func (v *CardListView) renderCardView() string {
	if len(v.cards) == 0 || v.cursor >= len(v.cards) {
		return ""
	}

	s := v.styles
	card := v.cards[v.cursor]
	maxContentWidth := styles.ContentWidth(v.width)
	columnName := v.cardColumnName(card)

	// Tags display
	var tagsLine string
	if len(card.Tags) > 0 {
		tagsLine = strings.Join(card.Tags, " ")
	} else {
		tagsLine = "None"
	}

	// Description
	descText := card.Description
	if descText == "" {
		descText = s.TitleMuted.Render("No description")
	}

	titleStyle := s.Title.MarginBottom(1)
	labelStyle := s.TitleMuted
	textWidth := clamp(maxContentWidth-10, 20, 70)

	v.commentInput.SetWidth(clamp(textWidth, 20, 50))

	// Comments section
	userComments, latestSystemComment := splitCardComments(v.viewCardComments)

	var systemContent string
	if latestSystemComment != nil {
		systemContent = lipgloss.NewStyle().Width(textWidth).Render(
			fmt.Sprintf("%s: %s",
				latestSystemComment.CreatedAt.Format("Jan 2, 2006 3:04 PM"),
				latestSystemComment.Body,
			),
		)
	} else {
		systemContent = s.TitleMuted.Render("No system messages")
	}

	var commentsContent string
	if len(userComments) == 0 {
		commentsContent = s.TitleMuted.Render("No comments yet")
	} else {
		var commentLines []string
		for _, comment := range userComments {
			timestamp := comment.CreatedAt.Format("Jan 2, 2006 3:04 PM")
			commentLine := lipgloss.JoinVertical(lipgloss.Left,
				labelStyle.Render(timestamp),
				lipgloss.NewStyle().Width(textWidth).Render(comment.Body),
			)
			commentLines = append(commentLines, commentLine)
		}
		commentsContent = lipgloss.JoinVertical(lipgloss.Left, appendInterleaved(commentLines, "")...)
	}

	commentInputStyle := s.Input
	if v.commentInputFocused {
		commentInputStyle = s.InputFocused
	}

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
			fmt.Sprintf("%s edit • %s tags • %s close • %s comment • %s back",
				s.HelpKey.Render("e"),
				s.HelpKey.Render("t"),
				s.HelpKey.Render("d"),
				s.HelpKey.Render("c"),
				s.HelpKey.Render("esc"),
			),
		)
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(fmt.Sprintf("#%d %s", card.Number, card.Title)),
		"",
		labelStyle.Render("Column"),
		columnName,
		"",
		labelStyle.Render("Tags"),
		tagsLine,
		"",
		labelStyle.Render("Description"),
		lipgloss.NewStyle().Width(textWidth).Render(descText),
		"",
		labelStyle.Render("Latest System Message"),
		systemContent,
		"",
		commentInputStyle.Render(v.commentInput.View()),
		"",
		labelStyle.Render("Comments"),
		commentsContent,
		"",
		helpText,
	)

	padded := lipgloss.NewStyle().Padding(1, 2).Render(content)
	return styles.CenterView(padded, v.width, v.height)
}

func (v *CardListView) cardColumnName(card models.Card) string {
	if card.ColumnName != "" {
		return card.ColumnName
	}
	if card.ColumnID != "" {
		for _, col := range v.columns {
			if col.ID == card.ColumnID {
				return col.Name
			}
		}
	}
	return "Unassigned"
}

func splitCardComments(comments []models.Comment) ([]models.Comment, *models.Comment) {
	userComments := make([]models.Comment, 0, len(comments))
	var latestSystemComment *models.Comment

	for _, comment := range comments {
		if isSystemComment(comment) {
			if latestSystemComment == nil || comment.CreatedAt.After(latestSystemComment.CreatedAt) {
				commentCopy := comment
				latestSystemComment = &commentCopy
			}
			continue
		}
		userComments = append(userComments, comment)
	}

	sort.Slice(userComments, func(i, j int) bool {
		return userComments[i].CreatedAt.After(userComments[j].CreatedAt)
	})

	return userComments, latestSystemComment
}

func isSystemComment(comment models.Comment) bool {
	role := strings.TrimSpace(strings.ToLower(comment.Role))
	author := strings.TrimSpace(strings.ToLower(comment.Author))
	return role == "system" || author == "system" || author == "sytem"
}

func (v *CardListView) restoreSavedColumn() bool {
	if v.pendingRestoreColumnID == "" {
		return false
	}

	savedColumnID := v.pendingRestoreColumnID
	v.pendingRestoreColumnID = ""

	for i, col := range v.columns {
		if col.ID == savedColumnID {
			if v.currentColumn != i+1 {
				v.currentColumn = i + 1
				v.cursor = 0
				v.scrollY = 0
				v.cards = nil
				v.loadingCards = true
				v.saveCurrentColumn()
				return true
			}
			return false
		}
	}

	v.currentColumn = 0
	v.saveCurrentColumn()
	return false
}

func (v *CardListView) saveCurrentColumn() {
	if v.settings == nil {
		return
	}
	_ = v.settings.Set(lastColumnSettingKey(v.board.ID), v.currentColumnID())
}

func (v *CardListView) currentColumnID() string {
	if v.currentColumn > 0 && v.currentColumn <= len(v.columns) {
		return v.columns[v.currentColumn-1].ID
	}
	return ""
}

func (v *CardListView) currentRealColumn() *models.Column {
	if v.currentColumn == 0 || v.currentColumn > len(v.columns) {
		return nil
	}

	col := v.columns[v.currentColumn-1]
	if col.Pseudo {
		return nil
	}

	return &col
}

func lastColumnSettingKey(boardID string) string {
	return "last_column_id:" + boardID
}

func appendInterleaved(items []string, separator string) []string {
	if len(items) < 2 {
		return items
	}

	interleaved := make([]string, 0, len(items)*2-1)
	for i, item := range items {
		if i > 0 {
			interleaved = append(interleaved, separator)
		}
		interleaved = append(interleaved, item)
	}
	return interleaved
}
