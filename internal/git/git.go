package git

import (
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type CommitInfo struct {
	Hash         string
	Message      string
	Author       string
	Branch       string
	FilesChanged []string
	CommittedAt  time.Time
}

// GetRepoRoot returns the root directory of the git repository
func GetRepoRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetCurrentCommit extracts information about the HEAD commit
func GetCurrentCommit() (*CommitInfo, error) {
	info := &CommitInfo{}

	// Get hash
	hash, err := runGitCommand("rev-parse", "HEAD")
	if err != nil {
		return nil, err
	}
	info.Hash = hash

	// Get message (subject only)
	message, err := runGitCommand("log", "-1", "--format=%s")
	if err != nil {
		return nil, err
	}
	info.Message = message

	// Get author
	author, err := runGitCommand("log", "-1", "--format=%an <%ae>")
	if err != nil {
		return nil, err
	}
	info.Author = author

	// Get branch
	branch, err := runGitCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return nil, err
	}
	info.Branch = branch

	// Get files changed
	filesOutput, err := runGitCommand("diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD")
	if err != nil {
		// Initial commit has no parent, use different command
		filesOutput, err = runGitCommand("ls-tree", "--name-only", "-r", "HEAD")
		if err != nil {
			return nil, err
		}
	}
	if filesOutput != "" {
		info.FilesChanged = strings.Split(filesOutput, "\n")
	} else {
		info.FilesChanged = []string{}
	}

	// Get timestamp
	timestamp, err := runGitCommand("log", "-1", "--format=%ci")
	if err != nil {
		return nil, err
	}
	committedAt, err := time.Parse("2006-01-02 15:04:05 -0700", timestamp)
	if err != nil {
		return nil, err
	}
	info.CommittedAt = committedAt

	return info, nil
}

// IsGitRepo checks if the current directory is inside a git repository
func IsGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	err := cmd.Run()
	return err == nil
}

// GetRepoName returns the name of the repository (directory name)
func GetRepoName(repoPath string) string {
	return filepath.Base(repoPath)
}

func runGitCommand(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
