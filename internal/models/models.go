package models

import "time"

// Board represents a Fizzy board (project)
type Board struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// Card represents a card on a board
type Card struct {
	ID          string
	Number      int
	Title       string
	Description string
	Tags        []string
	ColumnID    string
	ColumnName  string
	CreatedAt   time.Time
}

// Column represents a column on a board
type Column struct {
	ID     string
	Name   string
	Pseudo bool
}

// Tag represents a Fizzy tag
type Tag struct {
	ID    string
	Title string
}

// Comment represents a comment on a card
type Comment struct {
	ID        string
	Body      string
	Author    string
	Role      string
	CreatedAt time.Time
}
