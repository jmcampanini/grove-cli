package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	clog "github.com/charmbracelet/log"
)

// GitCli provides high-level git operations by executing real git commands via the git CLI.
type GitCli struct {
	dryRun     bool
	log        *clog.Logger
	timeout    time.Duration
	workingDir string
}

var _ Git = &GitCli{}

// New creates a new GitCli instance that executes git commands in the specified working directory.
func New(dryRun bool, workingDir string, timeout time.Duration) Git {
	return &GitCli{
		dryRun:     dryRun,
		log:        clog.Default().WithPrefix("git"),
		timeout:    timeout,
		workingDir: workingDir,
	}
}

func (g *GitCli) executeGitCommand(args ...string) (string, error) {
	g.log.Debug("Executing git command", "cmd", "git", "args", args, "workingDir", g.workingDir)

	ctx, cancel := context.WithTimeout(context.Background(), g.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.workingDir
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			g.log.Warn("git command timed out", "args", args, "timeout", g.timeout, "error", err)
			return "", fmt.Errorf("git %s timed out after %s", strings.Join(args, " "), g.timeout)
		}
		g.log.Warn("Git command failed", "args", args, "stderr", stderr.String(), "error", err)
		return "", fmt.Errorf("git %s failed: %w: %s", strings.Join(args, " "), err, stderr.String())
	}

	output := strings.TrimSpace(stdout.String())
	g.log.Debug("Git command succeeded", "args", args, "output", output)
	return output, nil
}

// executeMutatingCommand runs a git command that modifies state, unless in dry-run mode.
func (g *GitCli) executeMutatingCommand(errContext string, args ...string) error {
	if g.dryRun {
		g.log.Info("Would execute git command", "cmd", "git", "args", args)
		return nil
	}
	if _, err := g.executeGitCommand(args...); err != nil {
		return fmt.Errorf("%s: %w", errContext, err)
	}
	return nil
}

// executeMutatingCommandWithOutput runs a git command that modifies state and returns its output.
func (g *GitCli) executeMutatingCommandWithOutput(errContext string, args ...string) (string, error) {
	if g.dryRun {
		g.log.Info("Would execute git command", "cmd", "git", "args", args)
		return fmt.Sprintf("Would execute: git %s", strings.Join(args, " ")), nil
	}
	output, err := g.executeGitCommand(args...)
	if err != nil {
		return output, fmt.Errorf("%s: %w", errContext, err)
	}
	return output, nil
}

func (g *GitCli) GetMainWorktreePath() (string, error) {
	commonDir, err := g.executeGitCommand("rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("failed to get git common dir: %w", err)
	}

	absCommonDir := commonDir
	if !filepath.IsAbs(commonDir) {
		absCommonDir = filepath.Join(g.workingDir, commonDir)
	}

	absCommonDir, err = filepath.Abs(absCommonDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	absCommonDir = filepath.Clean(absCommonDir)
	mainWorktree := filepath.Dir(absCommonDir)

	g.log.Debug("Resolved main worktree path", "commonDir", commonDir, "mainWorktree", mainWorktree)
	return mainWorktree, nil
}

func (g *GitCli) GetWorktreeRoot() (string, error) {
	output, err := g.executeGitCommand("rev-parse", "--show-toplevel")
	if err != nil {
		if strings.Contains(err.Error(), "not a git repo") {
			// Not in a git repo - this is a valid state, not an error
			return "", nil
		}
		return "", fmt.Errorf("git command failed: %w", err)
	}
	return output, nil
}

func (g *GitCli) GetCurrentBranch() (string, error) {
	output, err := g.executeGitCommand("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}
	return output, nil
}

func (g *GitCli) GetCommitSubject() (string, error) {
	output, err := g.executeGitCommand("log", "-1", "--format=%s")
	if err != nil {
		return "", fmt.Errorf("failed to get commit subject: %w", err)
	}
	return output, nil
}

func (g *GitCli) GetDefaultRemote(fallback string) (string, error) {
	output, err := g.executeGitCommand("config", "--get", "remote.pushDefault")
	if err == nil && output != "" {
		g.log.Debug("Found remote.pushDefault", "remote", output)
		return output, nil
	}

	g.log.Debug("No remote.pushDefault configured, using fallback", "fallback", fallback)
	return fallback, nil
}

// remoteExists checks if a remote with the given name is configured.
func (g *GitCli) remoteExists(remoteName string) (bool, error) {
	_, err := g.executeGitCommand("remote", "get-url", remoteName)
	if err == nil {
		return true, nil
	}

	if strings.Contains(err.Error(), "No such remote") {
		return false, nil
	}

	return false, err
}

func (g *GitCli) GetRepoDefaultBranch(remoteName string) (string, error) {
	if exists, err := g.remoteExists(remoteName); err != nil {
		return "", fmt.Errorf("failed to check remote existence: %w", err)
	} else if !exists {
		return "", fmt.Errorf("remote '%s' does not exist", remoteName)
	}

	output, err := g.executeGitCommand("rev-parse", "--abbrev-ref", remoteName+"/HEAD")
	if err != nil {
		if strings.Contains(err.Error(), "unknown revision") {
			g.log.Debug("Remote HEAD not configured", "remoteName", remoteName)
			return "", nil
		}
		return "", fmt.Errorf("failed to get remote HEAD: %w", err)
	}

	branchName := strings.TrimPrefix(output, remoteName+"/")
	return branchName, nil
}

func (g *GitCli) ListLocalBranches() ([]LocalBranch, error) {
	format := `branch %(refname:short)
checkedOut %(if)%(HEAD)%(then)true%(else)false%(end)
commit %(objectname:short)
%(if)%(upstream:short)%(then)upstream %(upstream:short)
%(end)%(if)%(upstream:track)%(then)track %(upstream:track)
%(end)committedOn %(committerdate:iso-strict)
committedBy %(committername)
subject %(contents:subject)
worktreepath %(worktreepath)
`
	output, err := g.executeGitCommand("for-each-ref", "--format="+format, "refs/heads/")
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	if output == "" {
		return []LocalBranch{}, nil
	}

	return parseBranchesFromFormat(output), nil
}

// parseBranchesFromFormat parses the output of `git for-each-ref` with a custom format
// and returns a slice of LocalBranch structs with all metadata.
func parseBranchesFromFormat(output string) []LocalBranch {
	blocks := splitIntoBlocks(output)
	branches := make([]LocalBranch, 0, len(blocks))

	for _, block := range blocks {
		branch := parseBranchBlock(block)
		if branch.Name != "" {
			branches = append(branches, branch)
		}
	}

	return branches
}

// splitIntoBlocks splits porcelain output into blocks separated by blank lines.
// Each block is a slice of non-empty lines.
func splitIntoBlocks(output string) [][]string {
	var blocks [][]string
	var currentBlock []string

	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			currentBlock = append(currentBlock, line)
			continue
		}

		// found blank line, new block
		if len(currentBlock) > 0 {
			blocks = append(blocks, currentBlock)
			currentBlock = nil
		}
	}

	// handle last block if output doesn't end with blank line
	if len(currentBlock) > 0 {
		blocks = append(blocks, currentBlock)
	}

	return blocks
}

func parseBranchBlock(lines []string) LocalBranch {
	var (
		ahead                int
		behind               int
		committedBy          string
		committedOn          time.Time
		isCheckedOut         bool
		name                 string
		sha                  string
		subject              string
		upstreamName         string
		worktreeAbsolutePath string
	)

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "branch "):
			name = strings.TrimPrefix(line, "branch ")
		case strings.HasPrefix(line, "checkedOut "):
			isCheckedOut = strings.TrimPrefix(line, "checkedOut ") == "true"
		case strings.HasPrefix(line, "commit "):
			sha = strings.TrimPrefix(line, "commit ")
		case strings.HasPrefix(line, "upstream "):
			upstreamName = strings.TrimPrefix(line, "upstream ")
		case strings.HasPrefix(line, "track "):
			track := strings.TrimPrefix(line, "track ")
			ahead, behind = parseTrackInfo(track)
		case strings.HasPrefix(line, "committedOn "):
			dateStr := strings.TrimPrefix(line, "committedOn ")
			committedOn = parseISO8601Date(dateStr)
		case strings.HasPrefix(line, "committedBy "):
			committedBy = strings.TrimPrefix(line, "committedBy ")
		case strings.HasPrefix(line, "subject "):
			subject = strings.TrimPrefix(line, "subject ")
		case strings.HasPrefix(line, "worktreepath "):
			worktreeAbsolutePath = strings.TrimPrefix(line, "worktreepath ")
		}
	}

	commit := NewCommit(sha, subject, committedOn, committedBy)
	return NewLocalBranch(name, upstreamName, worktreeAbsolutePath, isCheckedOut, ahead, behind, commit)
}

// parseTrackInfo parses the upstream track info string like "[ahead 3, behind 2]"
func parseTrackInfo(track string) (ahead, behind int) {
	if track == "" || track == "[gone]" {
		return 0, 0
	}
	if n, _ := fmt.Sscanf(track, "[ahead %d, behind %d]", &ahead, &behind); n == 2 {
		return ahead, behind
	}
	if n, _ := fmt.Sscanf(track, "[ahead %d]", &ahead); n == 1 {
		return ahead, 0
	}
	if n, _ := fmt.Sscanf(track, "[behind %d]", &behind); n == 1 {
		return 0, behind
	}
	return 0, 0
}

// getCommitBySHA retrieves full commit information for a given SHA.
// Used when parsing worktrees with detached HEAD.
func (g *GitCli) getCommitBySHA(sha string) (Commit, error) {
	// Format: subject<NUL>committer date ISO<NUL>committer name
	// NUL separators handle multi-line subjects more robustly than line-based parsing
	format := "%s%x00%cI%x00%cn"
	output, err := g.executeGitCommand("log", "-1", "--format="+format, sha)
	if err != nil {
		return Commit{}, fmt.Errorf("failed to get commit info for %s: %w", sha, err)
	}

	parts := strings.Split(output, "\x00")
	if len(parts) != 3 {
		return Commit{}, fmt.Errorf("unexpected commit format for %s: got %d parts", sha, len(parts))
	}

	subject := parts[0]
	committedOn := time.Time{}
	if parsed, err := time.Parse(time.RFC3339, parts[1]); err == nil {
		committedOn = parsed
	} else {
		g.log.Debug("failed to parse commit date", "sha", sha, "date", parts[1], "error", err)
	}
	committedBy := parts[2]

	return NewCommit(sha, subject, committedOn, committedBy), nil
}

func (g *GitCli) ListRemoteBranches(remoteName string) ([]RemoteBranch, error) {
	format := `ref %(refname:short)
commit %(objectname:short)
committedOn %(committerdate:iso-strict)
committedBy %(committername)
subject %(contents:subject)
`
	output, err := g.executeGitCommand("for-each-ref", "--format="+format, "refs/remotes/"+remoteName+"/")
	if err != nil {
		return nil, fmt.Errorf("failed to list remote branches: %w", err)
	}

	if output == "" {
		return []RemoteBranch{}, nil
	}

	return parseRemoteBranchesFromFormat(output), nil
}

func parseRemoteBranchesFromFormat(output string) []RemoteBranch {
	blocks := splitIntoBlocks(output)
	branches := make([]RemoteBranch, 0, len(blocks))

	for _, block := range blocks {
		branch := parseRemoteBranchBlock(block)
		// Skip symbolic HEAD ref (e.g., origin/HEAD -> origin/main)
		if branch.Name != "" && branch.Name != "HEAD" {
			branches = append(branches, branch)
		}
	}

	return branches
}

func parseRemoteBranchBlock(lines []string) RemoteBranch {
	var (
		committedBy string
		committedOn time.Time
		name        string
		remoteName  string
		sha         string
		subject     string
	)

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "ref "):
			// ref is "origin/main", split on first "/" to get remote and branch name
			ref := strings.TrimPrefix(line, "ref ")
			if idx := strings.Index(ref, "/"); idx != -1 {
				remoteName = ref[:idx]
				name = ref[idx+1:]
			}
		case strings.HasPrefix(line, "commit "):
			sha = strings.TrimPrefix(line, "commit ")
		case strings.HasPrefix(line, "committedOn "):
			dateStr := strings.TrimPrefix(line, "committedOn ")
			committedOn = parseISO8601Date(dateStr)
		case strings.HasPrefix(line, "committedBy "):
			committedBy = strings.TrimPrefix(line, "committedBy ")
		case strings.HasPrefix(line, "subject "):
			subject = strings.TrimPrefix(line, "subject ")
		}
	}

	commit := NewCommit(sha, subject, committedOn, committedBy)
	return NewRemoteBranch(name, remoteName, commit)
}

func (g *GitCli) ListRemotes() ([]string, error) {
	output, err := g.executeGitCommand("remote")
	if err != nil {
		return nil, fmt.Errorf("failed to list remotes: %w", err)
	}

	if output == "" {
		return []string{}, nil
	}

	lines := strings.Split(output, "\n")
	remotes := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			remotes = append(remotes, trimmed)
		}
	}
	return remotes, nil
}

func (g *GitCli) SyncTags(remoteName string) error {
	if remoteName == "" {
		var err error
		remoteName, err = g.GetDefaultRemote("origin")
		if err != nil {
			return fmt.Errorf("failed to get default remote: %w", err)
		}
	}

	g.log.Info("Syncing tags from remote", "remote", remoteName)
	args := []string{"fetch", remoteName, "--prune", "--prune-tags", "--tags"}
	return g.executeMutatingCommand("failed to sync tags from remote", args...)
}

func (g *GitCli) ListTags() ([]Tag, error) {
	format := `name %(refname:short)
objecttype %(objecttype)
objectsha %(objectname:short)
derefsha %(*objectname:short)
taggername %(taggername)
taggeremail %(taggeremail)
taggedon %(taggerdate:iso-strict)
message %(contents:subject)
committedby %(*committername)
committedon %(*committerdate:iso-strict)
commitsubject %(*subject)
`
	output, err := g.executeGitCommand("for-each-ref", "--format="+format, "refs/tags/")
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	if output == "" {
		return []Tag{}, nil
	}

	return parseTagsFromFormat(output), nil
}

// parseISO8601Date attempts to parse an ISO 8601 date string and returns zero time on failure.
func parseISO8601Date(dateStr string) time.Time {
	if parsed, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return parsed
	}
	return time.Time{}
}

func parseTagsFromFormat(output string) []Tag {
	blocks := splitIntoBlocks(output)
	tags := make([]Tag, 0, len(blocks))

	for _, block := range blocks {
		tag := parseTagBlock(block)
		if tag.Name != "" {
			tags = append(tags, tag)
		}
	}

	return tags
}

func parseTagBlock(lines []string) Tag {
	var (
		commitSubject string
		committedBy   string
		committedOn   time.Time
		derefSHA      string
		message       string
		name          string
		objectSHA     string
		objectType    string
		taggedOn      time.Time
		taggerEmail   string
		taggerName    string
	)

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "name "):
			name = strings.TrimPrefix(line, "name ")
		case strings.HasPrefix(line, "objecttype "):
			objectType = strings.TrimPrefix(line, "objecttype ")
		case strings.HasPrefix(line, "objectsha "):
			objectSHA = strings.TrimPrefix(line, "objectsha ")
		case strings.HasPrefix(line, "derefsha "):
			derefSHA = strings.TrimPrefix(line, "derefsha ")
		case strings.HasPrefix(line, "taggername "):
			taggerName = strings.TrimPrefix(line, "taggername ")
		case strings.HasPrefix(line, "taggeremail "):
			taggerEmail = strings.TrimPrefix(line, "taggeremail ")
		case strings.HasPrefix(line, "taggedon "):
			taggedOn = parseISO8601Date(strings.TrimPrefix(line, "taggedon "))
		case strings.HasPrefix(line, "message "):
			message = strings.TrimPrefix(line, "message ")
		case strings.HasPrefix(line, "committedby "):
			committedBy = strings.TrimPrefix(line, "committedby ")
		case strings.HasPrefix(line, "committedon "):
			committedOn = parseISO8601Date(strings.TrimPrefix(line, "committedon "))
		case strings.HasPrefix(line, "commitsubject "):
			commitSubject = strings.TrimPrefix(line, "commitsubject ")
		}
	}

	// Determine the actual commit SHA and metadata
	// For lightweight tags (objecttype=commit): objectSHA IS the commit
	// For annotated tags (objecttype=tag): derefSHA is the dereferenced commit SHA
	var actualCommitSHA string
	var actualMessage string
	if objectType == "commit" {
		// Lightweight tag - objectSHA IS the commit
		actualCommitSHA = objectSHA
		// For lightweight tags, commit metadata comes from the commit itself
		// The %(*...) placeholders are empty, so use the tag's message as subject fallback
		if commitSubject == "" {
			commitSubject = message
		}
		// Don't set message for lightweight tags - they don't have tag messages
		actualMessage = ""
	} else {
		// Annotated tag - use dereferenced commit SHA
		actualCommitSHA = derefSHA
		actualMessage = message
	}

	commit := NewCommit(actualCommitSHA, commitSubject, committedOn, committedBy)
	return NewTag(name, commit, actualMessage, taggerName, taggerEmail, taggedOn)
}

func (g *GitCli) BranchExists(branchName string, caseInsensitive bool) (bool, error) {
	branches, err := g.ListLocalBranches()
	if err != nil {
		return false, err
	}

	for _, branch := range branches {
		if caseInsensitive {
			if strings.EqualFold(branch.Name, branchName) {
				return true, nil
			}
		} else {
			if branch.Name == branchName {
				return true, nil
			}
		}
	}

	return false, nil
}

func (g *GitCli) ListWorktrees() ([]Worktree, error) {
	branches, err := g.ListLocalBranches()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches for worktree lookup: %w", err)
	}

	tags, err := g.ListTags()
	if err != nil {
		return nil, fmt.Errorf("failed to list tags for worktree lookup: %w", err)
	}

	branchMap := make(map[string]LocalBranch, len(branches))
	for _, b := range branches {
		branchMap[b.Name] = b
	}

	tagMap := make(map[string]Tag, len(tags))
	for _, t := range tags {
		tagMap[t.Commit().SHA] = t
	}

	output, err := g.executeGitCommand("worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	return g.parseWorktreesFromPorcelain(output, branchMap, tagMap)
}

// parseWorktreesFromPorcelain parses the output of `git worktree list --porcelain`
func (g *GitCli) parseWorktreesFromPorcelain(output string, branchMap map[string]LocalBranch, tagMap map[string]Tag) ([]Worktree, error) {
	blocks := splitIntoBlocks(output)
	worktrees := make([]Worktree, 0, len(blocks))

	for _, block := range blocks {
		worktree, err := g.parseWorktreeBlock(block, branchMap, tagMap)
		if err != nil {
			return nil, err
		}
		if worktree.AbsolutePath != "" {
			worktrees = append(worktrees, worktree)
		}
	}

	return worktrees, nil
}

func (g *GitCli) parseWorktreeBlock(lines []string, branchMap map[string]LocalBranch, tagMap map[string]Tag) (Worktree, error) {
	var (
		absolutePath string
		branchName   string
		detached     bool
		sha          string
	)

	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "worktree "):
			absolutePath = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			sha = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			branchName = strings.TrimPrefix(ref, "refs/heads/")
		case line == "detached":
			detached = true
		case strings.HasPrefix(line, "bare"):
			// Bare worktrees don't have a ref, skip
			return Worktree{AbsolutePath: absolutePath}, nil
		}
	}

	worktree := Worktree{AbsolutePath: absolutePath}

	if branchName != "" {
		branch, ok := branchMap[branchName]
		if !ok {
			return Worktree{}, fmt.Errorf("worktree branch '%s' at sha '%s' not found in local map", branchName, sha)
		}
		worktree.Ref = &branch
	} else if detached {
		if tag, ok := tagMap[sha]; ok {
			worktree.Ref = &tag
		} else {
			// TODO: figure out a way to get the commit information without needing a reference to g GitCli
			commit, err := g.getCommitBySHA(sha)
			if err != nil {
				return Worktree{}, fmt.Errorf("failed to get commit for detached worktree: %w", err)
			}
			worktree.Ref = &commit
		}
	}

	return worktree, nil
}

func (g *GitCli) CreateWorktreeForNewBranch(newBranchName, worktreeAbsPath string) error {
	g.log.Info("Creating worktree for new branch", "branch", newBranchName, "path", worktreeAbsPath)
	args := []string{"worktree", "add", "-b", newBranchName, worktreeAbsPath}
	return g.executeMutatingCommand("failed to create worktree for new branch", args...)
}

func (g *GitCli) CreateWorktreeForNewBranchFromRef(newBranchName, worktreeAbsPath, baseRef string) error {
	g.log.Info("Creating worktree for new branch from ref", "branch", newBranchName, "path", worktreeAbsPath, "baseRef", baseRef)
	args := []string{"worktree", "add", "-b", newBranchName, worktreeAbsPath}
	if baseRef != "" {
		args = append(args, baseRef)
	}
	return g.executeMutatingCommand("failed to create worktree for new branch from ref", args...)
}

func (g *GitCli) CreateWorktreeForExistingBranch(branchName, worktreeAbsPath string) error {
	g.log.Info("Creating worktree for existing branch", "branch", branchName, "path", worktreeAbsPath)
	args := []string{"worktree", "add", worktreeAbsPath, branchName}
	return g.executeMutatingCommand("failed to create worktree for existing branch", args...)
}

func (g *GitCli) FetchRemoteBranch(remote, remoteRef, localRef string) error {
	g.log.Info("Fetching remote branch", "remote", remote, "remoteRef", remoteRef, "localRef", localRef)
	refSpec := remoteRef + ":" + localRef
	args := []string{"fetch", remote, refSpec}
	return g.executeMutatingCommand("failed to fetch remote branch", args...)
}

func (g *GitCli) FetchRemote(remoteName string) (string, error) {
	g.log.Info("Fetching from remote", "remote", remoteName)
	args := []string{"fetch", remoteName, "--prune", "--prune-tags", "--tags"}
	return g.executeMutatingCommandWithOutput("failed to fetch from remote", args...)
}
