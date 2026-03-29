package fizzy

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/tgienger/stm/internal/models"
)

// Fizzy wraps calls to the fizzy CLI
type Fizzy struct {
	binPath string
}

// New creates a new Fizzy client
func New() (*Fizzy, error) {
	binPath, err := exec.LookPath("fizzy")
	if err != nil {
		return nil, fmt.Errorf("fizzy CLI not found in PATH: %w", err)
	}
	return &Fizzy{binPath: binPath}, nil
}

// jsonEnvelope is the standard response envelope from fizzy CLI
type jsonEnvelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Error   *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (f *Fizzy) run(args ...string) (json.RawMessage, error) {
	out, err := exec.Command(f.binPath, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("fizzy %s: %w\n%s", strings.Join(args, " "), err, out)
	}

	var env jsonEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		return nil, fmt.Errorf("fizzy: failed to parse response: %w", err)
	}
	if !env.Success {
		msg := "unknown error"
		if env.Error != nil {
			msg = env.Error.Message
		}
		return nil, fmt.Errorf("fizzy: %s", msg)
	}
	return env.Data, nil
}

// --- Boards ---

func (f *Fizzy) ListBoards() ([]models.Board, error) {
	data, err := f.run("board", "list")
	if err != nil {
		return nil, err
	}

	var raw []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	boards := make([]models.Board, len(raw))
	for i, r := range raw {
		boards[i] = models.Board{
			ID:        r.ID,
			Name:      r.Name,
			CreatedAt: parseTime(r.CreatedAt),
		}
	}
	return boards, nil
}

func (f *Fizzy) CreateBoard(name string) (*models.Board, error) {
	data, err := f.run("board", "create", "--name", name)
	if err != nil {
		return nil, err
	}

	var raw struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	return &models.Board{
		ID:        raw.ID,
		Name:      raw.Name,
		CreatedAt: parseTime(raw.CreatedAt),
	}, nil
}

func (f *Fizzy) DeleteBoard(id string) error {
	_, err := f.run("board", "delete", id)
	return err
}

// --- Cards ---

func (f *Fizzy) ListCards(boardID string) ([]models.Card, error) {
	return f.listCards(boardID, "", false)
}

// ListCardsByColumn returns cards in a specific column (works with both real and pseudo column IDs).
func (f *Fizzy) ListCardsByColumn(boardID, columnID string, includeClosed bool) ([]models.Card, error) {
	return f.listCards(boardID, columnID, includeClosed)
}

func (f *Fizzy) listCards(boardID, columnID string, includeClosed bool) ([]models.Card, error) {
	args := []string{"card", "list", "--board", boardID, "--all"}
	if columnID != "" {
		args = append(args, "--column", columnID)
	}
	data, err := f.run(args...)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		ID          string   `json:"id"`
		Number      int      `json:"number"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
		Closed      bool     `json:"closed"`
		Column      *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"column"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	cards := make([]models.Card, 0, len(raw))
	for _, r := range raw {
		colID := ""
		colName := ""
		if r.Column != nil {
			colID = r.Column.ID
			colName = r.Column.Name
		}
		if !includeClosed && colID == "done" {
			continue
		}
		cards = append(cards, models.Card{
			ID:          r.ID,
			Number:      r.Number,
			Title:       r.Title,
			Description: r.Description,
			Tags:        r.Tags,
			ColumnID:    colID,
			ColumnName:  colName,
			CreatedAt:   parseTime(r.CreatedAt),
		})
	}
	return cards, nil
}

func (f *Fizzy) CreateCard(boardID, title, description string) (*models.Card, error) {
	args := []string{"card", "create", "--board", boardID, "--title", title}
	if description != "" {
		args = append(args, "--description", description)
	}

	data, err := f.run(args...)
	if err != nil {
		return nil, err
	}

	var raw struct {
		ID          string   `json:"id"`
		Number      int      `json:"number"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
		Column      *struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"column"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	colID := ""
	colName := ""
	if raw.Column != nil {
		colID = raw.Column.ID
		colName = raw.Column.Name
	}

	return &models.Card{
		ID:          raw.ID,
		Number:      raw.Number,
		Title:       raw.Title,
		Description: raw.Description,
		Tags:        raw.Tags,
		ColumnID:    colID,
		ColumnName:  colName,
		CreatedAt:   parseTime(raw.CreatedAt),
	}, nil
}

func (f *Fizzy) UpdateCard(number int, title, description string) error {
	args := []string{"card", "update", fmt.Sprintf("%d", number)}
	if title != "" {
		args = append(args, "--title", title)
	}
	if description != "" {
		args = append(args, "--description", description)
	}
	_, err := f.run(args...)
	return err
}

func (f *Fizzy) CloseCard(number int) error {
	_, err := f.run("card", "close", fmt.Sprintf("%d", number))
	return err
}

func (f *Fizzy) ReopenCard(number int) error {
	_, err := f.run("card", "reopen", fmt.Sprintf("%d", number))
	return err
}

func (f *Fizzy) DeleteCard(number int) error {
	_, err := f.run("card", "delete", fmt.Sprintf("%d", number))
	return err
}

// TagCard toggles a tag on a card. If the card has the tag, it removes it; otherwise adds it.
func (f *Fizzy) TagCard(cardNumber int, tagName string, hasTag bool) error {
	// fizzy card tag is a toggle, so we only call it if we need to change state
	_, err := f.run("card", "tag", fmt.Sprintf("%d", cardNumber), "--tag", tagName)
	return err
}

// MoveCardToColumn moves a card to a specific column
func (f *Fizzy) MoveCardToColumn(cardNumber int, columnID string) error {
	_, err := f.run("card", "column", fmt.Sprintf("%d", cardNumber), "--column", columnID)
	return err
}

// --- Columns ---

func (f *Fizzy) ListColumns(boardID string) ([]models.Column, error) {
	data, err := f.run("column", "list", "--board", boardID)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Pseudo bool   `json:"pseudo"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	columns := make([]models.Column, len(raw))
	for i, r := range raw {
		columns[i] = models.Column{
			ID:     r.ID,
			Name:   r.Name,
			Pseudo: r.Pseudo,
		}
	}
	return columns, nil
}

func (f *Fizzy) CreateColumn(boardID, name string) (*models.Column, error) {
	data, err := f.run("column", "create", "--board", boardID, "--name", name)
	if err != nil {
		return nil, err
	}

	var raw struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Pseudo bool   `json:"pseudo"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	return &models.Column{
		ID:     raw.ID,
		Name:   raw.Name,
		Pseudo: raw.Pseudo,
	}, nil
}

func (f *Fizzy) DeleteColumn(boardID, columnID string) error {
	_, err := f.run("column", "delete", columnID, "--board", boardID)
	return err
}

// --- Tags ---

func (f *Fizzy) ListTags() ([]models.Tag, error) {
	data, err := f.run("tag", "list")
	if err != nil {
		return nil, err
	}

	var raw []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	tags := make([]models.Tag, len(raw))
	for i, r := range raw {
		tags[i] = models.Tag{
			ID:    r.ID,
			Title: r.Title,
		}
	}
	return tags, nil
}

// --- Comments ---

func (f *Fizzy) ListComments(cardNumber int) ([]models.Comment, error) {
	data, err := f.run("comment", "list", "--card", fmt.Sprintf("%d", cardNumber), "--all")
	if err != nil {
		return nil, err
	}

	var raw []struct {
		ID   string `json:"id"`
		Body struct {
			PlainText string `json:"plain_text"`
		} `json:"body"`
		Creator struct {
			Name string `json:"name"`
			Role string `json:"role"`
		} `json:"creator"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var comments []models.Comment
	for _, r := range raw {
		comments = append(comments, models.Comment{
			ID:        r.ID,
			Body:      r.Body.PlainText,
			Author:    r.Creator.Name,
			Role:      r.Creator.Role,
			CreatedAt: parseTime(r.CreatedAt),
		})
	}
	return comments, nil
}

func (f *Fizzy) CreateComment(cardNumber int, body string) (*models.Comment, error) {
	data, err := f.run("comment", "create", "--card", fmt.Sprintf("%d", cardNumber), "--body", body)
	if err != nil {
		return nil, err
	}

	var raw struct {
		ID   string `json:"id"`
		Body struct {
			PlainText string `json:"plain_text"`
		} `json:"body"`
		Creator struct {
			Name string `json:"name"`
			Role string `json:"role"`
		} `json:"creator"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	return &models.Comment{
		ID:        raw.ID,
		Body:      raw.Body.PlainText,
		Author:    raw.Creator.Name,
		Role:      raw.Creator.Role,
		CreatedAt: parseTime(raw.CreatedAt),
	}, nil
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
