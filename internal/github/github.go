package github

type GitHub interface {

	// GetAuthStatus returns the output of `gh auth status`.
	GetAuthStatus() (string, error)

	// GetPullRequest returns a single pull request by number.
	GetPullRequest(prNum int) (PullRequest, error)

	// GetPullRequestByBranch returns the pull request for the given branch name.
	// Returns nil if no pull request exists for the branch.
	GetPullRequestByBranch(branchName string) (*PullRequest, error)

	// ListPullRequests returns a list of pull requests matching the given query.
	ListPullRequests(query PRQuery) ([]PullRequest, error)
}
