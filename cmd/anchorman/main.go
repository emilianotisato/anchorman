package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/emilianohg/anchorman/internal/config"
	"github.com/emilianohg/anchorman/internal/db"
	"github.com/emilianohg/anchorman/internal/git"
	"github.com/emilianohg/anchorman/internal/git/hooks"
	"github.com/emilianohg/anchorman/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "anchorman",
	Short: "Git activity tracker and report generator",
	Long:  `Anchorman tracks your git commits across multiple repos and generates reports for managers.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load config
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Open database (migrations will be checked in dashboard)
		database, err := db.Open()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
			os.Exit(1)
		}
		defer db.Close()

		// Run initial migration if this is a fresh database
		// This handles first-time setup without user interaction
		status, _ := db.GetMigrationStatus()
		if status != nil && status.CurrentVersion == 0 {
			if err := db.RunMigrations(); err != nil {
				fmt.Fprintf(os.Stderr, "Error running initial migrations: %v\n", err)
				os.Exit(1)
			}
		}

		// Launch TUI
		if err := tui.Run(database, cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var ingestCmd = &cobra.Command{
	Use:    "ingest",
	Short:  "Record the current commit to the database (called by git hooks)",
	Hidden: true,
	Run: func(cmd *cobra.Command, args []string) {
		verbose, _ := cmd.Flags().GetBool("verbose")

		result, err := git.Ingest(verbose)
		if err != nil {
			logError(err)
			if verbose {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			os.Exit(1)
		}

		if verbose {
			if result.Skipped {
				fmt.Printf("Skipped: %s\n", result.SkipReason)
			} else {
				fmt.Printf("Recorded commit %s in %s\n", result.CommitHash[:8], result.RepoPath)
				fmt.Printf("Message: %s\n", result.Message)
			}
		}
	},
}

var hooksCmd = &cobra.Command{
	Use:   "hooks",
	Short: "Manage git hooks",
}

var hooksInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install global git hooks for commit tracking",
	Run: func(cmd *cobra.Command, args []string) {
		if err := hooks.Install(); err != nil {
			fmt.Fprintf(os.Stderr, "Error installing hooks: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Global git hooks installed successfully!")
		fmt.Println("All commits in your configured scan_paths will now be tracked.")
	},
}

var hooksUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove anchorman git hooks",
	Run: func(cmd *cobra.Command, args []string) {
		if err := hooks.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Error uninstalling hooks: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Anchorman git hooks removed.")
	},
}

var importCmd = &cobra.Command{
	Use:   "import [count|date]",
	Short: "Import commits from the current git repository",
	Long: `Import historical commits from the current git repository.

Examples:
  anchorman import 10          # Last 10 commits
  anchorman import 2025-01-15  # Commits since date
  anchorman import             # All commits
  anchorman import 10 -f       # Force re-import last 10`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		opts := git.ImportOptions{}

		if len(args) > 0 {
			// Try parse as number first
			if count, err := strconv.Atoi(args[0]); err == nil {
				opts.Count = count
			} else if date, err := time.Parse("2006-01-02", args[0]); err == nil {
				opts.Since = date
			} else {
				fmt.Fprintf(os.Stderr, "Invalid argument: %s (expected number or YYYY-MM-DD)\n", args[0])
				os.Exit(1)
			}
		}

		opts.Branch, _ = cmd.Flags().GetString("branch")
		opts.Force, _ = cmd.Flags().GetBool("force")

		fmt.Println("Importing commits...")

		result, err := git.Import(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Repository: %s\n", result.RepoPath)
		fmt.Printf("Found: %d commits\n", result.TotalFound)
		fmt.Printf("Imported: %d\n", result.Imported)
		fmt.Printf("Skipped: %d (already exist)\n", result.Skipped)

		if opts.Force {
			fmt.Printf("Updated: %d (force mode)\n", result.Updated)
			fmt.Printf("Tasks deleted: %d\n", result.TasksDeleted)
		}

		if result.IsOrphan {
			fmt.Println("\nNote: Repository is not assigned to a project yet.")
			fmt.Println("Use the TUI to assign it to a company/project.")
		}

		if result.NotInScanPath {
			fmt.Println("\nWarning: Repository path is not in configured scan_paths.")
			fmt.Println("Future commits won't be auto-tracked until you add the path to config.")
		}

		fmt.Println("\nDone! Use 'anchorman' to process commits into tasks.")
	},
}

func init() {
	ingestCmd.Flags().Bool("verbose", false, "Print what was recorded")

	hooksCmd.AddCommand(hooksInstallCmd)
	hooksCmd.AddCommand(hooksUninstallCmd)

	importCmd.Flags().StringP("branch", "b", "", "Specific branch (default: all branches)")
	importCmd.Flags().BoolP("force", "f", false, "Re-ingest existing commits, mark as unprocessed, delete related tasks")

	rootCmd.AddCommand(ingestCmd)
	rootCmd.AddCommand(hooksCmd)
	rootCmd.AddCommand(importCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func logError(err error) {
	logPath, pathErr := config.ErrorLogPath()
	if pathErr != nil {
		return
	}

	// Ensure directory exists
	if err := config.EnsureDirectories(); err != nil {
		return
	}

	f, fileErr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if fileErr != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "[%s] %v\n", "ingest", err)
}
