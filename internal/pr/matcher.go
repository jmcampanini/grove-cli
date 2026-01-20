package pr

import (
	"github.com/jmcampanini/grove-cli/internal/git"
	"github.com/jmcampanini/grove-cli/internal/github"
	"github.com/jmcampanini/grove-cli/internal/naming"
)

// WorktreeMatch represents a PR with its worktree matching status.
type WorktreeMatch struct {
	HasWorktree  bool
	PR           github.PullRequest
	WorktreePath string
}

// Matcher matches pull requests to existing worktrees.
type Matcher struct {
	namer *naming.PRWorktreeNamer
}

// NewMatcher creates a new Matcher with the given PRWorktreeNamer.
func NewMatcher(namer *naming.PRWorktreeNamer) *Matcher {
	return &Matcher{
		namer: namer,
	}
}

// Match returns a WorktreeMatch for each PR, indicating whether a worktree exists.
func (m *Matcher) Match(prs []github.PullRequest, worktrees []git.Worktree) []WorktreeMatch {
	result := make([]WorktreeMatch, len(prs))
	for i, pr := range prs {
		wt := m.FindWorktreeForPR(pr, worktrees)
		match := WorktreeMatch{
			PR: pr,
		}
		if wt != nil {
			match.HasWorktree = true
			match.WorktreePath = wt.AbsolutePath
		}
		result[i] = match
	}
	return result
}

// FindWorktreeForPR searches worktrees for one that matches the given PR.
// It uses a dual-match strategy:
// 1. Template-generated branch name (for worktrees created via grove pr create)
// 2. PR's remote branch name directly (for manually created worktrees)
// Returns nil if no match is found.
func (m *Matcher) FindWorktreeForPR(pr github.PullRequest, worktrees []git.Worktree) *git.Worktree {
	// Apply template to get expected local branch name
	prData := naming.PRTemplateData{
		BranchName: pr.BranchName,
		Number:     pr.Number,
	}
	expectedBranch, err := m.namer.GenerateBranchName(prData)
	if err != nil {
		expectedBranch = "" // Continue with direct match only
	}

	// Search worktrees for matching branch
	for i := range worktrees {
		if branch, ok := worktrees[i].Ref.FullBranch(); ok {
			// Match 1: Template-generated branch name (grove pr create)
			if expectedBranch != "" && branch.Name == expectedBranch {
				return &worktrees[i]
			}
			// Match 2: PR's remote branch name directly (manual worktrees)
			if branch.Name == pr.BranchName {
				return &worktrees[i]
			}
		}
	}
	return nil
}
