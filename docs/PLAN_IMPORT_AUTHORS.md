# Plan: Import Legacy Commits & Author Tracking

## Overview

Two related features:
1. **Import Command** - `anchorman import` to import historical commits from current directory
2. **Author Tracking** - Toggle authors on/off in reports (like time toggle)

---

## Feature 1: Import Legacy Commits

### CLI Command (Simple API)

```bash
# Import last N commits (argument required)
anchorman import 10

# Import commits since date
anchorman import 2025-01-15

# Import from specific branch (default: all branches)
anchorman import 10 --branch main

# Force re-import (re-ingest existing, delete related tasks)
anchorman import 10 -f

# No argument = error with helpful message
anchorman import
# Error: missing required argument
# Expected: a number (commit count) or date (YYYY-MM-DD)
```

**Argument parsing:**
- If arg matches `YYYY-MM-DD` format → treat as date (since)
- If arg is numeric → treat as commit count
- **No argument → error** (argument is required)

**Flags:**
- `--branch string` - Specific branch (default: all branches)
- `-f, --force` - Re-ingest existing commits, mark as unprocessed, delete related tasks

**Always verbose** (no verbose flag needed).

### Behavior

1. Run from within a git repository
2. Auto-register repo as orphan if not already registered
3. Import commits as unprocessed
4. Skip duplicates (unless `-f` flag)
5. Warn if repo path not in config's `scan_paths`

### Force Mode (`-f`)

When force mode is enabled:
1. If commit already exists:
   - Mark it as unprocessed (`processed = 0`)
   - Find and delete any tasks that reference this commit in `source_commits`
2. Re-ingest the commit data (in case message/author changed)

---

### New Files

#### `internal/git/history.go`

```go
package git

import (
	"fmt"
	"strings"
	"time"
)

type HistoryOptions struct {
	Count  int       // 0 means all
	Since  time.Time // zero = no filter
	Branch string    // empty = all branches
}

// GetCommitHistory retrieves commit history from the repository
func GetCommitHistory(opts HistoryOptions) ([]CommitInfo, error) {
	args := []string{"log", "--format=%H|%s|%an <%ae>|%ci"}

	if opts.Count > 0 {
		args = append(args, fmt.Sprintf("-n%d", opts.Count))
	}

	if !opts.Since.IsZero() {
		args = append(args, fmt.Sprintf("--since=%s", opts.Since.Format("2006-01-02")))
	}

	if opts.Branch != "" {
		args = append(args, opts.Branch)
	} else {
		args = append(args, "--all")
	}

	output, err := runGitCommand(args...)
	if err != nil {
		return nil, fmt.Errorf("git log failed: %w", err)
	}

	var commits []CommitInfo
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) != 4 {
			continue
		}

		hash := parts[0]
		message := parts[1]
		author := parts[2]
		timestamp := parts[3]

		committedAt, err := time.Parse("2006-01-02 15:04:05 -0700", timestamp)
		if err != nil {
			continue
		}

		// Get files changed for this commit
		filesOutput, _ := runGitCommand("diff-tree", "--no-commit-id", "--name-only", "-r", hash)
		var filesChanged []string
		if filesOutput != "" {
			for _, f := range strings.Split(filesOutput, "\n") {
				if f != "" {
					filesChanged = append(filesChanged, f)
				}
			}
		}

		// Get branch containing this commit
		branch := getBranchForCommit(hash)

		commits = append(commits, CommitInfo{
			Hash:         hash,
			Message:      message,
			Author:       author,
			Branch:       branch,
			FilesChanged: filesChanged,
			CommittedAt:  committedAt,
		})
	}

	return commits, nil
}

func getBranchForCommit(hash string) string {
	output, err := runGitCommand("branch", "--contains", hash, "--format=%(refname:short)")
	if err != nil || output == "" {
		return "unknown"
	}
	branches := strings.Split(output, "\n")
	if len(branches) > 0 && branches[0] != "" {
		return branches[0]
	}
	return "unknown"
}
```

#### `internal/git/import.go`

```go
package git

import (
	"fmt"
	"time"

	"github.com/emilianohg/anchorman/internal/config"
	"github.com/emilianohg/anchorman/internal/db"
	"github.com/emilianohg/anchorman/internal/repository"
)

type ImportOptions struct {
	Count  int
	Since  time.Time
	Branch string
	Force  bool
}

type ImportResult struct {
	RepoPath      string
	Imported      int
	Skipped       int
	Updated       int // Force mode: existing commits updated
	TasksDeleted  int // Force mode: tasks deleted
	TotalFound    int
	IsOrphan      bool
	NotInScanPath bool
}

func Import(opts ImportOptions) (*ImportResult, error) {
	result := &ImportResult{}

	// 1. Check if we're in a git repo
	if !IsGitRepo() {
		return nil, fmt.Errorf("not a git repository")
	}

	// 2. Get repo root
	repoPath, err := GetRepoRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to get repo root: %w", err)
	}
	result.RepoPath = repoPath

	// 3. Load config and check if path is tracked
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.IsPathTracked(repoPath) {
		result.NotInScanPath = true
	}

	// 4. Open database
	database, err := db.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// 5. Get or create repo (as orphan if new)
	repoRepo := repository.NewRepoRepo(database)
	repo, err := repoRepo.GetOrCreate(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create repo: %w", err)
	}
	result.IsOrphan = repo.ProjectID == nil

	// 6. Get commit history
	historyOpts := HistoryOptions{
		Count:  opts.Count,
		Since:  opts.Since,
		Branch: opts.Branch,
	}

	commits, err := GetCommitHistory(historyOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit history: %w", err)
	}
	result.TotalFound = len(commits)

	// 7. Insert commits
	commitRepo := repository.NewCommitRepo(database)
	taskRepo := repository.NewTaskRepo(database)

	for _, commitInfo := range commits {
		existing, err := commitRepo.GetByRepoAndHash(repo.ID, commitInfo.Hash)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing commit: %w", err)
		}

		if existing != nil {
			if opts.Force {
				// Force mode: update and mark unprocessed
				if existing.Processed {
					// Delete related tasks
					deleted, err := taskRepo.DeleteTasksWithCommit(existing.ID)
					if err != nil {
						return nil, fmt.Errorf("failed to delete tasks: %w", err)
					}
					result.TasksDeleted += deleted
				}

				err = commitRepo.UpdateAndMarkUnprocessed(
					existing.ID,
					commitInfo.Message,
					commitInfo.Author,
					commitInfo.Branch,
					commitInfo.FilesChanged,
					commitInfo.CommittedAt,
				)
				if err != nil {
					return nil, fmt.Errorf("failed to update commit: %w", err)
				}
				result.Updated++
				fmt.Printf("  Updated: %s %.50s\n", commitInfo.Hash[:8], commitInfo.Message)
			} else {
				result.Skipped++
			}
			continue
		}

		// Create new commit record
		_, err = commitRepo.Create(
			repo.ID,
			commitInfo.Hash,
			commitInfo.Message,
			commitInfo.Author,
			commitInfo.Branch,
			commitInfo.FilesChanged,
			commitInfo.CommittedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create commit: %w", err)
		}

		result.Imported++
		fmt.Printf("  Imported: %s %.50s\n", commitInfo.Hash[:8], commitInfo.Message)
	}

	return result, nil
}
```

---

### Modify: `cmd/anchorman/main.go`

Add import command:

```go
var importCmd = &cobra.Command{
	Use:   "import <count|date>",
	Short: "Import commits from the current git repository",
	Long: `Import historical commits from the current git repository.

Argument (required):
  count  Number of recent commits to import (e.g., 10, 50, 100)
  date   Import commits since date in YYYY-MM-DD format

Examples:
  anchorman import 10          # Last 10 commits
  anchorman import 2025-01-15  # Commits since date
  anchorman import 10 -f       # Force re-import`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("missing required argument\n\nUsage: anchorman import <count|date>\n\nExamples:\n  anchorman import 10          # Import last 10 commits\n  anchorman import 2025-01-15  # Import commits since date")
		}
		if len(args) > 1 {
			return fmt.Errorf("too many arguments")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		opts := git.ImportOptions{}

		// Parse argument as number or date
		if count, err := strconv.Atoi(args[0]); err == nil {
			opts.Count = count
		} else if date, err := time.Parse("2006-01-02", args[0]); err == nil {
			opts.Since = date
		} else {
			fmt.Fprintf(os.Stderr, "Error: invalid argument '%s'\n\n", args[0])
			fmt.Fprintf(os.Stderr, "Expected:\n")
			fmt.Fprintf(os.Stderr, "  - A number for commit count (e.g., anchorman import 10)\n")
			fmt.Fprintf(os.Stderr, "  - A date in YYYY-MM-DD format (e.g., anchorman import 2025-01-15)\n")
			os.Exit(1)
		}

		opts.Branch, _ = cmd.Flags().GetString("branch")
		opts.Force, _ = cmd.Flags().GetBool("force")

		fmt.Println("Importing commits...")

		result, err := git.Import(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Warnings
		if result.NotInScanPath {
			fmt.Println("\nWarning: Repository is not in configured scan_paths")
			fmt.Println("         Future commits won't be auto-tracked. Add path to config.")
		}

		// Summary
		fmt.Printf("\nRepository: %s\n", result.RepoPath)
		if result.IsOrphan {
			fmt.Println("Status: Registered as orphan (assign to project via TUI)")
		}
		fmt.Printf("\nSummary:\n")
		fmt.Printf("  Found:    %d commits\n", result.TotalFound)
		fmt.Printf("  Imported: %d\n", result.Imported)
		fmt.Printf("  Skipped:  %d (duplicates)\n", result.Skipped)
		if opts.Force {
			fmt.Printf("  Updated:  %d\n", result.Updated)
			fmt.Printf("  Tasks deleted: %d\n", result.TasksDeleted)
		}

		if result.Imported > 0 || result.Updated > 0 {
			fmt.Println("\nProcess these commits via the TUI to create tasks.")
		}
	},
}

func init() {
	// ... existing init code ...

	importCmd.Flags().String("branch", "", "Specific branch (default: all branches)")
	importCmd.Flags().BoolP("force", "f", false, "Force re-import existing commits")
	rootCmd.AddCommand(importCmd)
}
```

---

### Modify: `internal/repository/task.go`

Add method to delete tasks containing a commit:

```go
// DeleteTasksWithCommit deletes tasks where source_commits contains the commit ID
// Returns number of tasks deleted
func (r *TaskRepo) DeleteTasksWithCommit(commitID int64) (int, error) {
	// SQLite JSON: source_commits is stored as JSON array like "[1,2,3]"
	// We need to find tasks where this array contains commitID

	rows, err := r.db.Query(`
		SELECT id, source_commits FROM tasks
	`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var toDelete []int64
	for rows.Next() {
		var id int64
		var commitsJSON string
		if err := rows.Scan(&id, &commitsJSON); err != nil {
			return 0, err
		}

		var commits []int64
		if err := json.Unmarshal([]byte(commitsJSON), &commits); err != nil {
			continue
		}

		for _, cid := range commits {
			if cid == commitID {
				toDelete = append(toDelete, id)
				break
			}
		}
	}

	if err := rows.Err(); err != nil {
		return 0, err
	}

	// Delete the tasks
	for _, id := range toDelete {
		if _, err := r.db.Exec("DELETE FROM tasks WHERE id = ?", id); err != nil {
			return 0, err
		}
	}

	return len(toDelete), nil
}
```

---

### Modify: `internal/repository/commit.go`

Add update method:

```go
// UpdateAndMarkUnprocessed updates commit data and marks it as unprocessed
func (r *CommitRepo) UpdateAndMarkUnprocessed(id int64, message, author, branch string, filesChanged []string, committedAt time.Time) error {
	filesJSON, err := json.Marshal(filesChanged)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(`
		UPDATE raw_commits
		SET message = ?, author = ?, branch = ?, files_changed = ?, committed_at = ?, processed = 0
		WHERE id = ?
	`, message, author, branch, string(filesJSON), committedAt, id)

	return err
}
```

---

## Feature 2: Author Tracking in Reports

### Modify: `internal/models/models.go`

Add Authors field to Task:

```go
type Task struct {
	ID             int64
	ProjectID      int64
	Description    string
	SourceCommits  []int64
	TaskDate       time.Time
	EstimatedHours float64
	CreatedAt      time.Time

	// Joined fields
	ProjectName string
	Authors     []string // Derived from source_commits -> raw_commits.author
}
```

---

### Modify: `internal/repository/task.go`

Add author methods:

```go
// GetAuthorsForCommits returns unique authors for the given commit IDs
// Authors are shortened to "FirstName L." format
func (r *TaskRepo) GetAuthorsForCommits(commitIDs []int64) ([]string, error) {
	if len(commitIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(commitIDs))
	args := make([]interface{}, len(commitIDs))
	for i, id := range commitIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `
		SELECT DISTINCT author
		FROM raw_commits
		WHERE id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY author
	`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authors []string
	for rows.Next() {
		var author string
		if err := rows.Scan(&author); err != nil {
			return nil, err
		}
		authors = append(authors, shortenAuthorName(author))
	}

	return authors, rows.Err()
}

// shortenAuthorName converts "John Doe <email>" to "John D."
func shortenAuthorName(fullAuthor string) string {
	name := fullAuthor
	if idx := strings.Index(fullAuthor, "<"); idx > 0 {
		name = strings.TrimSpace(fullAuthor[:idx])
	}

	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "Unknown"
	}
	if len(parts) == 1 {
		return parts[0]
	}

	return fmt.Sprintf("%s %s.", parts[0], string(parts[len(parts)-1][0]))
}

// GetByCompanyAndDateRangeWithAuthors returns tasks with authors populated
func (r *TaskRepo) GetByCompanyAndDateRangeWithAuthors(companyID int64, from, to time.Time) ([]models.Task, error) {
	tasks, err := r.GetByCompanyAndDateRange(companyID, from, to)
	if err != nil {
		return nil, err
	}

	for i := range tasks {
		authors, err := r.GetAuthorsForCommits(tasks[i].SourceCommits)
		if err != nil {
			return nil, err
		}
		tasks[i].Authors = authors
	}

	return tasks, nil
}
```

---

### Modify: `internal/tui/screens/reports.go`

Add showAuthors toggle:

```go
type Reports struct {
	// ... existing fields ...
	showTime    bool
	showAuthors bool // NEW
}
```

Add key bindings in `handleRangeKey()` and `handlePreviewKey()`:

```go
case "a":
	r.showAuthors = !r.showAuthors
```

Update `loadPreview()` to use `GetByCompanyAndDateRangeWithAuthors`.

Update `generateReport()` to include authors:
- Per-task: `- Fixed bug (John D.) (1.5h)`
- Project summary: `**Contributors:** John D., Jane S.`

Update help text:
```go
b.WriteString(HelpStyle.Render("[g/enter] Generate  [t] Toggle time  [a] Toggle authors  [esc] Back"))
```

---

## README Documentation

Add to `README.md` after "Quick Start" section:

```markdown
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

# Import from specific branch
anchorman import 50 --branch main

# Force re-import (overwrites existing, deletes related tasks)
anchorman import 10 -f
```

The argument is required - specify either a commit count or a date.
The repository will be auto-registered if not already tracked.
Imported commits are unprocessed - use the TUI to process them into tasks.

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
```

---

## Files Summary

| File | Action | Description |
|------|--------|-------------|
| `internal/git/history.go` | Create | Git log history retrieval |
| `internal/git/import.go` | Create | Import logic with force mode |
| `cmd/anchorman/main.go` | Modify | Add import command |
| `internal/repository/commit.go` | Modify | Add UpdateAndMarkUnprocessed |
| `internal/repository/task.go` | Modify | Add DeleteTasksWithCommit, author methods |
| `internal/models/models.go` | Modify | Add Authors field to Task |
| `internal/tui/screens/reports.go` | Modify | Add showAuthors toggle |
| `README.md` | Modify | Document commands and report options |

---

## Verification

### Import Command

```bash
cd /path/to/git/repo

# Test missing argument error
anchorman import
# Expected: Error message explaining argument is required

# Test basic import
anchorman import 5
# Expected: Shows 5 commits imported

# Test duplicate handling
anchorman import 5
# Expected: Shows 5 skipped (duplicates)

# Test force mode
anchorman import 5 -f
# Expected: Shows 5 updated, tasks deleted count

# Test date format
anchorman import 2024-01-01
# Expected: Imports commits since that date

# Test invalid argument
anchorman import abc
# Expected: Error explaining valid formats
```

### Author Toggle

1. Import and process some commits
2. Go to Reports → select company → select date range
3. Preview shows tasks without authors (default)
4. Press `a` → authors appear: `- Task description (John D.)`
5. Project shows: `Contributors: John D., Jane S.`
6. Press `a` again → authors hidden
7. Generate report → markdown includes authors when toggled on
