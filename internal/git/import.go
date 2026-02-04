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

	// Check if we're in a git repo
	if !IsGitRepo() {
		return nil, fmt.Errorf("not a git repository")
	}

	// Get repo root
	repoPath, err := GetRepoRoot()
	if err != nil {
		return nil, fmt.Errorf("failed to get repo root: %w", err)
	}
	result.RepoPath = repoPath

	// Load config and check if path is tracked
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if !cfg.IsPathTracked(repoPath) {
		result.NotInScanPath = true
	}

	// Get commit history
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

	if len(commits) == 0 {
		return result, nil
	}

	// Open database
	database, err := db.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Get or create repo
	repoRepo := repository.NewRepoRepo(database)
	repo, err := repoRepo.GetOrCreate(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get/create repo: %w", err)
	}

	// Check if repo is orphan
	result.IsOrphan = repo.ProjectID == nil

	commitRepo := repository.NewCommitRepo(database)
	taskRepo := repository.NewTaskRepo(database)

	for _, commit := range commits {
		// Check if commit already exists
		existing, err := commitRepo.GetByRepoAndHash(repo.ID, commit.Hash)
		if err != nil {
			return nil, fmt.Errorf("failed to check existing commit: %w", err)
		}

		if existing != nil {
			if opts.Force {
				// Delete tasks referencing this commit
				deleted, err := taskRepo.DeleteTasksWithCommit(existing.ID)
				if err != nil {
					return nil, fmt.Errorf("failed to delete tasks for commit %s: %w", commit.Hash[:8], err)
				}
				result.TasksDeleted += deleted

				// Update commit and mark as unprocessed
				err = commitRepo.UpdateAndMarkUnprocessed(
					existing.ID,
					commit.Message,
					commit.Author,
					commit.Branch,
					commit.FilesChanged,
					commit.CommittedAt,
				)
				if err != nil {
					return nil, fmt.Errorf("failed to update commit %s: %w", commit.Hash[:8], err)
				}
				result.Updated++
			} else {
				result.Skipped++
			}
			continue
		}

		// Create commit record
		_, err = commitRepo.Create(
			repo.ID,
			commit.Hash,
			commit.Message,
			commit.Author,
			commit.Branch,
			commit.FilesChanged,
			commit.CommittedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create commit record: %w", err)
		}
		result.Imported++
	}

	return result, nil
}
