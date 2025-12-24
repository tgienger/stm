package db

import (
	"github.com/tgienger/stm/internal/models"
)

// CreateProject creates a new project
func (db *DB) CreateProject(title, description string) (*models.Project, error) {
	result, err := db.Exec(`
		INSERT INTO projects (title, description) VALUES (?, ?)
	`, title, description)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return db.GetProject(id)
}

// GetProject retrieves a project by ID
func (db *DB) GetProject(id int64) (*models.Project, error) {
	p := &models.Project{}
	err := db.QueryRow(`
		SELECT id, title, description, created_at, updated_at 
		FROM projects WHERE id = ?
	`, id).Scan(&p.ID, &p.Title, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// ListProjects returns all projects
func (db *DB) ListProjects() ([]models.Project, error) {
	rows, err := db.Query(`
		SELECT id, title, description, created_at, updated_at 
		FROM projects ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// UpdateProject updates a project
func (db *DB) UpdateProject(id int64, title, description string) error {
	_, err := db.Exec(`
		UPDATE projects SET title = ?, description = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, title, description, id)
	return err
}

// DeleteProject deletes a project and all its tasks
func (db *DB) DeleteProject(id int64) error {
	_, err := db.Exec("DELETE FROM projects WHERE id = ?", id)
	return err
}

// ProjectCount returns the number of projects
func (db *DB) ProjectCount() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&count)
	return count, err
}
