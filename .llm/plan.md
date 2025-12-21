# Implementation Plan: Grove CLI - Layered Architecture with Practical Milestones

> **Important Notes:**
> - **Best Practices:** Read `.llm/best-practices.md` before implementing each phase for Go and Cobra conventions.
> - **Code Blocks:** All code blocks in this document are **illustrative examples**, not hard definitions. The actual implementation may vary based on context and should follow the intent described in prose.

## Overview

This plan implements the grove-cli `create` command using a layered architecture that enables independent testability while maintaining practical delivery milestones. Each phase produces working, testable code that can be validated before proceeding.

**Target Command:**
```bash
grove create "the name of a feature im working on"
```

This will:
1. Create a git branch with configurable naming convention
2. Create a worktree (using the newly created branch) with configurable naming convention
3. Output a path for shell integration to cd into (using zoxide if available)

**Terminology:**
- **Main Worktree**: The original git repository directory (e.g., `/path/to/project`)
- **Workspace Root**: The parent directory of the main worktree (e.g., `/path/to`). All additional worktrees are created as siblings to the main worktree within this directory.
- **Worktree**: An additional working directory linked to the same git repository

Example directory structure:
```
/path/to/                    <- Workspace Root
├── project/                 <- Main Worktree
│   └── .git/
├── wt-add-user-auth/        <- Additional Worktree
└── wt-fix-login-bug/        <- Additional Worktree
```

### Path Resolution

This section clarifies how key paths are derived and how errors are handled.

**Deriving Workspace Root:**
```go
// Get the main worktree path using the existing Git interface
mainWorktreePath, err := git.GetMainWorktreePath()
if err != nil {
    return fmt.Errorf("failed to get main worktree path: %w", err)
}

// Workspace root is the parent directory of the main worktree
workspaceRoot := filepath.Dir(mainWorktreePath)
```

**Handling "Not in a Git Repository":**

The `GetWorktreeRoot()` method returns `("", nil)` when not in a git repository. The create command must check this early and fail with a clear error:

```go
func runCreate(cmd *cobra.Command, args []string) error {
    // Check we're in a git repository FIRST (before config loading)
    worktreeRoot, err := git.GetWorktreeRoot()
    if err != nil {
        return fmt.Errorf("git error: %w", err)
    }
    if worktreeRoot == "" {
        return errors.New("grove must be run inside a git repository")
    }

    // Now safe to proceed with config loading and worktree creation...
}
```

**Order of Path Resolution:**
1. Validate we're in a git repo (`GetWorktreeRoot() != ""`)
2. Get current worktree root for config discovery (`GetWorktreeRoot()`)
3. Get main worktree path for workspace root derivation (`GetMainWorktreePath()`)
4. Derive workspace root as `filepath.Dir(mainWorktreePath)`
5. Compute new worktree path as `filepath.Join(workspaceRoot, worktreeName)`

**Development Workflow:**
- Commit code after completing each major step within a phase
- Commit all phase changes before moving to the next phase
- Use descriptive commit messages that reference the phase and step

---

## Architecture Diagram

```
+----------------------------------------------------------+
|                    Shell Integration                      |
|              (fish, zsh, bash functions)                  |
+----------------------------------------------------------+
                            |
                            v
+----------------------------------------------------------+
|                    Command Layer                          |
|              (Cobra commands: create, init)               |
+----------------------------------------------------------+
                            |
                            v
+----------------------------------------------------------+
|                   Naming/Generation Layer                 |
|        (branch name generator, worktree name generator)   |
+----------------------------------------------------------+
                            |
                            v
+----------------------------------------------------------+
|                   Configuration Layer                     |
|         (loading, merging, defaults, validation)          |
+----------------------------------------------------------+
                            |
                            v
+----------------------------------------------------------+
|                   Existing Foundation                     |
|     (internal/git, internal/naming/slugify)               |
+----------------------------------------------------------+
```

---

## Phase 1: Configuration Layer

### 1.1 Config Types Definition

**File:** `internal/config/config.go`

Define the configuration structure matching the TOML spec:

```go
package config

// Config represents the complete grove configuration
type Config struct {
    Branch   BranchConfig
    Git      GitConfig
    Slugify  SlugifyConfig
    Worktree WorktreeConfig
}

// BranchConfig configures branch naming
type BranchConfig struct {
    NewPrefix string `toml:"new_prefix"` // e.g., "feature/"
}

// GitConfig configures git command execution
type GitConfig struct {
    Timeout time.Duration `toml:"timeout"` // Timeout for git commands (e.g., "5s")
}

// SlugifyConfig configures slug generation (matches existing SlugifyOptions)
type SlugifyConfig struct {
    CollapseDashes     bool `toml:"collapse_dashes"`
    HashLength         int  `toml:"hash_length"`
    Lowercase          bool `toml:"lowercase"`
    MaxLength          int  `toml:"max_length"`
    ReplaceNonAlphanum bool `toml:"replace_non_alphanum"`
    TrimDashes         bool `toml:"trim_dashes"`
}

// WorktreeConfig configures worktree naming
type WorktreeConfig struct {
    NewPrefix         string   `toml:"new_prefix"`         // e.g., "wt-"
    StripBranchPrefix []string `toml:"strip_branch_prefix"` // e.g., ["feature/"]
    // Note: Only the first matching prefix is stripped (checked in list order)
    // e.g., branch "feature/add-auth" with ["fix/", "feature/"] -> "add-auth"
}

// Validate checks that all config values are valid
// Returns an error describing the first invalid value found
func (c Config) Validate() error {
    if c.Slugify.MaxLength < 0 {
        return errors.New("slugify.max_length cannot be negative")
    }
    if c.Slugify.HashLength < 0 {
        return errors.New("slugify.hash_length cannot be negative")
    }
    if c.Git.Timeout < 0 {
        return errors.New("git.timeout cannot be negative")
    }
    return nil
}
```

Note: Struct fields are sorted alphabetically per CLAUDE.md coding preferences.

### 1.2 Config Defaults

**File:** `internal/config/defaults.go`

```go
package config

// DefaultConfig returns sensible defaults for all configuration
func DefaultConfig() Config {
    return Config{
        Branch: BranchConfig{
            NewPrefix: "feature/",
        },
        Git: GitConfig{
            Timeout: 5 * time.Second,
        },
        Slugify: SlugifyConfig{
            CollapseDashes:     true,
            HashLength:         4,
            Lowercase:          true,
            MaxLength:          50,
            ReplaceNonAlphanum: true,
            TrimDashes:         true,
        },
        Worktree: WorktreeConfig{
            NewPrefix:         "wt-",
            StripBranchPrefix: []string{"feature/"},
        },
    }
}
```

**Note on Git Timeout:**
The git timeout is used when creating the git CLI executor. If a git command takes longer than the timeout, it will be cancelled. This prevents grove from hanging indefinitely on slow or unresponsive git operations.

### 1.3 Config File Discovery

**File:** `internal/config/discovery.go`

Implement hierarchical config file discovery:

```go
package config

// ConfigPaths returns ordered list of config file paths to check (highest to lowest priority)
// Order:
// 1. File in current working directory
// 2. File in current worktree root (if in a worktree)
// 3. File in git repository root (main worktree)
// 4. Files in directories up to home/root
// 5. File in XDG config directory
//
// The worktreeRoot and gitRoot may be the same directory if running from the main worktree.
func ConfigPaths(cwd, worktreeRoot, gitRoot, homeDir string) []string
```

Key implementation details:
- Use `os.UserConfigDir()` for XDG config directory (Go 1.13+)
- Config filename: `grove.toml`
- XDG path: `$XDG_CONFIG_HOME/grove/grove.toml` or `~/.config/grove/grove.toml`

### 1.4 Config Loading and Merging

**File:** `internal/config/loader.go`

```go
package config

import "github.com/BurntSushi/toml"

// LoadResult contains the loaded config and metadata about the load
type LoadResult struct {
    Config      Config
    SourcePaths []string // paths that were successfully loaded, in order applied
}

// Loader handles configuration loading and merging
type Loader struct {
    fs FileSystem // interface for testability
}

// FileSystem abstracts file system operations for testability
type FileSystem interface {
    // Exists returns true if the path exists and is a file (not a directory)
    Exists(path string) bool
}

// OSFileSystem implements FileSystem using the real OS
type OSFileSystem struct{}

func (OSFileSystem) Exists(path string) bool {
    info, err := os.Stat(path)
    if err != nil {
        return false
    }
    return !info.IsDir()
}

// Load reads and merges all config files in priority order
// Returns merged config with defaults as base, plus source paths for debugging
func (l *Loader) Load(paths []string) (LoadResult, error)
```

**Why use an interface:**
- Enables testing file existence checks without real files
- The `Exists` method can be mocked to simulate missing files, permission errors, etc.

**Note on TOML Loading:**
Since we use `toml.DecodeFile` directly (for simplicity), config loading tests that verify TOML parsing will need temporary files. Use `t.TempDir()` for these tests. The `FileSystem` interface is primarily for controlling which files are "seen" during discovery.

Note: `LoadResult.SourcePaths` aids debugging by showing which config files were applied.

#### Simplified Loading Approach Using Sequential Decoding

The BurntSushi TOML library provides a natural merging mechanism through its decoding behavior that eliminates the need for an explicit `merge()` function.

**How TOML Decoding Overlays Values:**

When you decode a TOML file into a pre-populated struct using `toml.DecodeFile()`, the decoder overwrites only the fields that are **present in the TOML file**. Fields not mentioned in the TOML file retain their existing values.

This behavior enables a simple sequential loading pattern:

1. Start with `DefaultConfig()` to initialize the struct with defaults
2. Decode the lowest priority config file (e.g., XDG config) into the struct
   - Overwrites defaults where values are specified in the file
3. Decode the next priority file into the same struct
   - Overwrites previous values where specified in this file
4. Continue until the highest priority file (e.g., CWD config) is decoded

**Comparison: Sequential Decoding vs Explicit Merge**

There is a subtle but important difference between these approaches:

- **Sequential TOML Decoding**: Overwrites fields that are *present in the TOML file*, even if they have zero values (e.g., `max_length = 0` will overwrite a previous `max_length = 50`)
- **Explicit Merge Function**: Could be designed to only overwrite *non-zero values*, treating zero values as "not set"

**Recommendation**: Use the simpler sequential decoding approach. It provides clear, predictable behavior: "what's in the config file is what you get." Users who want to reset a value to zero can do so explicitly by setting it in a higher-priority config file.

**Implementation Pattern:**

```go
func (l *Loader) Load(paths []string) (LoadResult, error) {
    cfg := DefaultConfig() // Start with defaults
    var sourcePaths []string

    // Decode each config file sequentially (lowest to highest priority)
    for _, path := range paths {
        if !l.fs.Exists(path) {
            continue // Skip missing files
        }

        metadata, err := toml.DecodeFile(path, &cfg)
        if err != nil {
            return LoadResult{}, fmt.Errorf("failed to parse %s: %w", path, err)
        }

        // Warn about unknown keys
        if undecoded := metadata.Undecoded(); len(undecoded) > 0 {
            // Log warning about unknown config keys
        }

        sourcePaths = append(sourcePaths, path)
    }

    // Validate the merged config
    if err := cfg.Validate(); err != nil {
        return LoadResult{}, fmt.Errorf("invalid config: %w", err)
    }

    return LoadResult{
        Config:      cfg,
        SourcePaths: sourcePaths,
    }, nil
}
```

**Validation and Error Detection:**

- Use `metadata.Undecoded()` to detect unknown/extra fields for strict validation
- This allows warning users about typos or deprecated config keys
- Consider using `metadata.IsDefined()` to check if specific keys were present in the TOML file

**Struct Field Tags:**

- Use the format `toml:"field_name"` for struct field tags
- Follow TOML naming conventions (snake_case) in the tags
```

**Config Loading Edge Cases to Handle:**

1. **File doesn't exist**: Skip gracefully, continue to next file in priority order
2. **File exists but isn't readable** (permission denied): Return error with clear message including path
3. **File is empty**: Treat as valid (no values to overlay), continue
4. **Invalid TOML syntax**: Return error with file path and line number from parser
5. **Invalid config values**:
   - Negative MaxLength in SlugifyConfig
   - HashLength < 0
   - Validate at load time with descriptive errors
6. **Config path is a directory**: Skip or return error (recommend skip)
7. **Unknown/extra fields**: Warn user (using metadata.Undecoded()) but don't error
8. **Home directory not available**: Handle `os.UserHomeDir()` errors gracefully for XDG path

### 1.5 Config Testing Strategy

**File:** `internal/config/config_test.go`

Tests following existing patterns from `slugify_test.go`:

- `TestDefaultConfig` - verify defaults are sensible
- `TestConfig_Validate` - verify validation catches invalid values (table-driven with negative MaxLength, negative HashLength, negative Timeout, and valid config cases)
- `TestConfigPaths` - verify path ordering (use table-driven tests for various path scenarios)
- `TestLoad_SingleFile` - parse various TOML configs (use table-driven tests for different config files)
- `TestLoad_SequentialOverlay` - verify sequential decoding overlays values correctly (use table-driven tests with multiple config files to verify that higher priority files overwrite lower priority files)
- `TestLoad_ZeroValueOverwrite` - verify that zero values in TOML files overwrite previous values (documents the sequential decoding behavior)
- `TestLoad_Integration` - full loading with temp files
- `TestLoad_ReturnsSourcePaths` - verify source path tracking
- `TestLoad_UndecodedKeys` - test detection of unknown config keys for user warnings

**Edge Case Tests:**

- `TestLoad_MissingFile` - verify missing files are skipped gracefully
- `TestLoad_UnreadableFile` - verify permission denied errors include file path
- `TestLoad_EmptyFile` - verify empty files are treated as valid with no overlay
- `TestLoad_InvalidTOML` - verify syntax errors include file path and line number
- `TestLoad_InvalidConfigValues` - verify validation catches:
  - Negative MaxLength
  - Negative HashLength
  - Other constraint violations with descriptive errors
- `TestLoad_PathIsDirectory` - verify directories are skipped (or error with clear message)
- `TestConfigPaths_HomeDirectoryError` - verify graceful handling when home directory unavailable

**Dependencies:** Add to `go.mod`:
```
github.com/BurntSushi/toml v1.x.x
```

### 1.6 Phase 1 Milestone

**Definition of Done:**
- [ ] Config types defined with TOML tags
- [ ] Default config function returns sensible values
- [ ] Config path discovery returns correct order
- [ ] Config loader merges files and tracks source paths
- [ ] All tests pass, `make check` succeeds

**Commit:** After completing this phase, commit all changes with a descriptive message.

---

## Phase 2: Naming/Generation Layer

### 2.1 Branch Name Generator

**File:** `internal/naming/branch.go`

```go
package naming

// BranchNameGenerator creates branch names from user input
type BranchNameGenerator struct {
    prefix        string
    slugifyOpts   SlugifyOptions
}

// NewBranchNameGenerator creates a generator from config
func NewBranchNameGenerator(branchCfg BranchConfig, slugCfg SlugifyConfig) *BranchNameGenerator

// Generate creates a branch name from a phrase
// e.g., "add user auth" -> "feature/add-user-auth"
func (g *BranchNameGenerator) Generate(phrase string) string
```

### 2.2 Worktree Name Generator

**File:** `internal/naming/worktree.go`

Don't assume branch names are pre-slugified - the worktree generator should ensure consistency by applying slugify rules.

```go
package naming

// WorktreeNameGenerator creates worktree directory names
type WorktreeNameGenerator struct {
    prefix            string
    slugifyOpts       SlugifyOptions
    stripBranchPrefix []string
}

// NewWorktreeNameGenerator creates a generator from config
// Note: Takes SlugifyConfig to ensure branch names are properly slugified
// even if passed in raw form
func NewWorktreeNameGenerator(worktreeCfg WorktreeConfig, slugCfg SlugifyConfig) *WorktreeNameGenerator

// Generate creates a worktree name from a branch name
// The branch name is slugified to ensure consistency, then prefixes are stripped
// e.g., "feature/add-user-auth" -> "wt-add-user-auth"
func (g *WorktreeNameGenerator) Generate(branchName string) string
```

### 2.3 Naming Testing Strategy

**Files:**
- `internal/naming/branch_test.go`
- `internal/naming/worktree_test.go`

Test cases (use table-driven tests for these scenarios):
- Branch generation with various prefixes
- Branch generation with slugify options (max length, hash)
- Worktree generation with prefix stripping
- Edge cases: empty input, special characters, unicode

### 2.4 Phase 2 Milestone

**Definition of Done:**
- [ ] Branch name generator produces correct output for all test cases
- [ ] Worktree name generator correctly strips prefixes
- [ ] Edge cases handled (empty input, special chars)
- [ ] All tests pass, `make check` succeeds

**Commit:** After completing this phase, commit all changes with a descriptive message.

---

## Phase 3: Command Layer

### 3.1 Root Command Setup

**File:** `cmd/root.go`

```go
package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
    Use:   "grove",
    Short: "Git worktree workspace manager",
    Long:  `Grove manages git worktrees in a workspace structure.`,
}

// Execute runs the root command
func Execute() error {
    return rootCmd.Execute()
}

func init() {
    // Global flags here (future: --config override)
    // TODO: Add version command (grove version) to show version info
    // TODO: Add shell completion command (grove completion bash|zsh|fish)
}
```

### 3.2 Create Command

**File:** `cmd/create.go`

```go
package cmd

import "github.com/spf13/cobra"

var createCmd = &cobra.Command{
    Use:   "create <phrase>",
    Short: "Create a new branch and worktree",
    Long: `Create creates a new git branch and worktree from a descriptive phrase.

The new branch is created from the current HEAD (the commit you're currently on).
The phrase is converted to a branch name using the configured slugify rules
and prefix. A worktree is then created with the configured worktree naming.

Example:
  grove create "add user authentication"
  grove create "fix bug in login"

Note: The create command takes a single quoted string argument. The shell wrapper
function (grc) can handle passing arbitrary phrases by quoting the arguments.`,
    Args: cobra.ExactArgs(1), // Require exactly one argument (the phrase)
    RunE: runCreate,
}

**Output Behavior:**
- **stdout**: Only the worktree directory path (e.g., `/path/to/workspace/wt-add-user-auth`)
  - This allows clean integration with shell scripts: `cd $(grove create "...")`
  - Use `fmt.Fprintln(cmd.OutOrStdout(), path)` for the final path output
- **stderr**: All other output including:
  - Error messages
  - Warnings (e.g., unknown config keys)
  - Informational messages (if any)

**Logger Configuration (charmbracelet/log):**
- Configure the logger to write to stderr: `log.SetOutput(os.Stderr)`
- This ensures all log output (info, warn, error) goes to stderr
- Cobra's error handling also writes to stderr by default
- Never use `fmt.Println()` for logging - use the logger or `cmd.ErrOrStderr()`

This separation ensures the create command can be used in pipelines and scripts without parsing issues.

func init() {
    rootCmd.AddCommand(createCmd)
    // TODO: Add --dry-run flag to show what would be created without executing
    // TODO: Add --base flag to specify a different base commit/branch (default: HEAD)
    // Future: ValidArgsFunction for shell completion of recent branches/phrases
}

func runCreate(cmd *cobra.Command, args []string) error {
    // Note: Using RunE instead of Run to return errors for proper handling
    // Cobra will automatically display errors with "Error:" prefix

    phrase := args[0]

    // 1. Validate phrase is not empty
    if strings.TrimSpace(phrase) == "" {
        return errors.New("phrase cannot be empty")
    }

    // 2. Load config
    //    Obtain paths for ConfigPaths():
    //    - cwd: os.Getwd()
    //    - worktreeRoot: git.GetWorktreeRoot() from internal/git package
    //    - gitRoot: git.GetMainWorktreePath() from internal/git package (main worktree)
    //    - homeDir: os.UserHomeDir()
    //    Then call config.ConfigPaths(cwd, worktreeRoot, gitRoot, homeDir)
    //    and loader.Load(paths)
    // 3. Validate we're in a git repository
    //    Use: git.GetWorktreeRoot() - returns "" if not in a repo
    // 4. Generate branch name (slugify the phrase)
    //    Use: naming.BranchNameGenerator.Generate()
    // 5. Validate slug is not empty - error out with helpful message
    //    If slug is empty:
    //      Error: phrase '!!!' produces an empty branch name after slugification
    //
    //      Please provide a phrase with at least one alphanumeric character.
    //      Examples:
    //        grove create "add user auth"
    //        grove create "fix-bug-123"
    // 6. Check if branch already exists - error out with helpful message
    //    Use: git.BranchExists(branchName, false)
    //    If branch exists:
    //      Error: branch 'feature/add-user-auth' already exists
    //
    //      To use the existing branch, try:
    //        git worktree add <path> feature/add-user-auth
    //
    //      Or choose a different name for your new branch.
    // 7. Generate worktree name
    //    Use: naming.WorktreeNameGenerator.Generate()
    // 8. Determine worktree path (workspace root + worktree name)
    //    Use: git.GetMainWorktreePath() then filepath.Dir() for workspace root
    //    Then: filepath.Join(workspaceRoot, worktreeName)
    // 9. Check if worktree path already exists - error out with helpful message
    //    Use: os.Stat() to check if path exists
    //    If path exists:
    //      Error: worktree path '/path/to/workspace/wt-add-user-auth' already exists
    //
    //      To remove the existing worktree:
    //        git worktree remove wt-add-user-auth
    //
    //      Or choose a different name for your new branch.
    // 10. Create branch and worktree from current HEAD
    //     Use: git.CreateWorktreeForNewBranchFromRef(branchName, worktreePath, "")
    //     Note: empty baseRef means use HEAD
    //     Note: This is an atomic operation - creates both branch and worktree together
    //     If it fails, neither should exist (git handles this internally)
    // 11. Output ONLY the worktree path to stdout (for shell integration)
    //     All other messages go to stderr
    return nil
}

// Failure Handling Policy:
// - No automatic rollback on partial failure
// - If CreateWorktreeForNewBranchFromRef fails, the error message from git is surfaced
// - The git layer's atomic operation means branch+worktree are created together
// - If a truly partial state occurs (unlikely), show cleanup instructions:
//   "Branch 'feature/x' may have been created. To cleanup: git branch -d feature/x"
// - User decides whether to retry, cleanup, or investigate
```

**Implementation Notes:**
- Uses `RunE` instead of `Run` for proper error handling (see Cobra CLI Best Practices)
- Uses `cobra.ExactArgs(1)` to require exactly one phrase argument
- Added validation for empty phrase input
- The create command accepts a single string parameter (already quoted by the shell)
- Future shell completion support can be added via `ValidArgsFunction`

**Empty Slug Error:**
When the slugified phrase results in an empty string (e.g., input was only special characters), error out with helpful information:

```
Error: phrase '!!!' produces an empty branch name after slugification

Please provide a phrase with at least one alphanumeric character.
Examples:
  grove create "add user auth"
  grove create "fix-bug-123"
```

### 3.3 Init Command (Shell Functions)

**File:** `cmd/init.go`

```go
package cmd

import "github.com/spf13/cobra"

var initCmd = &cobra.Command{
    Use:   "init [shell]",
    Short: "Generate shell integration functions",
    Long: `Init outputs shell functions for integration with your shell.

Add to your shell config:
  Fish:  grove init fish | source
  Zsh:   eval "$(grove init zsh)"
  Bash:  eval "$(grove init bash)"`,
    Args:      cobra.ExactArgs(1),
    ValidArgs: []string{"fish", "zsh", "bash"},
    RunE:      runInit,
}

func init() {
    rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
    shell := args[0]
    // Output shell-specific function
    return nil
}
```

### 3.4 Main Entry Point Update

**File:** `main.go`

```go
package main

import (
    "os"
    "github.com/jmcampanini/grove-cli/cmd"
)

func main() {
    if err := cmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

### 3.5 Command Testing Strategy

**Files:**
- `cmd/create_test.go`
- `cmd/init_test.go`

Test approach:
- Use Cobra's testing utilities
- For unit testing create command, extend `internal/git` package with an interface if not already present, enabling test doubles
- Capture stdout for init command output verification
- Integration tests with temporary git repos (following patterns from `git_cli_integration_test.go`)
- Use table-driven tests where applicable for testing various command-line argument combinations

**Dependencies:** Add to `go.mod`:
```
github.com/spf13/cobra v1.x.x
```

### 3.6 Phase 3 Milestone

**Definition of Done:**
- [ ] Root command shows help and version
- [ ] Create command produces branch and worktree
- [ ] Create command outputs worktree path on success
- [ ] Init command outputs valid shell functions
- [ ] Error messages are clear and actionable
- [ ] All tests pass, `make check` succeeds
- [ ] TODO: Add version command in future iteration
- [ ] TODO: Add shell completion command in future iteration

**Commit:** After completing this phase, commit all changes with a descriptive message.

---

## Phase 4: Shell Integration Layer

### 4.1 Shell Function Templates and File Structure

**File Structure for Shell Scripts:**
```
internal/shell/
├── functions.go       # Go code that uses embedded scripts
├── functions_test.go
└── scripts/
    ├── grc.fish      # Fish shell function (standalone file)
    ├── grc.bash      # Bash shell function (standalone file)
    └── grc.zsh       # Zsh shell function (can be same as bash)
```

**Embedding Scripts:**

Shell scripts live as files in the repo and are embedded using Go's `embed` package. This makes the content easy to see and edit.

**File:** `internal/shell/functions.go`

```go
package shell

import (
    _ "embed"
)

//go:embed scripts/grc.fish
var fishScript string

//go:embed scripts/grc.bash
var bashScript string

//go:embed scripts/grc.zsh
var zshScript string

// FunctionGenerator generates shell functions
type FunctionGenerator struct{}

// GenerateFish returns the fish shell function
func (g *FunctionGenerator) GenerateFish() string {
    return fishScript
}

// GenerateZsh returns the zsh shell function
func (g *FunctionGenerator) GenerateZsh() string {
    return zshScript
}

// GenerateBash returns the bash shell function
func (g *FunctionGenerator) GenerateBash() string {
    return bashScript
}
```

**Benefits:**
- Shell scripts are easy to read, edit, and test in isolation
- Syntax highlighting works in editors
- Can be validated with shell linting tools
- Compile-time embedding ensures scripts are always included

### 4.2 Shell Function Specifications

**Function Naming Convention:**
- Public functions: `grc` (short, user-friendly names)
- Internal helper functions: `__grove_*` prefix (e.g., `__grove_handle_error`)
  - The double underscore prefix follows shell convention for "private" functions
  - Prevents collisions with other shell functions in the user's environment

The current implementation uses a single self-contained `grc` function. As the shell integration grows and requires helper functions, they should use the `__grove_` prefix to avoid name collisions.

**Fish (`grc` function):**
```fish
function grc --description "Grove create - create branch and worktree"
    # Tries to use zoxide (z) for navigation if available, falls back to cd
    # Pass all arguments as a single quoted string to grove create
    set -l output (grove create "$argv")
    if test $status -eq 0
        if command -q z
            z $output
        else
            cd $output
        end
    else
        echo $output
        return 1
    end
end
```

**Zsh/Bash (`grc` function):**
```bash
grc() {
    # Tries to use zoxide (z) for navigation if available, falls back to cd
    local output
    # Pass all arguments as a single quoted string to grove create
    output=$(grove create "$*")
    if [ $? -eq 0 ]; then
        if command -v z &> /dev/null; then
            z "$output"
        else
            cd "$output"
        fi
    else
        echo "$output"
        return 1
    fi
}
```

**Shell Wrapper Notes:**
- Fish: `"$argv"` passes all arguments as a single quoted string
- Bash/Zsh: `"$*"` joins all arguments with spaces into a single string
- This allows users to invoke `grc add user authentication` without manually quoting
- The wrapper handles the quoting so the grove CLI receives a single argument
- **Important**: These shell functions rely on `grove create` outputting only the worktree path to stdout (see section 3.2 Output Behavior). This enables clean command substitution and navigation without parsing issues.

### 4.3 Shell Integration Best Practices

When implementing shell functions, follow these shell-specific best practices:

**Fish Shell:**
- Use `$status` for exit code (not `$?` like bash)
- Use `if command` directly or `if test $status -eq 0` for success checks
- Use `set -l varname value` for local variables
- Use `command -q` to check if a command exists (e.g., `command -q z` for zoxide)
- Strings are quoted with `"` (single quotes are less common)

**Bash/Zsh:**
- Use `$?` for exit code of last command
- Use `command -v cmd >/dev/null 2>&1` to check if command exists
- Use `local varname` for local variables
- Redirect errors to stderr with `>&2`
- Use `set -e` in scripts to exit on error (not typically in functions)

**All Shells:**
- Check command success before proceeding
- Handle errors gracefully with helpful messages
- Use stderr for errors, stdout for output that will be consumed by other commands
- Test shell syntax with `bash -n script.sh` or `fish --no-execute script.fish`

**Implementation Notes:**
- The shell functions in section 4.2 follow these practices
- Fish: Uses `test $status -eq 0` for success check and `command -q z` for zoxide detection
- Bash/Zsh: Uses `[ $? -eq 0 ]` for success check and `command -v z &> /dev/null` for zoxide detection
- All functions properly handle errors by echoing output and returning non-zero exit codes

### 4.4 Naming Convention

Per spec requirement for extendable naming:
- `grc` - Grove Create
- Future: `grw` - Grove Worktree (list/switch)
- Future: `grd` - Grove Delete

### 4.5 Shell Testing Strategy

**File:** `internal/shell/functions_test.go`

- Verify output contains required function name
- Verify zoxide detection logic
- Verify grove command invocation
- Optional: Shell syntax validation using shell -n

### 4.6 Phase 4 Milestone

**Definition of Done:**
- [ ] Fish function works with zoxide fallback
- [ ] Zsh function works with zoxide fallback
- [ ] Bash function works with zoxide fallback
- [ ] Shell syntax validates with `shell -n`
- [ ] All tests pass, `make check` succeeds

**Commit:** After completing this phase, commit all changes with a descriptive message.

---

## File Structure Summary

```
grove-cli/
├── cmd/
│   ├── create.go          # Create command implementation
│   ├── create_test.go     # Create command tests
│   ├── init.go            # Shell init command
│   ├── init_test.go       # Init command tests
│   └── root.go            # Root command setup
├── internal/
│   ├── config/
│   │   ├── config.go      # Config types
│   │   ├── config_test.go # Config tests
│   │   ├── defaults.go    # Default values
│   │   ├── discovery.go   # Path discovery
│   │   └── loader.go      # TOML loading/merging
│   ├── git/               # (existing)
│   ├── naming/
│   │   ├── branch.go      # Branch name generator
│   │   ├── branch_test.go
│   │   ├── slugify.go     # (existing)
│   │   ├── slugify_test.go # (existing)
│   │   ├── worktree.go    # Worktree name generator
│   │   └── worktree_test.go
│   └── shell/
│       ├── functions.go   # Shell function generation
│       ├── functions_test.go
│       └── scripts/
│           ├── grc.fish   # Fish shell function
│           ├── grc.bash   # Bash shell function
│           └── grc.zsh    # Zsh shell function
├── go.mod                 # Add cobra, toml deps
├── main.go                # Updated entry point
└── Makefile               # (existing)
```

---

## External Dependencies

| Package | Purpose | Version |
|---------|---------|---------|
| github.com/spf13/cobra | CLI framework | v1.8.x |
| github.com/BurntSushi/toml | TOML parsing | v1.3.x |

Both are well-maintained, widely-used Go libraries.

---

## Risks and Edge Cases

### Git Operation Failures

| Risk | Mitigation |
|------|------------|
| Not in a git repository | Check early with clear error: "grove must be run inside a git repository" |
| Branch already exists | Use `BranchExists()` before creation; error with: "Error: branch 'feature/add-user-auth' already exists\n\nTo use the existing branch, try:\n  git worktree add <path> feature/add-user-auth\n\nOr choose a different name for your new branch." |
| Worktree path already exists | Check path before creation; error with: "Error: worktree path '/path/to/workspace/wt-add-user-auth' already exists\n\nTo remove the existing worktree:\n  git worktree remove wt-add-user-auth\n\nOr choose a different name for your new branch." |
| Main worktree path discovery fails | Clear error with troubleshooting steps |
| Git command execution fails | Surface git's error message with context |
| Branch created but worktree fails | Don't auto-delete branch; show: "Branch 'feature/x' was created but worktree failed. To cleanup: git branch -d feature/x" |

### Config Resolution Failures

| Risk | Mitigation |
|------|------------|
| File doesn't exist | Skip gracefully, continue to next file in priority order |
| File exists but isn't readable | Return error with clear message including path |
| File is empty | Treat as valid (no values to overlay), continue |
| TOML parse error | Include file path and line number in error message |
| Conflicting configs | Document merge behavior; higher priority wins |
| Invalid config values | Validate at load time with clear messages (see section 1.4 for details) |
| Config path is a directory | Skip gracefully |
| Unknown/extra fields | Warn user but don't error |
| Home directory not available | Handle `os.UserHomeDir()` errors gracefully for XDG path |
| No config found | Use sensible defaults; document in help text |

**Note:** Detailed edge case handling and test coverage is documented in section 1.4.

### Shell Function Conflicts

| Risk | Mitigation |
|------|------------|
| Function name collision | Public functions like `grc` use short names; internal helper functions use `__grove_` prefix (e.g., `__grove_handle_error`) to avoid collisions with user's shell environment |
| Zoxide not in PATH | Detect with `command -v z` and fallback gracefully |
| Shell function differs from binary name | Document clearly; `grove` = binary, `grc` = shell function |

### Naming Edge Cases

| Risk | Mitigation |
|------|------------|
| Empty phrase input | Require minimum args; show usage |
| Phrase produces empty slug | Error out with helpful information: "Error: phrase '!!!' produces an empty branch name after slugification\n\nPlease provide a phrase with at least one alphanumeric character.\nExamples:\n  grove create \"add user auth\"\n  grove create \"fix-bug-123\"" |
| Very long phrases | MaxLength config with truncation + hash |
| Unicode/special characters | Slugify normalizes to ASCII |

---

## Action Items Checklist

### Phase 1: Configuration Layer
- [ ] Add `github.com/BurntSushi/toml` dependency
- [ ] Create `internal/config/config.go` with type definitions
- [ ] Create `internal/config/defaults.go` with DefaultConfig()
- [ ] Create `internal/config/discovery.go` with ConfigPaths()
- [ ] Create `internal/config/loader.go` with Load() returning LoadResult
- [ ] Create `internal/config/config_test.go` with comprehensive tests
- [ ] Run `make check` to validate
- [ ] Commit changes for Phase 1 with descriptive message

### Phase 2: Naming/Generation Layer
- [ ] Create `internal/naming/branch.go` with BranchNameGenerator
- [ ] Create `internal/naming/branch_test.go`
- [ ] Create `internal/naming/worktree.go` with WorktreeNameGenerator
- [ ] Create `internal/naming/worktree_test.go`
- [ ] Run `make check` to validate
- [ ] Commit changes for Phase 2 with descriptive message

### Phase 3: Command Layer
- [ ] Add `github.com/spf13/cobra` dependency
- [ ] Create `cmd/root.go` with root command
- [ ] Create `cmd/create.go` with create command
- [ ] Create `cmd/create_test.go`
- [ ] Create `cmd/init.go` with shell init command
- [ ] Create `cmd/init_test.go`
- [ ] Update `main.go` to call cmd.Execute()
- [ ] Run `make check` to validate
- [ ] Commit changes for Phase 3 with descriptive message

### Phase 4: Shell Integration Layer
- [ ] Create `internal/shell/functions.go` with templates
- [ ] Create `internal/shell/functions_test.go`
- [ ] Wire shell generation into cmd/init.go
- [ ] Run `make check` to validate
- [ ] Commit changes for Phase 4 with descriptive message

### Final Integration
- [ ] End-to-end manual test of `grove create` + shell function
- [ ] Verify all edge cases produce clear error messages
- [ ] Final `make check` and review

---

## Future Scope (Out of Current Implementation)

These items are explicitly deferred but designed for:

1. **CLI/Environment Overrides**: Config loading interface supports adding `--config` flag and env var precedence
2. **Diagnostics Command**: `grove doctor` or `grove config show` to display resolved config and source paths (LoadResult.SourcePaths enables this)
3. **Dry-Run Mode**: `--dry-run` flag to show what would be created without executing
4. **Base Ref Flag**: `--base` flag to specify a different base commit/branch (default: HEAD)
5. **Version Command**: `grove version` to display the current version, build info, etc.
6. **Shell Completion**: `grove completion bash|zsh|fish` to generate shell completion scripts using Cobra's built-in completion generator
7. **Verbose Flag**: `--verbose` or `-v` flag to show which config files were loaded (uses LoadResult.SourcePaths)
8. **Shell Script Linting**: Add shellcheck validation for embedded shell scripts (make lint-shell target)

---

## Testing Verification

Run `make check` after each phase to verify:
1. All tests pass (`go test ./...`)
2. Linting passes (`golangci-lint run ./...`)
3. Build succeeds (`go build`)

When writing tests:
- Use table-driven tests for functions with multiple input/output combinations
- Test helpers should call `t.Helper()` for better error reporting
- Use `wantErr bool` pattern for error testing
- Create interface test doubles rather than mocking concrete types
