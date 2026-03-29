package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/tgienger/stm/internal/fizzy"
	"github.com/tgienger/stm/internal/models"
	"github.com/tgienger/stm/internal/ui/views"
)

type View int

const (
	ViewBoards View = iota
	ViewCards
)

type App struct {
	fizzy       *fizzy.Fizzy
	settings    *fizzy.Settings
	currentView View
	boardList   *views.BoardListView
	cardList    *views.CardListView
	width       int
	height      int
}

type initialBoardsLoadedMsg struct {
	boards []models.Board
	err    error
}

func NewApp(f *fizzy.Fizzy, s *fizzy.Settings) *App {
	return &App{
		fizzy:       f,
		settings:    s,
		currentView: ViewBoards,
		boardList:   views.NewBoardListView(f),
	}
}

func (a *App) Init() tea.Cmd {
	return a.loadInitialBoards
}

func (a *App) loadInitialBoards() tea.Msg {
	boards, err := a.fizzy.ListBoards()
	return initialBoardsLoadedMsg{boards: boards, err: err}
}

func (a *App) openBoard(board models.Board) tea.Cmd {
	a.currentView = ViewCards
	a.cardList = views.NewCardListView(a.fizzy, a.settings, board)

	_ = a.settings.Set("last_board_id", board.ID)

	return tea.Batch(
		a.cardList.Init(),
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
		a.boardList.Update(msg)

	case initialBoardsLoadedMsg:
		if msg.err != nil {
			return a, nil
		}

		a.boardList.SetBoards(msg.boards)

		lastBoardID := a.settings.Get("last_board_id")
		if lastBoardID == "" {
			return a, nil
		}

		for _, board := range msg.boards {
			if board.ID == lastBoardID {
				return a, a.openBoard(board)
			}
		}

		_ = a.settings.Set("last_board_id", "")
		return a, nil

	case views.SelectedBoard:
		return a, a.openBoard(msg.Board)

	case views.BackToBoards:
		a.currentView = ViewBoards
		return a, tea.Batch(
			a.boardList.Init(),
			func() tea.Msg {
				return tea.WindowSizeMsg{Width: a.width, Height: a.height}
			},
		)
	}

	var cmd tea.Cmd
	switch a.currentView {
	case ViewBoards:
		_, cmd = a.boardList.Update(msg)
	case ViewCards:
		_, cmd = a.cardList.Update(msg)
	}

	return a, cmd
}

func (a *App) View() string {
	switch a.currentView {
	case ViewCards:
		if a.cardList != nil {
			return a.cardList.View()
		}
	}
	return a.boardList.View()
}
