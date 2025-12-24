package db

import (
	"github.com/tgienger/stm/internal/models"
)

// CreateTagGroup creates a new tag group
func (db *DB) CreateTagGroup(name string) (*models.TagGroup, error) {
	result, err := db.Exec("INSERT INTO tag_groups (name) VALUES (?)", name)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return db.GetTagGroup(id)
}

// GetTagGroup retrieves a tag group by ID
func (db *DB) GetTagGroup(id int64) (*models.TagGroup, error) {
	g := &models.TagGroup{}
	err := db.QueryRow("SELECT id, name, created_at FROM tag_groups WHERE id = ?", id).
		Scan(&g.ID, &g.Name, &g.CreatedAt)
	if err != nil {
		return nil, err
	}
	return g, nil
}

// ListTagGroups returns all tag groups
func (db *DB) ListTagGroups() ([]models.TagGroup, error) {
	rows, err := db.Query("SELECT id, name, created_at FROM tag_groups ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []models.TagGroup
	for rows.Next() {
		var g models.TagGroup
		if err := rows.Scan(&g.ID, &g.Name, &g.CreatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// DeleteTagGroup deletes a tag group (tags in the group will have their group_id set to NULL)
func (db *DB) DeleteTagGroup(id int64) error {
	_, err := db.Exec("DELETE FROM tag_groups WHERE id = ?", id)
	return err
}

// CreateTag creates a new tag
func (db *DB) CreateTag(name, color string, tagGroupID *int64) (*models.Tag, error) {
	result, err := db.Exec("INSERT INTO tags (name, color, tag_group_id) VALUES (?, ?, ?)", name, color, tagGroupID)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return db.GetTag(id)
}

// GetTag retrieves a tag by ID
func (db *DB) GetTag(id int64) (*models.Tag, error) {
	t := &models.Tag{}
	err := db.QueryRow("SELECT id, name, color, tag_group_id, created_at FROM tags WHERE id = ?", id).
		Scan(&t.ID, &t.Name, &t.Color, &t.TagGroupID, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// ListTags returns all tags
func (db *DB) ListTags() ([]models.Tag, error) {
	rows, err := db.Query("SELECT id, name, color, tag_group_id, created_at FROM tags ORDER BY name")
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

// ListTagsByGroup returns all tags in a specific group
func (db *DB) ListTagsByGroup(groupID int64) ([]models.Tag, error) {
	rows, err := db.Query(`
		SELECT id, name, color, tag_group_id, created_at 
		FROM tags 
		WHERE tag_group_id = ?
		ORDER BY name
	`, groupID)
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

// UpdateTag updates a tag
func (db *DB) UpdateTag(id int64, name, color string, tagGroupID *int64) error {
	_, err := db.Exec("UPDATE tags SET name = ?, color = ?, tag_group_id = ? WHERE id = ?", name, color, tagGroupID, id)
	return err
}

// DeleteTag deletes a tag
func (db *DB) DeleteTag(id int64) error {
	_, err := db.Exec("DELETE FROM tags WHERE id = ?", id)
	return err
}

// GetTagByName retrieves a tag by its name (case-insensitive)
func (db *DB) GetTagByName(name string) (*models.Tag, error) {
	t := &models.Tag{}
	err := db.QueryRow("SELECT id, name, color, tag_group_id, created_at FROM tags WHERE LOWER(name) = LOWER(?)", name).
		Scan(&t.ID, &t.Name, &t.Color, &t.TagGroupID, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}
