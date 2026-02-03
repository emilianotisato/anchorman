package hooks

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const postCommitHook = `#!/bin/bash
# Anchorman post-commit hook
# Chain existing hook if present
if [ -x "$0.legacy" ]; then
    "$0.legacy" "$@"
fi
# Ingest commit (silent, non-blocking)
anchorman ingest 2>/dev/null &
`

const postMergeHook = `#!/bin/bash
# Anchorman post-merge hook
# Chain existing hook if present
if [ -x "$0.legacy" ]; then
    "$0.legacy" "$@"
fi
# Ingest commit (silent, non-blocking)
anchorman ingest 2>/dev/null &
`

// HooksDir returns the global git hooks directory
func HooksDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "git", "hooks"), nil
}

// Install sets up global git hooks for commit tracking
func Install() error {
	hooksDir, err := HooksDir()
	if err != nil {
		return err
	}

	// Create hooks directory
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	// Check current core.hooksPath
	currentPath, err := getGitConfig("core.hooksPath")
	if err == nil && currentPath != "" && currentPath != hooksDir {
		// There's an existing hooksPath, we need to preserve those hooks
		fmt.Printf("Note: Found existing hooks at %s\n", currentPath)
		if err := migrateExistingHooks(currentPath, hooksDir); err != nil {
			return fmt.Errorf("failed to migrate existing hooks: %w", err)
		}
	}

	// Install post-commit hook
	if err := installHook(hooksDir, "post-commit", postCommitHook); err != nil {
		return err
	}

	// Install post-merge hook
	if err := installHook(hooksDir, "post-merge", postMergeHook); err != nil {
		return err
	}

	// Set global hooks path
	if err := setGitConfig("core.hooksPath", hooksDir); err != nil {
		return fmt.Errorf("failed to set core.hooksPath: %w", err)
	}

	return nil
}

// Uninstall removes anchorman git hooks
func Uninstall() error {
	hooksDir, err := HooksDir()
	if err != nil {
		return err
	}

	// Remove our hooks
	for _, hookName := range []string{"post-commit", "post-merge"} {
		hookPath := filepath.Join(hooksDir, hookName)
		legacyPath := hookPath + ".legacy"

		// Check if this is our hook
		content, err := os.ReadFile(hookPath)
		if err == nil && strings.Contains(string(content), "Anchorman") {
			// Remove our hook
			os.Remove(hookPath)

			// Restore legacy hook if exists
			if _, err := os.Stat(legacyPath); err == nil {
				os.Rename(legacyPath, hookPath)
				fmt.Printf("Restored original %s hook\n", hookName)
			}
		}
	}

	// Check if hooks directory is empty
	entries, err := os.ReadDir(hooksDir)
	if err == nil && len(entries) == 0 {
		// Remove empty directory and unset config
		os.Remove(hooksDir)
		unsetGitConfig("core.hooksPath")
	}

	return nil
}

func installHook(hooksDir, name, content string) error {
	hookPath := filepath.Join(hooksDir, name)
	legacyPath := hookPath + ".legacy"

	// Check if there's an existing hook that's not ours
	if existingContent, err := os.ReadFile(hookPath); err == nil {
		if !strings.Contains(string(existingContent), "Anchorman") {
			// Backup existing hook
			if err := os.Rename(hookPath, legacyPath); err != nil {
				return fmt.Errorf("failed to backup existing %s hook: %w", name, err)
			}
			fmt.Printf("Backed up existing %s hook to %s.legacy\n", name, name)
		}
	}

	// Write our hook
	if err := os.WriteFile(hookPath, []byte(content), 0755); err != nil {
		return fmt.Errorf("failed to write %s hook: %w", name, err)
	}

	return nil
}

func migrateExistingHooks(oldDir, newDir string) error {
	entries, err := os.ReadDir(oldDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		oldPath := filepath.Join(oldDir, entry.Name())
		newPath := filepath.Join(newDir, entry.Name()+".legacy")

		// Copy file
		content, err := os.ReadFile(oldPath)
		if err != nil {
			continue
		}

		if err := os.WriteFile(newPath, content, 0755); err != nil {
			return err
		}
		fmt.Printf("Migrated %s to %s\n", entry.Name(), newPath)
	}

	return nil
}

func getGitConfig(key string) (string, error) {
	cmd := exec.Command("git", "config", "--global", key)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func setGitConfig(key, value string) error {
	cmd := exec.Command("git", "config", "--global", key, value)
	return cmd.Run()
}

func unsetGitConfig(key string) error {
	cmd := exec.Command("git", "config", "--global", "--unset", key)
	return cmd.Run()
}
