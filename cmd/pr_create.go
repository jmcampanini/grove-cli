package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jmcampanini/grove-cli/internal/config"
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/jmcampanini/grove-cli/internal/naming"
	"github.com/jmcampanini/grove-cli/internal/pr"
	"github.com/spf13/cobra"
)

var prCreateCmd = &cobra.Command{
	Use:   "create [number]",
	Short: "Create worktree from pull request",
	Long: `Create a local worktree from a GitHub pull request.

Note: Only works with PRs from the same repository. Fork PRs are not yet supported.`,
	Args: cobra.ExactArgs(1),
	RunE: runPRCreate,
}

func init() {
	prCmd.AddCommand(prCreateCmd)
}

func runPRCreate(cmd *cobra.Command, args []string) error {
	return runPRCreateWithDeps(cmd, args, nil, nil)
}

// prCreateDeps holds injectable dependencies for testing.
type prCreateDeps struct {
	gh  github.GitHub
	git git.Git
}

// prCreateContext holds the resolved dependencies for the pr create command.
type prCreateContext struct {
	cfg       config.Config
	ghClient  github.GitHub
	gitClient git.Git
}

func runPRCreateWithDeps(cmd *cobra.Command, args []string, deps *prCreateDeps, cfg *config.Config) error {
	// Step 1: Parse PR number from args
	prNum, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid PR number: %s", args[0])
	}

	// Initialize context with deps or from environment
	ctx, err := initPRCreateContext(deps, cfg)
	if err != nil {
		return err
	}

	// Step 2: Validate gh CLI
	if err := ctx.ghClient.Validate(); err != nil {
		return err
	}

	// Step 3: Fetch PR info
	prInfo, err := ctx.ghClient.GetPullRequest(prNum)
	if err != nil {
		return fmt.Errorf("failed to get pull request: %w", err)
	}

	// Step 4: Detect fork PRs (fail fast)
	if prInfo.IsCrossRepository {
		return fmt.Errorf("PR #%d is from a fork, which is not yet supported.\nTip: You can manually add the fork as a remote and create a worktree with 'git worktree add'", prInfo.Number)
	}

	// Step 5: Warn if merged/closed (but continue)
	if prInfo.State == github.PRStateMerged || prInfo.State == github.PRStateClosed {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Note: PR #%d is %s\n", prInfo.Number, strings.ToLower(string(prInfo.State)))
	}

	return createPRWorktree(cmd, ctx, prInfo)
}

// initPRCreateContext initializes the context from deps (for testing) or from environment.
func initPRCreateContext(deps *prCreateDeps, cfg *config.Config) (*prCreateContext, error) {
	if deps != nil {
		loadedCfg := config.DefaultConfig()
		if cfg != nil {
			loadedCfg = *cfg
		}
		return &prCreateContext{
			cfg:       loadedCfg,
			ghClient:  deps.gh,
			gitClient: deps.git,
		}, nil
	}

	return initPRCreateContextFromEnv()
}

// initPRCreateContextFromEnv loads config and creates clients from the environment.
func initPRCreateContextFromEnv() (*prCreateContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	// Create git client with default timeout first
	gitClient := git.New(false, cwd, config.DefaultConfig().Git.Timeout)

	worktreeRoot, err := gitClient.GetWorktreeRoot()
	if err != nil {
		return nil, fmt.Errorf("git error: %w", err)
	}
	if worktreeRoot == "" {
		return nil, fmt.Errorf("grove must be run inside a git repository")
	}

	mainWorktreePath, err := gitClient.GetMainWorktreePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get main worktree path: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	configPaths := config.ConfigPaths(cwd, worktreeRoot, mainWorktreePath, homeDir)
	loader := config.NewDefaultLoader()
	loadResult, err := loader.Load(configPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Recreate git client with configured timeout
	return &prCreateContext{
		cfg:       loadResult.Config,
		ghClient:  github.New(cwd, loadResult.Config.Git.Timeout),
		gitClient: git.New(false, cwd, loadResult.Config.Git.Timeout),
	}, nil
}

// createPRWorktree handles the worktree creation logic after PR validation.
func createPRWorktree(cmd *cobra.Command, ctx *prCreateContext, prInfo github.PullRequest) error {
	// Step 6: Generate local branch name via template
	namer, err := naming.NewPRWorktreeNamer(ctx.cfg.PR, ctx.cfg.Slugify)
	if err != nil {
		return fmt.Errorf("failed to create PR namer: %w", err)
	}

	prData := naming.PRTemplateData{
		BranchName: prInfo.BranchName,
		Number:     prInfo.Number,
	}
	localBranch, err := namer.GenerateBranchName(prData)
	if err != nil {
		return fmt.Errorf("failed to generate branch name: %w", err)
	}

	// Step 7: Generate worktree name
	worktreeName := namer.GenerateWorktreeName(localBranch)
	if worktreeName == "" {
		return fmt.Errorf("failed to generate worktree name: empty result")
	}

	// Step 8: Check if worktree exists using Matcher
	worktrees, err := ctx.gitClient.ListWorktrees()
	if err != nil {
		return fmt.Errorf("failed to list worktrees: %w", err)
	}

	matcher := pr.NewMatcher(namer)
	existingWorktree := matcher.FindWorktreeForPR(prInfo, worktrees)

	if existingWorktree != nil {
		// Output path to stdout, info message to stderr
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Worktree already exists\n")
		_, err := fmt.Fprintln(cmd.OutOrStdout(), existingWorktree.AbsolutePath)
		return err
	}

	// Step 9: Check for path collision
	workspacePath, err := ctx.gitClient.GetWorkspacePath()
	if err != nil {
		return fmt.Errorf("failed to get workspace path: %w", err)
	}

	wtPath := filepath.Join(workspacePath, worktreeName)

	if _, err := os.Stat(wtPath); err == nil {
		// Path exists but Matcher didn't find a matching worktree
		return fmt.Errorf("worktree path %s already exists (not a PR worktree or different branch)", wtPath)
	}

	// Step 10: Check if local branch already exists
	branchExists, err := ctx.gitClient.BranchExists(localBranch, false)
	if err != nil {
		return fmt.Errorf("failed to check branch existence: %w", err)
	}

	// Step 11: Fetch remote branch (if local branch doesn't exist)
	if !branchExists {
		if err := ctx.gitClient.FetchRemoteBranch("origin", prInfo.BranchName, localBranch); err != nil {
			return fmt.Errorf("failed to fetch remote branch: %w", err)
		}
	}

	// Step 12: Create worktree
	if err := ctx.gitClient.CreateWorktreeForExistingBranch(localBranch, wtPath); err != nil {
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	// Step 13: Output absolute path to stdout
	_, err = fmt.Fprintln(cmd.OutOrStdout(), wtPath)
	return err
}
