package models

import "time"

// Project represents a task management project
type Project struct {
	ID          int64
	Title       string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TagGroup represents a group of mutually exclusive tags
type TagGroup struct {
	ID        int64
	Name      string
	CreatedAt time.Time
}

// Tag represents a tag that can be applied to tasks
type Tag struct {
	ID         int64
	Name       string
	Color      string
	TagGroupID *int64 // nil if not part of a group
	CreatedAt  time.Time
}

// Comment represents a comment on a task
type Comment struct {
	ID        int64
	TaskID    int64
	Content   string
	CreatedAt time.Time
}

// Task represents a single task
type Task struct {
	ID          int64
	ProjectID   int64
	Title       string
	Description string
	Notes       string
	Priority    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Tags        []Tag     // populated when loading tasks
	Comments    []Comment // populated when loading task details
}
