package git

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type HistoryOptions struct {
	Count  int       // 0 means all
	Since  time.Time // zero = no filter
	Branch string    // empty = all branches
}

// GetCommitHistory retrieves commit history based on options
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
		// All branches
		args = append(args, "--all")
	}

	cmd := exec.Command("git", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git log: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return []CommitInfo{}, nil
	}

	var commits []CommitInfo
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
		filesChanged, err := getFilesChangedForCommit(hash)
		if err != nil {
			filesChanged = []string{}
		}

		// Get branch for this commit
		branch, err := getBranchForCommit(hash)
		if err != nil {
			branch = "unknown"
		}

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

func getFilesChangedForCommit(hash string) ([]string, error) {
	cmd := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", hash)
	output, err := cmd.Output()
	if err != nil {
		// Initial commit has no parent
		cmd = exec.Command("git", "ls-tree", "--name-only", "-r", hash)
		output, err = cmd.Output()
		if err != nil {
			return nil, err
		}
	}

	result := strings.TrimSpace(string(output))
	if result == "" {
		return []string{}, nil
	}

	return strings.Split(result, "\n"), nil
}

func getBranchForCommit(hash string) (string, error) {
	// Get branches containing this commit
	cmd := exec.Command("git", "branch", "--contains", hash, "--format=%(refname:short)")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	branches := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(branches) > 0 && branches[0] != "" {
		return branches[0], nil
	}

	return "unknown", nil
}
