package git

import (
	"fmt"

	"github.com/emilianohg/anchorman/internal/config"
	"github.com/emilianohg/anchorman/internal/db"
	"github.com/emilianohg/anchorman/internal/repository"
)

type IngestResult struct {
	RepoPath  string
	CommitHash string
	Message   string
	Skipped   bool
	SkipReason string
}

func Ingest(verbose bool) (*IngestResult, error) {
	result := &IngestResult{}

	// Check if we're in a git repo
	if !IsGitRepo() {
		result.Skipped = true
		result.SkipReason = "not a git repository"
		return result, nil
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
		result.Skipped = true
		result.SkipReason = "repo path not in configured scan_paths"
		return result, nil
	}

	// Get commit info
	commitInfo, err := GetCurrentCommit()
	if err != nil {
		return nil, fmt.Errorf("failed to get commit info: %w", err)
	}
	result.CommitHash = commitInfo.Hash
	result.Message = commitInfo.Message

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

	// Check if commit already exists
	commitRepo := repository.NewCommitRepo(database)
	existing, err := commitRepo.GetByRepoAndHash(repo.ID, commitInfo.Hash)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing commit: %w", err)
	}

	if existing != nil {
		result.Skipped = true
		result.SkipReason = "commit already recorded"
		return result, nil
	}

	// Create commit record
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
		return nil, fmt.Errorf("failed to create commit record: %w", err)
	}

	return result, nil
}
