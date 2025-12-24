-- Schema for STM (Simple Task Manager)

-- Application settings (stores last opened project, etc.)
CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Tag groups (for radio-button behavior)
-- Tags within the same group are mutually exclusive on a task
CREATE TABLE IF NOT EXISTS tag_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Tags table
CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    color TEXT DEFAULT '#7aa2f7',
    tag_group_id INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tag_group_id) REFERENCES tag_groups(id) ON DELETE SET NULL,
    UNIQUE(name)
);

-- Tasks table
CREATE TABLE IF NOT EXISTS tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL,
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    notes TEXT DEFAULT '',
    priority INTEGER DEFAULT 0 CHECK (priority >= 0 AND priority <= 10),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

-- Task-Tag relationship (many-to-many)
CREATE TABLE IF NOT EXISTS task_tags (
    task_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    PRIMARY KEY (task_id, tag_id),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

-- Comments on tasks
CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    content TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_tasks_project_id ON tasks(project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_priority_created ON tasks(priority DESC, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_task_tags_task_id ON task_tags(task_id);
CREATE INDEX IF NOT EXISTS idx_task_tags_tag_id ON task_tags(tag_id);
CREATE INDEX IF NOT EXISTS idx_tags_group_id ON tags(tag_group_id);
CREATE INDEX IF NOT EXISTS idx_comments_task_id ON comments(task_id);

-- Insert default tag group for workflow status
INSERT OR IGNORE INTO tag_groups (name) VALUES ('Status');

-- Insert default tags
INSERT OR IGNORE INTO tags (name, color, tag_group_id) 
SELECT 'design', '#bb9af7', id FROM tag_groups WHERE name = 'Status';
INSERT OR IGNORE INTO tags (name, color, tag_group_id) 
SELECT 'todo', '#7aa2f7', id FROM tag_groups WHERE name = 'Status';
INSERT OR IGNORE INTO tags (name, color, tag_group_id) 
SELECT 'active', '#e0af68', id FROM tag_groups WHERE name = 'Status';
INSERT OR IGNORE INTO tags (name, color, tag_group_id) 
SELECT 'complete', '#9ece6a', id FROM tag_groups WHERE name = 'Status';
