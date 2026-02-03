# PRD: Anchorman

> Git activity tracker and report generator for multi-project workflows

## 1. Overview

### Problem Statement

Developers working across multiple projects for different companies/clients need a way to quickly summarize their work when managers or clients ask for updates. Manually reviewing git history across dozens of repos is time-consuming and error-prone.

### Solution

**Anchorman** is a Go-based TUI application that:
1. Automatically captures git commits via global hooks
2. Organizes repos into a Company → Project hierarchy
3. Uses AI agents (Claude/Codex) to transform raw commits into human-readable task summaries
4. Generates markdown reports grouped by project for easy sharing with stakeholders

### Target Users

- Developers/contractors working for multiple clients
- Freelancers managing several projects
- Anyone needing to report work progress to non-technical stakeholders

## 2. Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Git Hooks                                │
│                    (post-commit, post-merge)                     │
└─────────────────────┬───────────────────────────────────────────┘
                      │ Raw commit data
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                     SQLite Database                              │
│              (~/.anchorman/db/anchorman.sqlite)                  │
│                                                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────────────┐ │
│  │Companies │  │ Projects │  │  Repos   │  │   Raw Commits    │ │
│  └──────────┘  └──────────┘  └──────────┘  └──────────────────┘ │
│                                             ┌──────────────────┐ │
│                                             │      Tasks       │ │
│                                             └──────────────────┘ │
└─────────────────────┬───────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                    TUI (Bubble Tea)                              │
│                                                                  │
│  ┌───────────┐ ┌──────────┐ ┌────────┐ ┌────────┐ ┌──────────┐  │
│  │ Dashboard │ │Companies │ │Projects│ │ Repos  │ │ Reports  │  │
│  └───────────┘ └──────────┘ └────────┘ └────────┘ └──────────┘  │
│                                                                  │
│                    ┌─────────────────┐                           │
│                    │ Process Action  │──► AI Agent (Claude/Codex)│
│                    └─────────────────┘                           │
└─────────────────────────────────────────────────────────────────┘
                      │
                      ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Markdown Reports                                │
│              (configurable output folder)                        │
└─────────────────────────────────────────────────────────────────┘
```

## 3. Data Model

### Companies
| Field | Type | Description |
|-------|------|-------------|
| id | INTEGER | Primary key |
| name | TEXT | Company/client name |
| created_at | DATETIME | Creation timestamp |

### Projects
| Field | Type | Description |
|-------|------|-------------|
| id | INTEGER | Primary key |
| name | TEXT | Project name (e.g., "Cobra", "Lion") |
| company_id | INTEGER | FK to companies (nullable for orphans) |
| created_at | DATETIME | Creation timestamp |

### Repos
| Field | Type | Description |
|-------|------|-------------|
| id | INTEGER | Primary key |
| path | TEXT | Absolute path to repo |
| project_id | INTEGER | FK to projects (nullable for orphans) |
| created_at | DATETIME | First seen timestamp |

### Raw Commits
| Field | Type | Description |
|-------|------|-------------|
| id | INTEGER | Primary key |
| repo_id | INTEGER | FK to repos |
| hash | TEXT | Commit SHA |
| message | TEXT | Commit message |
| author | TEXT | Author name/email |
| branch | TEXT | Branch name |
| files_changed | TEXT | JSON array of file paths |
| committed_at | DATETIME | Commit timestamp |
| processed | BOOLEAN | Whether converted to task |
| created_at | DATETIME | Ingestion timestamp |

### Tasks
| Field | Type | Description |
|-------|------|-------------|
| id | INTEGER | Primary key |
| project_id | INTEGER | FK to projects |
| description | TEXT | Human-readable task description |
| source_commits | TEXT | JSON array of commit IDs that formed this task |
| task_date | DATE | Date the work was performed |
| created_at | DATETIME | Processing timestamp |


## 4. Configuration

**Location:** `~/.anchorman/config.toml`

```toml
# Default AI agent for processing commits into tasks
# Options: "claude", "codex"
default_agent = "codex"

# Output directory for generated markdown reports
reports_output = "~/Documents/reports"

# Base paths to scan for git repos (used by hooks to determine if repo should be tracked)
# Repos outside these paths are ignored
scan_paths = [
    "~/Projects"
]
```

**Directory Structure:**
```
~/.anchorman/
├── config.toml
├── db/
│   └── anchorman.sqlite
└── errors.log
```

## 5. Commands

### `anchorman`
Launches the TUI application.

### `anchorman hooks install`
Sets up global git hooks for commit tracking.

**Behavior:**
1. Check if `core.hooksPath` is already configured
2. If existing hooks present, chain them (run existing + anchorman's)
3. Create `post-commit` hook in `~/.config/git/hooks/`
4. Set `git config --global core.hooksPath ~/.config/git/hooks/`
5. Confirm success

**Generated hook script** (`~/.config/git/hooks/post-commit`):
```bash
#!/bin/bash
# Anchorman post-commit hook
# Chain existing hook if present
if [ -x "$0.legacy" ]; then
    "$0.legacy" "$@"
fi
# Ingest commit (silent, non-blocking)
anchorman ingest 2>/dev/null &
```

### `anchorman hooks uninstall`
Removes anchorman hooks and restores previous configuration.

### `anchorman ingest`
Records the current commit to the database. Called by git hooks.

**Behavior:**
1. Get current repo path from `git rev-parse --show-toplevel`
2. Check if path is under configured `scan_paths` - exit silently if not
3. Extract commit info via git:
   - Hash: `git rev-parse HEAD`
   - Message: `git log -1 --format=%s`
   - Author: `git log -1 --format=%an <%ae>`
   - Branch: `git rev-parse --abbrev-ref HEAD`
   - Files changed: `git diff-tree --no-commit-id --name-only -r HEAD`
   - Timestamp: `git log -1 --format=%ci`
4. Insert into `raw_commits` table
5. Create repo record if not exists (as orphan)
6. Exit silently (no stdout), errors logged to `~/.anchorman/errors.log`

**Flags:**
- `--verbose` - Print what was recorded (for debugging)

## 6. TUI Screens

### 6.1 Dashboard
**Purpose:** Overview and quick actions

**Content:**
- Unprocessed commits count (with indicator if > 0)
- Recent activity summary by company
- Last processed timestamp
- Quick action: "Process commits" (with date range picker)

**Actions:**
- `p` - Process unprocessed commits
- `r` - Go to Reports
- `c` - Go to Companies

### 6.2 Companies
**Purpose:** Manage companies/clients

**Content:**
- List of companies with project count
- Total repos and tasks per company

**Actions:**
- `a` - Add new company
- `e` - Edit company name
- `d` - Delete company (with confirmation, reassigns projects to orphan)
- `Enter` - View projects under company

### 6.3 Projects
**Purpose:** Manage projects within a company

**Content:**
- List of projects (filtered by company or all)
- Repos count per project
- Company assignment

**Actions:**
- `a` - Add new project
- `e` - Edit project
- `m` - Move project to different company
- `d` - Delete project
- `Enter` - View repos under project

### 6.4 Repos
**Purpose:** View and organize tracked repositories

**Content:**
- List of all tracked repos
- Orphan repos highlighted (no project assigned)
- Path, project name, company name
- Commit count (processed/unprocessed)

**Actions:**
- `a` - Assign orphan repo to project
- `f` - Filter (orphans only, by company, by project)
- `Enter` - View repo details and recent commits

### 6.5 Reports
**Purpose:** Generate markdown reports

**Content:**
- Company selector
- Date range picker (presets: today, this week, last week, this month, custom)
- Preview of what will be included (project count, task count)

**Actions:**
- `Enter` - Generate report
- Generated report saved to configured output folder
- Filename format: `{company}_{date-range}.md`


## 7. AI Processing

### Input to Agent
When user triggers "Process commits", the system:

1. Queries unprocessed commits (optionally filtered by date range)
2. Groups commits by repo/project
3. Sends to configured agent with prompt:

```
You are analyzing git commits to create human-readable task summaries for manager reports.

Project: {project_name}
Repository: {repo_name}

Commits:
- {hash}: {message} (branch: {branch}, files: {files_changed})
- ...

Create a list of conceptual tasks that summarize the work done.
- Group related commits into single tasks
- Use plain, non-technical language suitable for managers
- Focus on WHAT was accomplished, not HOW
- Each task should be a single line starting with a verb (Implemented, Fixed, Added, Updated, etc.)

Output format (one task per line):
- Task description 1
- Task description 2
```

### Output Handling
- Parse agent response into individual tasks
- Store in `tasks` table with reference to source commits
- Mark source commits as `processed = true`
- Show spinner during processing, errors logged to `~/.anchorman/errors.log`

### Context Window Management
- Allow user to select date range for processing
- If too many commits, suggest narrowing the date range
- Process in batches if needed (by project)

## 8. Report Format

**Filename:** `{company-slug}_{start-date}_to_{end-date}.md`

**Example:** `main-company_2025-01-27_to_2025-02-02.md`

```markdown
# Main Company - Weekly Report

**Period:** January 27 - February 2, 2025
**Generated:** 2025-02-02

---

## Cobra

- Implemented user authentication system with login and registration
- Fixed redirect loop issue after login
- Added password strength validation
- Updated user profile page with new fields

## Lion

- Integrated payment gateway (Stripe)
- Fixed currency conversion bug for international transactions
- Added invoice PDF generation

---

*Generated by Anchorman*
```

## 9. Technical Stack

| Component | Technology |
|-----------|------------|
| Language | Go |
| TUI Framework | Bubble Tea |
| Database | SQLite |
| DB Migrations | golang-migrate/migrate |
| Config | TOML |
| AI Agents | Claude CLI, Codex CLI |
| Git Integration | Git CLI, Global Hooks |
| GitHub Integration | gh CLI (optional, for future features) |

### Database Migrations

Schema changes are managed with [golang-migrate](https://github.com/golang-migrate/migrate):

- Migrations stored in `internal/db/migrations/` as SQL files
- Naming convention: `{version}_{description}.up.sql` and `{version}_{description}.down.sql`
- Migrations embedded in binary using Go's `embed` package
- Auto-run on application start (migrate to latest)

Example:
```
internal/db/migrations/
├── 000001_initial_schema.up.sql
├── 000001_initial_schema.down.sql
├── 000002_add_tasks_table.up.sql
└── 000002_add_tasks_table.down.sql
```

## 10. User Stories

### US-1: Initial Setup
**As a** developer
**I want to** install anchorman and set up hooks
**So that** my commits are automatically tracked

**Acceptance Criteria:**
- [ ] `anchorman hooks install` configures global git hooks
- [ ] Existing hooks are preserved (chained)
- [ ] Config file created with defaults if not exists
- [ ] Database initialized on first run

### US-2: Organize Repos
**As a** developer
**I want to** assign my repos to companies and projects
**So that** my work is properly categorized

**Acceptance Criteria:**
- [ ] TUI shows orphan repos that need assignment
- [ ] Can create companies and projects from TUI
- [ ] Can assign repos to projects
- [ ] Can move projects between companies

### US-3: Process Commits
**As a** developer
**I want to** convert my raw commits into readable tasks
**So that** I can generate meaningful reports

**Acceptance Criteria:**
- [ ] Dashboard shows unprocessed commit count
- [ ] Can trigger processing with date range filter
- [ ] AI agent groups related commits into tasks
- [ ] Processing errors are logged and displayed
- [ ] Processed commits are marked (not deleted)

### US-4: Generate Reports
**As a** developer
**I want to** generate a markdown report for a company
**So that** I can share my progress with stakeholders

**Acceptance Criteria:**
- [ ] Can select company and date range
- [ ] Report groups tasks by project
- [ ] Report saved to configured output folder
- [ ] Report uses clean, professional formatting

### US-5: Switch AI Agent
**As a** developer
**I want to** switch between Claude and Codex
**So that** I can use whichever has available credits

**Acceptance Criteria:**
- [ ] `default_agent` in config.toml controls which agent is used
- [ ] Editing config.toml and restarting TUI picks up the change
- [ ] Processing uses the configured agent

## 11. Out of Scope (v1)

- Automatic/scheduled processing (cron/systemd)
- Pull from remote branches (local only)
- GitHub issue/PR linking
- Team collaboration features
- Web interface
- Real-time streaming of AI output
- Multiple author filtering
- Export formats other than Markdown

## 12. Future Considerations (v2+)

- GitHub integration: link commits to issues/PRs
- Slack/Discord integration for report sharing
- Time tracking estimates based on commit patterns
- Multiple report templates
- Report editing before export
- Cloud sync of database

## 13. Success Metrics

- Time to generate weekly report: < 2 minutes
- Zero missed commits (hook reliability)
- AI task accuracy: tasks are understandable by non-technical readers

## 14. Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| AI context window exceeded | Processing fails | Date range filtering, batch processing |
| Global hooks conflict with existing setup | User's workflow broken | Chain existing hooks, provide uninstall |
| AI generates poor summaries | Useless reports | Allow re-processing, prompt tuning |
| Large repos slow down hooks | Git operations delayed | Async write to SQLite, minimal hook logic |
