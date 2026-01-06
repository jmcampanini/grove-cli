package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/spf13/cobra"
)

var prPreviewFzfFlag bool

var prPreviewCmd = &cobra.Command{
	Use:   "preview [number]",
	Short: "Show pull request details",
	Long: `Show detailed information about a pull request.

Displays PR metadata (title, author, branch, state), a list of changed files
with additions/deletions counts, and the PR body.

With --fzf, errors are printed to stdout instead of returning an error code,
making it suitable for use in fzf preview panes.`,
	Args: cobra.ExactArgs(1),
	RunE: runPRPreview,
}

func init() {
	prPreviewCmd.Flags().BoolVar(&prPreviewFzfFlag, "fzf", false, "Print errors to stdout instead of returning error (for fzf preview)")
	prCmd.AddCommand(prPreviewCmd)
}

// handlePreviewError handles errors based on the --fzf flag.
// In fzf mode, prints error to stdout and returns nil.
// Otherwise, returns the error normally.
func handlePreviewError(cmd *cobra.Command, err error) error {
	if prPreviewFzfFlag {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Error: %v\n", err)
		return nil
	}
	return err
}

func runPRPreview(cmd *cobra.Command, args []string) error {
	// Parse PR number
	prNum, err := strconv.Atoi(args[0])
	if err != nil {
		return handlePreviewError(cmd, fmt.Errorf("invalid PR number: %s", args[0]))
	}

	cwd, err := os.Getwd()
	if err != nil {
		return handlePreviewError(cmd, fmt.Errorf("failed to get current directory: %w", err))
	}

	// Load config to get timeout
	gitClient := git.New(false, cwd, config.DefaultConfig().Git.Timeout)

	worktreeRoot, err := gitClient.GetWorktreeRoot()
	if err != nil {
		return handlePreviewError(cmd, fmt.Errorf("git error: %w", err))
	}
	if worktreeRoot == "" {
		return handlePreviewError(cmd, fmt.Errorf("grove must be run inside a git repository"))
	}

	mainWorktreePath, err := gitClient.GetMainWorktreePath()
	if err != nil {
		return handlePreviewError(cmd, fmt.Errorf("failed to get main worktree path: %w", err))
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return handlePreviewError(cmd, fmt.Errorf("failed to get user home directory: %w", err))
	}

	configPaths := config.ConfigPaths(cwd, worktreeRoot, mainWorktreePath, homeDir)
	loader := config.NewDefaultLoader()
	loadResult, err := loader.Load(configPaths)
	if err != nil {
		return handlePreviewError(cmd, fmt.Errorf("failed to load config: %w", err))
	}
	cfg := loadResult.Config

	// Create GitHub client and validate
	gh := github.New(cwd, cfg.Git.Timeout)
	if err := gh.Validate(); err != nil {
		return handlePreviewError(cmd, err)
	}

	// Fetch PR info
	pr, err := gh.GetPullRequest(prNum)
	if err != nil {
		return handlePreviewError(cmd, err)
	}

	// Fetch PR files
	files, err := gh.GetPullRequestFiles(prNum)
	if err != nil {
		return handlePreviewError(cmd, err)
	}

	// Display PR preview
	return outputPRPreview(cmd, pr, files)
}

// outputPRPreview renders the PR preview to stdout.
func outputPRPreview(cmd *cobra.Command, pr github.PullRequest, files []github.PullRequestFile) error {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("PR #%d\n", pr.Number))
	sb.WriteString(strings.Repeat("\u2500", 29)) // horizontal line
	sb.WriteString("\n")

	// Metadata
	sb.WriteString(fmt.Sprintf("Title:  %s\n", pr.Title))
	sb.WriteString(fmt.Sprintf("Author: %s\n", pr.AuthorLogin))
	sb.WriteString(fmt.Sprintf("Branch: %s\n", pr.BranchName))
	sb.WriteString(fmt.Sprintf("State:  %s\n", strings.ToLower(string(pr.State))))
	sb.WriteString("\n")

	// Files list
	const maxFiles = 30
	sb.WriteString(fmt.Sprintf("Files changed (%d):\n", pr.FilesChanged))

	displayCount := len(files)
	if displayCount > maxFiles {
		displayCount = maxFiles
	}

	for _, f := range files[:displayCount] {
		sb.WriteString(fmt.Sprintf("  %s (+%d, -%d)\n", f.Path, f.Additions, f.Deletions))
	}

	if len(files) > maxFiles {
		sb.WriteString(fmt.Sprintf("  (and %d more files...)\n", len(files)-maxFiles))
	}

	// Body
	sb.WriteString("\n")
	sb.WriteString(pr.Body)
	sb.WriteString("\n")

	_, err := fmt.Fprint(cmd.OutOrStdout(), sb.String())
	return err
}
