# STM (Simple Task Manager)

This is the beginning of my interpretation of a simple task manager application. I like to keep things simple and minimal and this will likely go through many changes.

A keyboard-driven terminal UI (TUI) for managing projects and tasks. Built with Go and the [Charm](https://charm.sh) stack.

![Go Version](https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat&logo=go)

## Features

- **Projects** - Organize tasks into separate projects
- **Tasks** - Create, edit, and delete tasks with title, description, notes, and priority (0-10)
- **Tags** - Color-coded tags with support for mutually exclusive tag groups
- **Comments** - Add comments to tasks for additional context
- **Search & Filter** - Search tasks by title/description, filter by tag
- **Completed Tasks** - Toggle between active and completed task views
- **Unsaved Changes Protection** - Confirmation prompt when discarding edits
- **Keyboard-Driven** - Full vim-style navigation (hjkl)
- **Neovim Plugin** - [stm.nvim](https://github.com/tgienger/stm.nvim) for seamless integration with Neovim

## TODO
- Thinking...

## Installation

```bash
# Clone the repository
git clone https://github.com/tgienger/stm.git
cd stm

# Build
go build -o stm ./cmd/stm

# Run
./stm
```

Or install directly:

```bash
go install github.com/tgienger/stm/cmd/stm@latest
```

## Usage

```bash
stm              # Start the application
stm --version    # Show version
```

Data is stored in `~/.local/share/stm/stm.db` (SQLite).

## Keyboard Shortcuts

### Navigation

| Key | Action |
|-----|--------|
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `Enter` | Select / Confirm |
| `Esc` | Back / Cancel |
| `q` | Quit |
| `?` | Show help |

### Actions

| Key | Action |
|-----|--------|
| `n` | New project/task |
| `e` | Edit selected |
| `d` | Delete selected |
| `/` | Search |
| `f` | Filter by tag |
| `t` | Assign tags |
| `c` | Toggle completed tasks |

### Edit Form

| Key | Action |
|-----|--------|
| `Tab` | Next field |
| `Shift+Tab` | Previous field |
| `Ctrl+S` | Save |
| `Esc` | Cancel (prompts if unsaved) |

### Confirmation Dialogs

| Key | Action |
|-----|--------|
| `Y` | Yes / Discard |
| `S` | Save |
| `N` | No / Cancel |

## Architecture

```
cmd/stm/           # Application entry point
internal/
  db/              # SQLite database layer
  models/          # Data models (Project, Task, Tag, Comment)
  ui/
    keys/          # Keyboard bindings
    styles/        # Theme and styling (Tokyo Night)
    views/         # UI views (projects, tasks)
```

## Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Bubbles](https://github.com/charmbracelet/bubbles) - UI components
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Styling
- [go-sqlite3](https://github.com/mattn/go-sqlite3) - SQLite driver

## License

MIT
