# Anchorman

Git activity tracker and report generator for developers working across multiple projects and clients.

Anchorman automatically tracks your git commits via global hooks, organizes them by company and project, uses AI to summarize work into manager-friendly tasks, and generates markdown reports.

## Features

- **Automatic commit tracking** via global git hooks
- **Company/Project organization** for multi-client workflows
- **AI-powered summarization** using Claude or Codex CLI
- **Markdown report generation** grouped by project
- **TUI interface** built with Bubble Tea

## Installation

### Prerequisites

- Go 1.21 or later
- Git
- One of the following AI CLI tools (for processing commits):
  - [Claude CLI](https://github.com/anthropics/claude-cli)
  - [Codex CLI](https://github.com/openai/codex)

### Install from source

```bash
git clone https://github.com/emilianohg/anchorman.git
cd anchorman
make install
```

This installs `anchorman` to `~/.local/bin`. Make sure this directory is in your `PATH`:

```bash
export PATH="$HOME/.local/bin:$PATH"
```

### Updating

To update to a new version:

```bash
git pull
make install
```

The TUI dashboard will detect any pending database migrations and prompt you to run them.

### Uninstall

```bash
make uninstall
```

## Quick Start

1. **Install the hooks** to start tracking commits:

```bash
anchorman hooks install
```

2. **Launch the TUI**:

```bash
anchorman
```

3. **Create a company** (press `c` in dashboard)

4. **Create projects** and assign repos to them

5. **Make some commits** in your tracked repos

6. **Process commits** (press `p` in dashboard) - this uses AI to create task summaries

7. **Generate reports** (press `r` in dashboard)

## Commands

### Launch TUI

```bash
anchorman
```

### Import Historical Commits

Import existing commits from a git repository:

```bash
# Navigate to your git repository
cd ~/Projects/my-project

# Import last 10 commits
anchorman import 10

# Import commits since a date
anchorman import 2025-01-15

# Import all commits
anchorman import

# Import from specific branch
anchorman import 50 --branch main

# Force re-import (overwrites existing, deletes related tasks)
anchorman import 10 -f
```

The repository will be auto-registered if not already tracked. Imported commits are unprocessed - use the TUI to process them into tasks.

### Manage Git Hooks

```bash
# Install global hooks (auto-track future commits)
anchorman hooks install

# Remove hooks
anchorman hooks uninstall
```

## Report Options

When generating reports, use these toggles in the preview screen:

| Key | Toggle |
|-----|--------|
| `t` | Show/hide time estimates |
| `a` | Show/hide authors |

## Configuration

Configuration is stored in `~/.anchorman/config.toml`:

```toml
# AI agent for processing commits ("claude" or "codex")
default_agent = "codex"

# Output directory for generated reports
reports_output = "~/Documents/reports"

# Directories to track (repos outside these paths are ignored)
scan_paths = [
    "~/Projects"
]
```

## Data Storage

- **Database**: `~/.anchorman/db/anchorman.sqlite`
- **Error log**: `~/.anchorman/errors.log`
- **Reports**: Configured via `reports_output` in config

## TUI Navigation

| Key | Action |
|-----|--------|
| `p` | Process unprocessed commits |
| `c` | Manage companies |
| `o` | Manage repositories |
| `r` | Generate reports |
| `q` | Quit / Go back |
| `j/k` or arrows | Navigate lists |
| `Enter` | Select / Confirm |
| `a` | Add new item |
| `e` | Edit selected item |
| `d` | Delete selected item |

## Development

### Building

```bash
make build
```

### Running locally

```bash
make run
```

### Formatting and linting

```bash
make fmt
make lint  # requires golangci-lint
```

### Testing

```bash
make test
```

### Database

Reset the database (useful during development):

```bash
make db-reset
```

### Adding migrations

1. Create new migration files in `internal/db/migrations/`:
   - `XXXXXX_description.up.sql`
   - `XXXXXX_description.down.sql`

2. The version number should be sequential (e.g., `000002`, `000003`)

3. After installing a new version, the TUI dashboard will detect pending migrations and prompt to run them

## Architecture

```
cmd/anchorman/          # CLI entry point
internal/
├── agent/              # AI agent integration (Claude/Codex)
├── config/             # Configuration loading
├── db/                 # Database and migrations
├── git/                # Git operations and hooks
├── models/             # Data structures
├── repository/         # Database access layer
└── tui/                # Bubble Tea TUI
    └── screens/        # Individual TUI screens
```

## How It Works

1. **Git hooks** (installed globally) call `anchorman ingest` on every commit
2. **Ingest** checks if the repo is in a tracked path, then stores commit data
3. **Processing** groups commits by project and sends them to the AI agent
4. **AI agent** returns human-readable task descriptions
5. **Reports** are generated as markdown, grouped by project

## License

MIT
