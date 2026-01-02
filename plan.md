# Plan: PR Worktree Commands for grove-cli

## Summary
Add `grove pr list`, `grove pr preview`, `grove pr create` commands and `grp` shell function to manage GitHub pull request worktrees.

## User Preferences (Confirmed)
- **Command structure**: Nested (`grove pr list`)
- **Branch naming**: Configurable template with `{{.BranchName}}`, `{{.Number}}`; default `{{.BranchName}}`
- **Duplicate handling**: Return existing worktree path (idempotent)
- **Worktree prefix**: `pr-` (separate from regular `wt-`)
- **Shell function**: `grp`

---

## Implementation Phases

### Phase 1: Config Changes

**Files to modify:**
- `internal/config/config.go` - Add `PRConfig` struct
- `internal/config/defaults.go` - Add defaults

```go
// Add between Git and Slugify (alpha order)
type PRConfig struct {
    BranchTemplate   string `toml:"branch_template"`   // default: "{{.BranchName}}"
    WorktreePrefix   string `toml:"worktree_prefix"`   // default: "pr-"
}
```

**Template Validation** in `Validate()`:
```go
if c.PR.BranchTemplate != "" {
    // 1. Parse template
    tmpl, err := template.New("branch").Parse(c.PR.BranchTemplate)
    if err != nil {
        return fmt.Errorf("pr.branch_template: %w", err)
    }

    // 2. Execute with test data to verify fields
    var buf bytes.Buffer
    testData := struct{ BranchName string; Number int }{"test/branch", 1}
    if err := tmpl.Execute(&buf, testData); err != nil {
        return fmt.Errorf("pr.branch_template uses invalid field: %w", err)
    }

    // 3. Validate output is valid git branch name (pure Go implementation)
    // Rules: no "..", no "~", no "^", no ":", no spaces, no trailing ".", no "@{", etc.
    if !isValidBranchName(buf.String()) {
        return fmt.Errorf("pr.branch_template produces invalid branch name: %s", buf.String())
    }
}

// isValidBranchName validates git branch name rules in pure Go.
// Implements checks for: no "..", no trailing ".", no "~^:?*[\", no "@{", no leading "-", etc.
func isValidBranchName(name string) bool
```

---

### Phase 2: PR Naming Package

**File to create:** `internal/naming/pr.go`

```go
type PRTemplateData struct {
    BranchName string  // PR's head branch (e.g., "feature/add-auth")
    Number     int     // PR number (e.g., 123)
}

type PRWorktreeNamer struct {
    branchTemplate *template.Template
    worktreePrefix string
    slugifyOpts    SlugifyOptions
}

func NewPRWorktreeNamer(prCfg config.PRConfig, slugCfg config.SlugifyConfig) (*PRWorktreeNamer, error)
func (n *PRWorktreeNamer) GenerateBranchName(pr github.PullRequest) (string, error)
func (n *PRWorktreeNamer) GenerateWorktreeName(branchName string) string
func (n *PRWorktreeNamer) HasPrefix(name string) bool
func (n *PRWorktreeNamer) ExtractFromAbsolutePath(absPath string) string
```

Note: Follows same pattern as `WorktreeNamer`. `HasPrefix` and `ExtractFromAbsolutePath`
are needed for display formatting in `pr list`.

**Tests:** `internal/naming/pr_test.go` - Table-driven tests for various templates.

---

### Phase 3: PR Matcher Utility

**File to create:** `internal/pr/matcher.go`

```go
type Matcher struct {
    namer *naming.PRWorktreeNamer
}

type WorktreeMatch struct {
    PR            github.PullRequest
    HasWorktree   bool
    WorktreePath  string
}

func NewMatcher(namer *naming.PRWorktreeNamer) *Matcher

func (m *Matcher) Match(prs []github.PullRequest, worktrees []git.Worktree) []WorktreeMatch

func (m *Matcher) FindWorktreeForPR(pr github.PullRequest, worktrees []git.Worktree) *git.Worktree
```

**Critical**: Matching checks both the expected template-generated branch AND the PR's remote branch name.
This ensures worktrees created via `grove create` (with `wt-` prefix) are also detected:
```go
func (m *Matcher) FindWorktreeForPR(pr github.PullRequest, worktrees []git.Worktree) *git.Worktree {
    // Apply template to get expected local branch name
    expectedBranch, err := m.namer.GenerateBranchName(pr)
    if err != nil {
        expectedBranch = "" // Fall through to remote branch check
    }

    // Search worktrees for matching branch (check both expected and remote branch name)
    for i := range worktrees {
        if branch, ok := worktrees[i].Ref.FullBranch(); ok {
            // Match by expected template-generated name OR by PR's remote branch name
            if branch.Name == expectedBranch || branch.Name == pr.BranchName {
                return &worktrees[i]
            }
        }
    }
    return nil
}
```

This ensures:
1. PR worktrees created via `grove pr create` are detected (matches template-generated name)
2. Worktrees created via `grove create` for the same branch are also detected (matches remote branch name)
3. Matching works regardless of worktree prefix (`pr-` or `wt-`)

**Tests:** `internal/pr/matcher_test.go`

---

### Phase 4: Commands

**Files to create:**

1. `cmd/pr.go` - Parent command
   ```go
   var prCmd = &cobra.Command{
       Use:   "pr",
       Short: "Manage pull request worktrees",
   }
   ```

2. `cmd/pr_list.go` - List PRs with worktree status
   - `--fzf` flag for fzf-compatible output
   - Default: lipgloss table with columns: **#, Title, Author, Branch, State, Local, Updated**
   - "State" column shows "open" or "draft" (distinguishes draft PRs from regular open PRs)
   - "Local" column shows ✓ when worktree exists, empty otherwise

   **Table format:**
   ```
   | #   | Title       | Author | Branch      | State | Local | Updated |
   |-----|-------------|--------|-------------|-------|-------|---------|
   | 123 | Add feature | user   | feature/add | open  | ✓     | 1h ago  |
   | 124 | Fix bug     | other  | fix/bug     | draft |       | 3d ago  |
   ```

   **FZF format:** `<number>\t<searchable>\t<display>`

   Three columns with distinct purposes:
   - **Column 1 (Extract)**: PR number - extracted via `cut -f1` after selection for `pr create`, also passed to preview via `{1}`
   - **Column 2 (Search)**: Searchable content - fzf searches this column, hidden from display
   - **Column 3 (Display)**: Pretty formatted string - shown via `--with-nth 3`

   **Column 2 searchable content:** Space-concatenated: number, title, branch, author, state
   - Example: `123 Add authentication feature/add-auth jsmith open`
   - **Note**: PR body text is intentionally excluded. While the original prompt requested body search,
     the current approach (title, branch, author, state) is deemed sufficient. Body text would require
     sanitization (newlines, tabs break fzf parsing) and significantly increase line length.
   - Includes state for filtering (e.g., type "draft" to find draft PRs)

   **Column 3 display:** `✓ #123 Add authentication [jsmith] feature/add-auth`

   **Example output line:**
   ```
   123\t123 Add auth feature/add-auth jsmith open\t✓ #123 Add auth [jsmith] feature/add-auth
   ```

3. `cmd/pr_preview.go` - Show PR details
   - Takes PR number as argument
   - Single API fetch, displays all info at once (~200-500ms latency is acceptable)
   - **Note**: The original prompt suggested progressive loading (show instant data, then fetch more).
     Single API fetch is simpler and the latency is acceptable for preview use. This is a deliberate
     simplification.
   - Designed for fzf `--preview` usage

   ```go
   func runPreview(cmd *cobra.Command, args []string) error {
       prNum := parseNumber(args[0])

       pr, err := gh.GetPullRequest(prNum)
       if err != nil { return err }

       files, err := gh.GetPullRequestFiles(prNum)
       if err != nil { return err }

       fmt.Printf("PR #%d\n", pr.Number)
       fmt.Printf("─────────────────────────────\n")
       fmt.Printf("Title:  %s\n", pr.Title)
       fmt.Printf("Author: %s\n", pr.AuthorLogin)
       fmt.Printf("Branch: %s\n", pr.BranchName)
       fmt.Printf("State:  %s\n", pr.State)
       fmt.Printf("\n")

       // Show file list with +/- counts
       fmt.Printf("Files changed (%d):\n", len(files))
       for _, f := range files {
           fmt.Printf("  %s (+%d, -%d)\n", f.Path, f.Additions, f.Deletions)
       }

       fmt.Printf("\n%s\n", pr.Body)
       return nil
   }
   ```

   **Note**: Requires adding `GetPullRequestFiles(num)` to `internal/github/` package.
   Returns list of `{Path, Additions, Deletions}` structs.

4. `cmd/pr_create.go` - Create worktree from PR
   - Takes PR number
   - Idempotent: returns existing path if worktree exists (user runs `git pull` to update)
   - **Fork limitation documented:**
   ```go
   Long: `Create a local worktree from a GitHub pull request.

   Note: Only works with PRs from the same repository. Fork PRs are not yet supported.`,
   ```

   **Flow:**
   1. Fetch PR info via `gh.GetPullRequest(num)`
   2. Generate local branch name via template
   3. Check if worktree already exists → return path
   4. Fetch remote branch: `git.FetchRemoteBranch("origin", pr.BranchName, localBranch)`
      - On failure, detect fork PRs and show specific error
   5. Create worktree: `git.CreateWorktreeForExistingBranch(localBranch, wtPath)`
   6. Output absolute path

   **Fork detection** (using HeadRepoOwner field):
   ```go
   // Detect fork PRs before attempting fetch (cleaner than catching git errors)
   if pr.HeadRepoOwner != "" && pr.HeadRepoOwner != baseRepoOwner {
       return fmt.Errorf("PR #%d is from a fork (%s), which is not yet supported", pr.Number, pr.HeadRepoOwner)
   }
   ```

   **Note**: Requires adding `HeadRepoOwner` field to `PullRequest` struct in `internal/github/pull_request.go`.
   Fetch from GitHub API field `headRepositoryOwner.login`.

**Tests:** `cmd/pr_list_test.go`, `cmd/pr_preview_test.go`, `cmd/pr_create_test.go`

---

### Phase 5: Shell Scripts

**Files to create:**
- `internal/shell/scripts/grp.bash`
- `internal/shell/scripts/grp.zsh`
- `internal/shell/scripts/grp.fish`

Pattern (bash/zsh):
```bash
# FZF column layout: <number>\t<searchable>\t<display>
#   --with-nth 3   → show column 3 (pretty display)
#   {1}            → PR number for pr create and preview
#   cut -f1        → extract PR number after selection
grp() {
    local pr_num
    pr_num=$(grove pr list --fzf | fzf --delimiter '\t' --with-nth 3 --preview 'grove pr preview {1}' | cut -f1)
    if [ -n "$pr_num" ]; then
        local output
        output=$(grove pr create "$pr_num")
        if [ $? -eq 0 ]; then
            # Prefer zoxide (z) when available for better directory history
            if command -v z &> /dev/null; then
                z "$output"
            else
                cd "$output"
            fi
        else
            echo "$output"; return 1
        fi
    fi
}
```

Fish script follows existing `grs.fish` patterns (uses `set -l`, `test`, `function...end`).

**File to modify:** `internal/shell/functions.go`
- Add `//go:embed` directives for grp scripts
- Update `GenerateFish()`, `GenerateZsh()`, `GenerateBash()` to include grp

---

## PR Number to Worktree Path Transformation

Complete end-to-end flow showing how a PR number becomes a worktree path:

**Example Configuration:**
```toml
[pr]
branch_template = "{{.BranchName}}"
worktree_prefix = "pr-"

[slugify]
lowercase = true
replace_non_alphanum = true
collapse_dashes = true
```

**Transformation Steps:**

```
Input: PR #123
  └─ BranchName: "feature/add-auth"
  └─ Workspace: /Users/me/code/myrepo

Step 1: Apply branch_template
  Template: "{{.BranchName}}"
  Data: {BranchName: "feature/add-auth", Number: 123}
  Result: "feature/add-auth"

Step 2: Apply slugify to branch name
  Input: "feature/add-auth"
  Operations: lowercase, replace / with -, collapse dashes
  Result: "feature-add-auth"

Step 3: Apply worktree_prefix
  Input: "feature-add-auth"
  Prefix: "pr-"
  Result: "pr-feature-add-auth"

Step 4: Join with workspace path
  Workspace: /Users/me/code/myrepo
  Directory: "pr-feature-add-auth"
  Result: /Users/me/code/myrepo/pr-feature-add-auth

Output: /Users/me/code/myrepo/pr-feature-add-auth
```

**Alternative Template Example:**

```
Input: PR #123 with branch_template = "pr/{{.Number}}"

Step 1: Apply template → "pr/123"
Step 2: Slugify → "pr-123"
Step 3: Smart prefix check: "pr-123" starts with "pr-" → skip prefix
Step 4: Join → /Users/me/code/myrepo/pr-123
```

---

## Critical Files

| File | Action |
|------|--------|
| `internal/config/config.go` | Add PRConfig struct, add `isValidBranchName()` |
| `internal/config/defaults.go` | Add PR defaults |
| `internal/naming/pr.go` | NEW: PR naming logic |
| `internal/pr/matcher.go` | NEW: PR-worktree matcher |
| `internal/github/github.go` | Add `GetPullRequestFiles(num)` to interface |
| `internal/github/github_cli.go` | Implement `GetPullRequestFiles` using `gh api` |
| `internal/github/pull_request.go` | Add `HeadRepoOwner` field for fork detection |
| `cmd/pr.go` | NEW: Parent command |
| `cmd/pr_list.go` | NEW: List command |
| `cmd/pr_preview.go` | NEW: Preview command |
| `cmd/pr_create.go` | NEW: Create command |
| `internal/shell/scripts/grp.{bash,zsh,fish}` | NEW: Shell functions |
| `internal/shell/functions.go` | Add grp embeds |

---

## Key Design Decisions

1. **Template-based branch naming**: Flexible, users can use `{{.BranchName}}` or `pr/{{.Number}}`
2. **Separate `pr-` prefix**: Distinguishes PR worktrees from regular `wt-` worktrees
3. **Idempotent create**: Safe to run multiple times, just returns existing path. User runs `git pull` manually to update stale worktrees.
4. **Match by branch name (not prefix)**: For each PR, check worktrees for matching branch name - both the template-generated name AND the PR's remote branch name. This detects worktrees regardless of prefix (`pr-` or `wt-`), so worktrees created via `grove create` are also recognized.
5. **Lipgloss tables**: Beautiful default output, degraded to 3-column tab-separated for fzf
6. **3-column fzf format**: `<number>\t<searchable>\t<display>` - intentional divergence from `grove list`'s 2-column format. PRs need separate searchable content (title, author, state) that differs from display. This is the right design for the use case.
7. **Preview fetches via API**: Each preview makes an API call (~200-500ms). Simpler than caching, no shell escaping issues.
8. **File list in preview**: Shows actual file paths with +/- line counts, not just a total count
9. **Minimal error handling**: Wrap errors with context, let them propagate naturally

---

## Execution Order

1. Config changes (PRConfig)
2. Naming package (PRWorktreeNamer)
3. Matcher utility
4. Parent `pr` command
5. `pr list` command
6. `pr preview` command
7. `pr create` command
8. Shell scripts (`grp`)
9. Run `make check` to validate

---

## Known Limitations (v1)

1. **Fork PRs not supported**: Only works with PRs from the same repository. PRs from forks require fetching from the forker's remote, which is not implemented. Fork PRs are detected via `HeadRepoOwner` field and a specific error message is shown.

2. **Config changes may break matching**: If you change `branch_template` after creating PR worktrees, previously created worktrees may not be detected as "Local" in `pr list`. The matcher regenerates expected branch names using the current config. Worktrees created with a different template will only be detected if they match the PR's remote branch name.

3. **State flag deferred**: `pr list` only shows open PRs. Flags like `--state closed` or `--all` are deferred to a future version.

4. **No JSON output**: `--json` flag for scriptability is deferred.

---

## Design Decision: Smart Prefix Detection

### Problem

When using a branch template that includes a prefix pattern (e.g., `branch_template = "pr/{{.Number}}"`), the slugify + prefix flow produces redundant prefixes:

```
branch_template = "pr/{{.Number}}"   worktree_prefix = "pr-"

"pr/123" → slugify → "pr-123" → add prefix → "pr-pr-123"  ← PROBLEM
```

### Solution: Smart Detection

The `PRWorktreeNamer.GenerateWorktreeName` method will check if the slugified branch name already starts with the `worktree_prefix`. If so, skip adding the prefix.

**Implementation in `internal/naming/pr.go`:**
```go
func (n *PRWorktreeNamer) GenerateWorktreeName(branchName string) string {
    slug := Slugify(branchName, n.slugifyOpts)
    if slug == "" {
        return ""
    }

    // Smart detection: skip prefix if slug already starts with it
    if strings.HasPrefix(slug, n.worktreePrefix) {
        return slug
    }

    return n.worktreePrefix + slug
}
```

**Examples:**

| Branch Template | Slugified | Prefix | Result |
|-----------------|-----------|--------|--------|
| `pr/{{.Number}}` → `pr/123` | `pr-123` | `pr-` | `pr-123` (prefix skipped) |
| `{{.BranchName}}` → `feature/add-auth` | `feature-add-auth` | `pr-` | `pr-feature-add-auth` |
| `{{.BranchName}}` → `pr-fix/bug` | `pr-fix-bug` | `pr-` | `pr-fix-bug` (prefix skipped) |

### Known Edge Case

Branches that happen to start with the prefix pattern (e.g., `pr-fix/bug`) will not get the prefix added. This is considered acceptable behavior:
- Such branch names are rare in practice
- If a branch is already named `pr-*`, the user likely doesn't want `pr-pr-*`
- The behavior is consistent and predictable once understood

### Updated Transformation Flow

```
Input: PR #123 with branch_template = "pr/{{.Number}}"

Step 1: Apply template → "pr/123"
Step 2: Slugify → "pr-123"
Step 3: Smart prefix check: "pr-123" starts with "pr-" → skip prefix
Step 4: Result: "pr-123"

Output: /Users/me/code/myrepo/pr-123 ✓
```
