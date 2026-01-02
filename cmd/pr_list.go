package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/dustin/go-humanize"
	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/jmcampanini/grove-cli/internal/pr"
	"github.com/spf13/cobra"
)

var prListFzfFlag bool

var prListCmd = &cobra.Command{
	Use:   "list",
	Short: "List open pull requests",
	Long: `List open pull requests for the current repository.

By default, outputs a formatted table with PR details.

With --fzf, outputs tab-separated format suitable for fzf integration:
  <number>\t<searchable>\t<display>

The "Local" column shows a checkmark when a worktree exists for the PR.`,
	Args: cobra.NoArgs,
	RunE: runPRList,
}

func init() {
	prListCmd.Flags().BoolVar(&prListFzfFlag, "fzf", false, "Output in fzf-compatible format")
	prCmd.AddCommand(prListCmd)
}

func runPRList(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	// Load config to get timeout and PR settings
	gitClient := git.New(false, cwd, config.DefaultConfig().Git.Timeout)

	worktreeRoot, err := gitClient.GetWorktreeRoot()
	if err != nil {
		return fmt.Errorf("git error: %w", err)
	}
	if worktreeRoot == "" {
		return fmt.Errorf("grove must be run inside a git repository")
	}

	mainWorktreePath, err := gitClient.GetMainWorktreePath()
	if err != nil {
		return fmt.Errorf("failed to get main worktree path: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	configPaths := config.ConfigPaths(cwd, worktreeRoot, mainWorktreePath, homeDir)
	loader := config.NewDefaultLoader()
	loadResult, err := loader.Load(configPaths)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	cfg := loadResult.Config

	// Recreate git client with configured timeout
	gitClient = git.New(false, cwd, cfg.Git.Timeout)

	// Create GitHub client and validate
	gh := github.New(cwd, cfg.Git.Timeout)
	if err := gh.Validate(); err != nil {
		return err
	}

	// Create PRWorktreeNamer for matching
	namer, err := naming.NewPRWorktreeNamer(cfg.PR, cfg.Slugify)
	if err != nil {
		return fmt.Errorf("failed to create PR namer: %w", err)
	}

	// Fetch open PRs - explicitly use PRStateOpen for consistent behavior
	// Future: add --state flag to allow filtering by state
	query := github.PRQuery{State: github.PRStateOpen}
	prs, err := gh.ListPullRequests(query, github.DefaultPRLimit)
	if err != nil {
		return fmt.Errorf("failed to list pull requests: %w", err)
	}

	// Fetch worktrees for matching
	worktrees, err := gitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	// Match PRs with worktrees
	matcher := pr.NewMatcher(namer)
	matches := matcher.Match(prs, worktrees)

	// Output based on format flag
	if prListFzfFlag {
		return outputPRListFzf(cmd, matches)
	}
	return outputPRListTable(cmd, matches)
}

// outputPRListTable renders a lipgloss table to stdout.
func outputPRListTable(cmd *cobra.Command, matches []pr.WorktreeMatch) error {
	if len(matches) == 0 {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "No open pull requests found.")
		return err
	}

	// Define colors
	purple := lipgloss.Color("99")
	gray := lipgloss.Color("245")
	lightGray := lipgloss.Color("241")

	// Define styles
	headerStyle := lipgloss.NewStyle().Foreground(purple).Bold(true).Align(lipgloss.Center)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	oddRowStyle := cellStyle.Foreground(gray)
	evenRowStyle := cellStyle.Foreground(lightGray)

	// Build rows
	rows := make([][]string, len(matches))
	for i, match := range matches {
		localMarker := ""
		if match.HasWorktree {
			localMarker = "\u2713" // checkmark
		}

		state := strings.ToLower(string(match.PR.State))
		updated := humanize.Time(match.PR.UpdatedAt)

		rows[i] = []string{
			fmt.Sprintf("%d", match.PR.Number),
			truncateString(match.PR.Title, 40),
			match.PR.AuthorLogin,
			truncateString(match.PR.BranchName, 30),
			state,
			localMarker,
			updated,
		}
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(purple)).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return headerStyle
			case row%2 == 0:
				return evenRowStyle
			default:
				return oddRowStyle
			}
		}).
		Headers("#", "Title", "Author", "Branch", "State", "Local", "Updated").
		Rows(rows...)

	_, err := fmt.Fprintln(cmd.OutOrStdout(), t)
	return err
}

// outputPRListFzf renders fzf-compatible TSV format.
// Format: <number>\t<searchable>\t<display>
func outputPRListFzf(cmd *cobra.Command, matches []pr.WorktreeMatch) error {
	for _, match := range matches {
		number := fmt.Sprintf("%d", match.PR.Number)

		// Column 2: Searchable content (space-separated)
		// Includes: number, title, branch, author, state
		state := strings.ToLower(string(match.PR.State))
		searchable := sanitizeFzfField(fmt.Sprintf("%d %s %s %s %s",
			match.PR.Number,
			match.PR.Title,
			match.PR.BranchName,
			match.PR.AuthorLogin,
			state,
		))

		// Column 3: Display string
		// Format: "checkmark #123 Title [author] branch" or "#123 Title [author] branch"
		localPrefix := ""
		if match.HasWorktree {
			localPrefix = "\u2713 " // checkmark with space
		}
		display := sanitizeFzfField(fmt.Sprintf("%s#%d %s [%s] %s",
			localPrefix,
			match.PR.Number,
			match.PR.Title,
			match.PR.AuthorLogin,
			match.PR.BranchName,
		))

		_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", number, searchable, display)
		if err != nil {
			return err
		}
	}
	return nil
}

// sanitizeFzfField replaces tabs and newlines with spaces to prevent fzf parsing issues.
func sanitizeFzfField(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
