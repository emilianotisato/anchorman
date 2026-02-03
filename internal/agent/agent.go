package agent

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/emilianohg/anchorman/internal/models"
)

type Agent interface {
	Process(projectName string, commits []models.RawCommit) ([]string, error)
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

func (a *CodexAgent) Process(projectName string, commits []models.RawCommit) ([]string, error) {
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

func (a *ClaudeAgent) Process(projectName string, commits []models.RawCommit) ([]string, error) {
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
	sb.WriteString("\nOutput ONLY the tasks, one per line, starting with '- ':\n")

	return sb.String()
}

func parseResponse(response string) []string {
	var tasks []string
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

		tasks = append(tasks, line)
	}

	return tasks
}
