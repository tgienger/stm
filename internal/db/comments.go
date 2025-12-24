package db

import (
	"github.com/tgienger/stm/internal/models"
)

// CreateComment creates a new comment on a task
func (db *DB) CreateComment(taskID int64, content string) (*models.Comment, error) {
	result, err := db.Exec(`
		INSERT INTO comments (task_id, content) VALUES (?, ?)
	`, taskID, content)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return db.GetComment(id)
}

// GetComment retrieves a comment by ID
func (db *DB) GetComment(id int64) (*models.Comment, error) {
	c := &models.Comment{}
	err := db.QueryRow(`
		SELECT id, task_id, content, created_at 
		FROM comments WHERE id = ?
	`, id).Scan(&c.ID, &c.TaskID, &c.Content, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// GetTaskComments retrieves all comments for a task, ordered by creation time (oldest first)
func (db *DB) GetTaskComments(taskID int64) ([]models.Comment, error) {
	rows, err := db.Query(`
		SELECT id, task_id, content, created_at 
		FROM comments 
		WHERE task_id = ?
		ORDER BY created_at ASC
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []models.Comment
	for rows.Next() {
		var c models.Comment
		if err := rows.Scan(&c.ID, &c.TaskID, &c.Content, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// DeleteComment deletes a comment
func (db *DB) DeleteComment(id int64) error {
	_, err := db.Exec("DELETE FROM comments WHERE id = ?", id)
	return err
}
