# Implementation Plan: Grove CLI - Top-Down Feature-First / Vertical Slice

## Overview

This plan prioritizes getting a working end-to-end `create` command as fast as possible, then iteratively layering in config loading and shell function generation. The approach favors working software over perfect architecture, allowing patterns to emerge naturally.

**Target Command:**
```bash
grc the name of a feature im working on
```

---

## Current State Analysis

### Existing Infrastructure

**`internal/git` package** - Comprehensive Git interface with:
- `Git` interface defining all operations
- `GitCli` implementation with dry-run support
- Key methods for our use case:
  - `GetMainWorktreePath()` - returns the main worktree path
  - `GetWorktreeRoot()` - returns current worktree root
  - `BranchExists(branchName, caseInsensitive)` - checks for existing branches
  - `CreateWorktreeForNewBranch(newBranchName, worktreeAbsPath)` - creates branch + worktree atomically
  - `CreateWorktreeForNewBranchFromRef(newBranchName, worktreeAbsPath, baseRef)` - same but from specific ref

**`internal/naming` package** - Slugify functionality:
- `SlugifyOptions` struct with fields matching config spec exactly
- `Slugify(input, opts)` function ready to use

**`main.go`** - Empty entry point ready for Cobra setup

**Dependencies to add**:
- `github.com/spf13/cobra` - CLI framework
- `github.com/BurntSushi/toml` - TOML config parsing

---

## Iteration 1: Minimal Working Create Command

### Goal
Get `grove create "my feature name"` to create a branch and worktree with hardcoded defaults, outputting the path.

### Definition of Done
- Running `grove create "my feature name"` creates:
  - Branch: `feature/my-feature-name` (hardcoded prefix)
  - Worktree: `../wt-my-feature-name` (sibling to main worktree)
- Outputs the absolute path to the new worktree
- `make check` passes

### Files to Create/Modify

1. **`cmd/root.go`** (new)
   - Initialize Cobra root command
   - Set up basic app structure

2. **`cmd/create.go`** (new)
   - `create` subcommand taking phrase argument
   - Hardcoded defaults for slugify, branch prefix, worktree prefix
   - Core flow:
     ```go
     func runCreate(cmd *cobra.Command, args []string) error {
         phrase := strings.Join(args, " ")

         // 1. Slugify the phrase (hardcoded options)
         slug := naming.Slugify(phrase, naming.SlugifyOptions{
             MaxLength:          20,
             HashLength:         4,
             Lowercase:          true,
             ReplaceNonAlphaNum: true,
             CollapseDashes:     true,
             TrimDashes:         true,
         })

         // 2. Generate branch name
         branchName := "feature/" + slug

         // 3. Generate worktree path
         mainWorktree, _ := gitClient.GetMainWorktreePath()
         worktreePath := filepath.Join(filepath.Dir(mainWorktree), "wt-"+slug)

         // 4. Create branch + worktree
         err := gitClient.CreateWorktreeForNewBranch(branchName, worktreePath)

         // 5. Output the path
         fmt.Println(worktreePath)
         return err
     }
     ```

3. **`main.go`** (modify)
   - Call into cmd package

4. **`go.mod`** (modify)
   - Add `github.com/spf13/cobra`

### Implementation Steps

1. Add Cobra dependency: `go get github.com/spf13/cobra`
2. Create `cmd/root.go` with basic Cobra structure
3. Create `cmd/create.go` with hardcoded logic
4. Update `main.go` to execute root command
5. Run `make check` to validate

---

## Iteration 2: Extract Config Types and Loading

### Goal
Replace hardcoded defaults with config loaded from files, using hierarchical loading.

### Definition of Done
- Config file (`grove.toml`) is loaded from hierarchical locations
- All hardcoded values are driven by config
- Default values work when no config file exists
- `make check` passes

### Files to Create/Modify

1. **`internal/config/config.go`** (new)
   - Config struct matching spec:
     ```go
     type Config struct {
         Branch   BranchConfig   `toml:"branch"`
         Slugify  SlugifyConfig  `toml:"slugify"`
         Worktree WorktreeConfig `toml:"worktree"`
     }

     type BranchConfig struct {
         NewPrefix string `toml:"new_prefix"`
     }

     type SlugifyConfig struct {
         CollapseDashes     bool `toml:"collapse_dashes"`
         HashLength         int  `toml:"hash_length"`
         Lowercase          bool `toml:"lowercase"`
         MaxLength          int  `toml:"max_length"`
         ReplaceNonAlphanum bool `toml:"replace_non_alphanum"`
         TrimDashes         bool `toml:"trim_dashes"`
     }

     type WorktreeConfig struct {
         NewPrefix         string   `toml:"new_prefix"`
         StripBranchPrefix []string `toml:"strip_branch_prefix"`
     }
     ```
   - `DefaultConfig()` function returning sensible defaults
   - Note: struct fields sorted alphabetically per CLAUDE.md

2. **`internal/config/loader.go`** (new)
   - `Load()` function implementing hierarchical loading:
     ```go
     func Load() (*Config, error) {
         cfg := DefaultConfig()

         // Load in order (lowest to highest priority):
         // 1. XDG config dir: ~/.config/grove/grove.toml
         // 2. Dirs up to home/root
         // 3. Git root tree dir
         // 4. Current working directory

         paths := collectConfigPaths()
         for _, path := range paths {
             if exists(path) {
                 merge(cfg, loadFile(path))
             }
         }
         return cfg, nil
     }
     ```
   - Helper to find git root (using existing git package)
   - Helper to walk directories up to home/root

3. **`internal/config/config_test.go`** (new)
   - Tests for default config
   - Tests for loading single file
   - Tests for hierarchical merging

4. **`internal/config/loader_test.go`** (new)
   - Tests for path collection logic
   - Tests for merge behavior

5. **`cmd/create.go`** (modify)
   - Load config at command start
   - Use config values instead of hardcoded defaults

6. **`go.mod`** (modify)
   - Add `github.com/BurntSushi/toml`

### Implementation Steps

1. Add TOML dependency: `go get github.com/BurntSushi/toml`
2. Create config types with defaults
3. Implement loader with hierarchical path collection
4. Add tests for config loading
5. Update create command to use config
6. Run `make check` to validate

---

## Iteration 3: Shell Function Generation

### Goal
Add `grove shell <shell-type>` command that outputs shell functions for sourcing.

### Definition of Done
- `grove shell fish`, `grove shell zsh`, `grove shell bash` output appropriate functions
- Functions use zoxide (`z`) if available, fallback to `cd`
- Naming pattern is extensible for future commands
- `make check` passes

### Files to Create/Modify

1. **`cmd/shell.go`** (new)
   - `shell` subcommand with shell type argument
   - Templates for each shell:
     ```go
     const fishTemplate = `
     function __grove_create
         set -l path (grove create $argv)
         if test $status -eq 0
             if type -q z
                 z $path
             else
                 cd $path
             end
         end
     end

     # Alias for convenience
     alias grc='__grove_create'
     `
     ```
   - Similar templates for zsh and bash
   - Naming convention: `__grove_<command>` for internal, `grc` for user alias

2. **`cmd/shell_test.go`** (new)
   - Tests that each shell template is valid syntax
   - Tests that output contains expected function names

### Implementation Steps

1. Create shell command with subcommand for each shell type
2. Write templates for fish, zsh, bash
3. Add tests for template generation
4. Run `make check` to validate

---

## Iteration 4: Edge Cases and Polish

### Goal
Handle edge cases, improve error messages, add helpful output.

### Definition of Done
- Graceful handling of: existing branch, existing worktree path, not in git repo
- Helpful error messages with suggestions
- Dry-run mode (`--dry-run` flag)
- Verbose mode for debugging (`--verbose` flag)
- `make check` passes

### Files to Create/Modify

1. **`cmd/create.go`** (modify)
   - Add validation before creating:
     - Check if branch already exists (use `BranchExists`)
     - Check if worktree path already exists
     - Verify we're in a git repository
   - Add `--dry-run` flag (use existing dryRun support in git package)
   - Add `--base-ref` flag to start from specific ref (use `CreateWorktreeForNewBranchFromRef`)
   - Improve error messages with actionable suggestions

2. **`cmd/root.go`** (modify)
   - Add persistent `--verbose` flag
   - Configure logger based on verbosity

3. **`cmd/create_test.go`** (new)
   - Integration tests using testRepo pattern from git package
   - Test error cases

### Implementation Steps

1. Add validation checks with clear error messages
2. Add dry-run and base-ref flags
3. Add verbose flag to root command
4. Write integration tests
5. Run `make check` to validate

---

## Iteration 5: Worktree Naming Refinement

### Goal
Implement the `strip_branch_prefix` config option for worktree naming.

### Definition of Done
- Worktree name correctly strips configured prefixes from branch name
- Example: branch `feature/my-thing` with `strip_branch_prefix = ["feature/"]` becomes worktree `wt-my-thing`
- `make check` passes

### Files to Create/Modify

1. **`internal/naming/worktree.go`** (new)
   - `WorktreeNameFromBranch(branchName string, cfg config.WorktreeConfig) string`
   - Logic to strip prefixes and apply worktree prefix:
     ```go
     func WorktreeNameFromBranch(branchName string, cfg WorktreeConfig) string {
         name := branchName
         for _, prefix := range cfg.StripBranchPrefix {
             name = strings.TrimPrefix(name, prefix)
         }
         return cfg.NewPrefix + name
     }
     ```

2. **`internal/naming/worktree_test.go`** (new)
   - Tests for prefix stripping
   - Tests for multiple prefixes (first match wins)

3. **`cmd/create.go`** (modify)
   - Use new worktree naming function

### Implementation Steps

1. Create worktree naming function
2. Add tests
3. Update create command to use it
4. Run `make check` to validate

---

## Summary: File Structure After All Iterations

```
grove-cli/
├── cmd/
│   ├── create.go          # create command implementation
│   ├── create_test.go     # create command tests
│   ├── root.go            # root command setup
│   ├── shell.go           # shell function generation
│   └── shell_test.go      # shell tests
├── internal/
│   ├── config/
│   │   ├── config.go      # config types and defaults
│   │   ├── config_test.go # config tests
│   │   ├── loader.go      # hierarchical config loading
│   │   └── loader_test.go # loader tests
│   ├── git/
│   │   └── (existing)     # unchanged
│   └── naming/
│       ├── slugify.go     # existing
│       ├── slugify_test.go# existing
│       ├── worktree.go    # worktree name generation
│       └── worktree_test.go
├── go.mod                 # add cobra, toml
├── go.sum
├── main.go                # calls cmd.Execute()
└── Makefile               # unchanged
```

---

## Testing Strategy

Following patterns established in `internal/git`:
- Unit tests for pure functions (config parsing, naming logic)
- Table-driven tests with `testing.T` and `testify/assert`
- Integration tests using temp directories for git operations
- Test helpers for common setup (similar to `testRepo` in git package)

---

## Pros and Cons

### Pros of This Approach

1. **Fast feedback loop** - Working software in Iteration 1 validates the core concept immediately
2. **Reduces risk** - Discovers integration issues with git package early
3. **Maintains momentum** - Each iteration delivers visible progress
4. **Avoids over-engineering** - Config system is designed after we understand actual needs
5. **Easy to pivot** - If requirements change, minimal code to throw away
6. **Testable at every stage** - Each iteration has clear "done" criteria

### Cons of This Approach

1. **Config refactoring** - Iteration 2 requires touching create.go again
2. **Less clean initial code** - Hardcoded values will need extraction
3. **Potential for tech debt** - Quick iterations may leave rough edges
4. **Harder to parallelize** - Sequential dependencies between iterations
5. **Shell functions may need updates** - If create command output format changes

### When to Prefer This Approach

- **Preferred when:**
  - Requirements are not fully crystallized
  - Quick validation is valuable
  - Solo developer or small team
  - Greenfield project with low switching costs
  - Time-to-first-demo matters

- **Consider alternatives when:**
  - Requirements are well-defined upfront
  - Multiple developers need to work in parallel
  - Config system is complex and central
  - API stability is critical from the start
