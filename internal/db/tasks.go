package db

import (
	"github.com/tgienger/stm/internal/models"
)

// CreateTask creates a new task
func (db *DB) CreateTask(projectID int64, title, description string, priority int) (*models.Task, error) {
	result, err := db.Exec(`
		INSERT INTO tasks (project_id, title, description, priority) VALUES (?, ?, ?, ?)
	`, projectID, title, description, priority)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return db.GetTask(id)
}

// GetTask retrieves a task by ID with its tags
func (db *DB) GetTask(id int64) (*models.Task, error) {
	t := &models.Task{}
	err := db.QueryRow(`
		SELECT id, project_id, title, description, notes, priority, created_at, updated_at 
		FROM tasks WHERE id = ?
	`, id).Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Notes, &t.Priority, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}

	tags, err := db.GetTaskTags(id)
	if err != nil {
		return nil, err
	}
	t.Tags = tags

	return t, nil
}

// ListTasks returns all tasks for a project, ordered by priority (desc) then created_at (desc)
func (db *DB) ListTasks(projectID int64) ([]models.Task, error) {
	rows, err := db.Query(`
		SELECT id, project_id, title, description, notes, priority, created_at, updated_at 
		FROM tasks 
		WHERE project_id = ?
		ORDER BY priority DESC, created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Notes, &t.Priority, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load tags for each task
	for i := range tasks {
		tags, err := db.GetTaskTags(tasks[i].ID)
		if err != nil {
			return nil, err
		}
		tasks[i].Tags = tags
	}

	return tasks, nil
}

// ListTasksFiltered returns tasks filtered by search query and/or tag
// excludeTagID allows excluding tasks that have a specific tag (used to hide completed tasks)
func (db *DB) ListTasksFiltered(projectID int64, search string, tagID *int64, excludeTagID *int64) ([]models.Task, error) {
	query := `
		SELECT DISTINCT t.id, t.project_id, t.title, t.description, t.notes, t.priority, t.created_at, t.updated_at 
		FROM tasks t
	`
	args := []interface{}{projectID}

	if tagID != nil {
		query += " JOIN task_tags tt ON t.id = tt.task_id"
	}

	query += " WHERE t.project_id = ?"

	if search != "" {
		query += " AND (t.title LIKE ? OR t.description LIKE ?)"
		searchPattern := "%" + search + "%"
		args = append(args, searchPattern, searchPattern)
	}

	if tagID != nil {
		query += " AND tt.tag_id = ?"
		args = append(args, *tagID)
	}

	if excludeTagID != nil {
		query += " AND t.id NOT IN (SELECT task_id FROM task_tags WHERE tag_id = ?)"
		args = append(args, *excludeTagID)
	}

	query += " ORDER BY t.priority DESC, t.created_at DESC"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var t models.Task
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Title, &t.Description, &t.Notes, &t.Priority, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load tags for each task
	for i := range tasks {
		tags, err := db.GetTaskTags(tasks[i].ID)
		if err != nil {
			return nil, err
		}
		tasks[i].Tags = tags
	}

	return tasks, nil
}

// UpdateTask updates a task
func (db *DB) UpdateTask(id int64, title, description, notes string, priority int) error {
	_, err := db.Exec(`
		UPDATE tasks SET title = ?, description = ?, notes = ?, priority = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, title, description, notes, priority, id)
	return err
}

// DeleteTask deletes a task
func (db *DB) DeleteTask(id int64) error {
	_, err := db.Exec("DELETE FROM tasks WHERE id = ?", id)
	return err
}

// GetTaskTags returns all tags for a task
func (db *DB) GetTaskTags(taskID int64) ([]models.Tag, error) {
	rows, err := db.Query(`
		SELECT t.id, t.name, t.color, t.tag_group_id, t.created_at
		FROM tags t
		JOIN task_tags tt ON t.id = tt.tag_id
		WHERE tt.task_id = ?
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []models.Tag
	for rows.Next() {
		var t models.Tag
		if err := rows.Scan(&t.ID, &t.Name, &t.Color, &t.TagGroupID, &t.CreatedAt); err != nil {
			return nil, err
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// AddTagToTask adds a tag to a task, enforcing radio-button behavior for grouped tags
func (db *DB) AddTagToTask(taskID, tagID int64) error {
	// Get the tag to check if it belongs to a group
	var tagGroupID *int64
	err := db.QueryRow("SELECT tag_group_id FROM tags WHERE id = ?", tagID).Scan(&tagGroupID)
	if err != nil {
		return err
	}

	// If the tag belongs to a group, remove any existing tags from that group
	if tagGroupID != nil {
		_, err = db.Exec(`
			DELETE FROM task_tags 
			WHERE task_id = ? AND tag_id IN (
				SELECT id FROM tags WHERE tag_group_id = ?
			)
		`, taskID, *tagGroupID)
		if err != nil {
			return err
		}
	}

	// Add the new tag
	_, err = db.Exec(`
		INSERT OR IGNORE INTO task_tags (task_id, tag_id) VALUES (?, ?)
	`, taskID, tagID)
	return err
}

// RemoveTagFromTask removes a tag from a task
func (db *DB) RemoveTagFromTask(taskID, tagID int64) error {
	_, err := db.Exec("DELETE FROM task_tags WHERE task_id = ? AND tag_id = ?", taskID, tagID)
	return err
}
