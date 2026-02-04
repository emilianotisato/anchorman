package agent

import (
	"bytes"
	"fmt"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/emilianohg/anchorman/internal/models"
)

// TaskResult represents a processed task with its time estimate
type TaskResult struct {
	Description    string
	EstimatedHours float64
}

type Agent interface {
	Process(projectName string, commits []models.RawCommit) ([]TaskResult, error)
}

func New(agentType string) (Agent, error) {
	switch agentType {
	case "codex":
		return &CodexAgent{}, nil
	case "claude":
		return &ClaudeAgent{}, nil
	default:
		return nil, fmt.Errorf("unknown agent type: %s", agentType)
	}
}

type CodexAgent struct{}

func (a *CodexAgent) Process(projectName string, commits []models.RawCommit) ([]TaskResult, error) {
	prompt := buildPrompt(projectName, commits)

	// Use codex exec for non-interactive mode, pass prompt via stdin
	cmd := exec.Command("codex", "exec", "-")
	cmd.Stdin = strings.NewReader(prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("codex failed: %w\nstderr: %s", err, stderr.String())
	}

	return parseResponse(stdout.String()), nil
}

type ClaudeAgent struct{}

func (a *ClaudeAgent) Process(projectName string, commits []models.RawCommit) ([]TaskResult, error) {
	prompt := buildPrompt(projectName, commits)

	// Use claude -p for non-interactive print mode
	cmd := exec.Command("claude", "-p", prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("claude failed: %w\nstderr: %s", err, stderr.String())
	}

	return parseResponse(stdout.String()), nil
}

func buildPrompt(projectName string, commits []models.RawCommit) string {
	var sb strings.Builder

	sb.WriteString("You are analyzing git commits to create human-readable task summaries for manager reports.\n\n")
	sb.WriteString(fmt.Sprintf("Project: %s\n\n", projectName))
	sb.WriteString("Commits:\n")

	for _, c := range commits {
		files := strings.Join(c.FilesChanged, ", ")
		if len(files) > 100 {
			files = files[:100] + "..."
		}
		sb.WriteString(fmt.Sprintf("- %s: %s (branch: %s, files: %s)\n",
			c.Hash[:8], c.Message, c.Branch, files))
	}

	sb.WriteString("\nCreate a list of conceptual tasks that summarize the work done.\n")
	sb.WriteString("- Group related commits into single tasks\n")
	sb.WriteString("- Use plain, non-technical language suitable for managers\n")
	sb.WriteString("- Focus on WHAT was accomplished, not HOW\n")
	sb.WriteString("- Each task should be a single line starting with a verb (Implemented, Fixed, Added, Updated, etc.)\n")
	sb.WriteString("\nFor each task, estimate the time spent based on:\n")
	sb.WriteString("- Number of commits involved\n")
	sb.WriteString("- Number and types of files changed\n")
	sb.WriteString("- Complexity implied by commit messages\n")
	sb.WriteString("\nUse 0.5 hour increments (minimum 0.5h). Examples: 0.5, 1.0, 1.5, 2.0, 2.5, etc.\n")
	sb.WriteString("\nOutput format: - [X.Xh] Task description\n")
	sb.WriteString("Examples:\n")
	sb.WriteString("- [2.0h] Implemented user authentication system\n")
	sb.WriteString("- [0.5h] Fixed login button styling\n")
	sb.WriteString("- [1.5h] Refactored database connection handling\n")
	sb.WriteString("\nOutput ONLY the tasks in this format:\n")

	return sb.String()
}

// timePattern matches [X.Xh] at the start of a task line
var timePattern = regexp.MustCompile(`^\[(\d+\.?\d*)h\]\s*`)

func parseResponse(response string) []TaskResult {
	var tasks []TaskResult
	lines := strings.Split(response, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Remove leading "- " or "* " if present
		if strings.HasPrefix(line, "- ") {
			line = strings.TrimPrefix(line, "- ")
		} else if strings.HasPrefix(line, "* ") {
			line = strings.TrimPrefix(line, "* ")
		}

		// Skip empty or very short lines
		if len(line) < 5 {
			continue
		}

		// Extract time estimate and description
		result := TaskResult{
			EstimatedHours: 0.5, // default
		}

		if match := timePattern.FindStringSubmatch(line); match != nil {
			if hours, err := strconv.ParseFloat(match[1], 64); err == nil {
				result.EstimatedHours = roundToHalfHour(hours)
			}
			result.Description = strings.TrimSpace(timePattern.ReplaceAllString(line, ""))
		} else {
			result.Description = line
		}

		if result.Description != "" {
			tasks = append(tasks, result)
		}
	}

	return tasks
}

// roundToHalfHour rounds hours to nearest 0.5h (minimum 0.5h)
func roundToHalfHour(hours float64) float64 {
	if hours < 0.5 {
		return 0.5
	}
	return math.Round(hours*2) / 2
}
