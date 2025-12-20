# Implementation Plan: Grove CLI - Layered Architecture with Practical Milestones

## Overview

This plan implements the grove-cli `create` command using a layered architecture that enables independent testability while maintaining practical delivery milestones. Each phase produces working, testable code that can be validated before proceeding.

**Target Command:**
```bash
grc the name of a feature im working on
```

This will:
1. Create a git branch with configurable naming convention
2. Create a worktree with configurable naming convention
3. Output a path for shell integration to cd into (using zoxide if available)

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
    Slugify  SlugifyConfig
    Worktree WorktreeConfig
}

// BranchConfig configures branch naming
type BranchConfig struct {
    NewPrefix string `toml:"new_prefix"` // e.g., "feature/"
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
            NewPrefix: "",
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
            StripBranchPrefix: []string{},
        },
    }
}
```

### 1.3 Config File Discovery

**File:** `internal/config/discovery.go`

Implement hierarchical config file discovery:

```go
package config

// ConfigPaths returns ordered list of config file paths to check (highest to lowest priority)
// Order:
// 1. File in current working directory
// 2. File in git worktree root
// 3. Files in directories up to home/root
// 4. File in XDG config directory
func ConfigPaths(cwd, gitRoot, homeDir string) []string
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

// Load reads and merges all config files in priority order
// Returns merged config with defaults as base, plus source paths for debugging
func (l *Loader) Load(paths []string) (LoadResult, error)

// loadFile reads a single TOML config file
func (l *Loader) loadFile(path string) (*Config, error)

// merge combines two configs, with overlay taking precedence for non-zero values
func merge(base, overlay Config) Config
```

Note: `LoadResult.SourcePaths` aids debugging by showing which config files were applied.

### 1.5 Config Testing Strategy

**File:** `internal/config/config_test.go`

Tests following existing patterns from `slugify_test.go`:

- `TestDefaultConfig` - verify defaults are sensible
- `TestConfigPaths` - verify path ordering
- `TestLoadFile` - parse various TOML configs
- `TestMerge` - verify overlay behavior
- `TestLoad_Integration` - full loading with temp files
- `TestLoad_ReturnsSourcePaths` - verify source path tracking

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

```go
package naming

// WorktreeNameGenerator creates worktree directory names
type WorktreeNameGenerator struct {
    prefix            string
    stripBranchPrefix []string
}

// NewWorktreeNameGenerator creates a generator from config
func NewWorktreeNameGenerator(worktreeCfg WorktreeConfig) *WorktreeNameGenerator

// Generate creates a worktree name from a branch name
// e.g., "feature/add-user-auth" -> "wt-add-user-auth"
func (g *WorktreeNameGenerator) Generate(branchName string) string
```

### 2.3 Naming Testing Strategy

**Files:**
- `internal/naming/branch_test.go`
- `internal/naming/worktree_test.go`

Test cases:
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
}
```

### 3.2 Create Command

**File:** `cmd/create.go`

```go
package cmd

import "github.com/spf13/cobra"

var createCmd = &cobra.Command{
    Use:   "create [phrase]",
    Short: "Create a new branch and worktree",
    Long: `Create creates a new git branch and worktree from a descriptive phrase.

The phrase is converted to a branch name using the configured slugify rules
and prefix. A worktree is then created with the configured worktree naming.

Example:
  grove create add user authentication
  grove create "fix bug in login"`,
    Args: cobra.MinimumNArgs(1),
    RunE: runCreate,
}

func init() {
    rootCmd.AddCommand(createCmd)
    // Future: --base flag for base branch
    // Future: --dry-run flag
}

func runCreate(cmd *cobra.Command, args []string) error {
    // 1. Join args into phrase
    // 2. Load config (with source path tracking for error messages)
    // 3. Validate we're in a git repository
    // 4. Generate branch name
    // 5. Check if branch already exists
    // 6. Generate worktree name
    // 7. Check if worktree path already exists
    // 8. Determine worktree path (workspace root + worktree name)
    // 9. Call git.CreateWorktreeForNewBranch
    // 10. Output the worktree path for shell integration
    return nil
}
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
- Mock git interface for create command
- Capture stdout for init command output verification
- Integration tests with temporary git repos (following patterns from `git_cli_integration_test.go`)

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

---

## Phase 4: Shell Integration Layer

### 4.1 Shell Function Templates

**File:** `internal/shell/functions.go`

```go
package shell

// FunctionGenerator generates shell functions
type FunctionGenerator struct{}

// GenerateFish returns fish shell functions
func (g *FunctionGenerator) GenerateFish() string

// GenerateZsh returns zsh shell functions
func (g *FunctionGenerator) GenerateZsh() string

// GenerateBash returns bash shell functions
func (g *FunctionGenerator) GenerateBash() string
```

### 4.2 Shell Function Specifications

**Fish (`grc` function):**
```fish
function grc --description "Grove create - create branch and worktree"
    set -l output (grove create $argv)
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
    local output
    output=$(grove create "$@")
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

### 4.3 Naming Convention

Per spec requirement for extendable naming:
- `grc` - Grove Create
- Future: `grw` - Grove Worktree (list/switch)
- Future: `grd` - Grove Delete

### 4.4 Shell Testing Strategy

**File:** `internal/shell/functions_test.go`

- Verify output contains required function name
- Verify zoxide detection logic
- Verify grove command invocation
- Optional: Shell syntax validation using shell -n

### 4.5 Phase 4 Milestone

**Definition of Done:**
- [ ] Fish function works with zoxide fallback
- [ ] Zsh function works with zoxide fallback
- [ ] Bash function works with zoxide fallback
- [ ] Shell syntax validates with `shell -n`
- [ ] All tests pass, `make check` succeeds

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
│       └── functions_test.go
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
| Branch already exists | Use `BranchExists()` before creation; suggest alternative name or `--force` |
| Worktree path already exists | Check path before creation; suggest removal or alternative path |
| Main worktree path discovery fails | Clear error with troubleshooting steps |
| Git command execution fails | Surface git's error message with context |

### Config Resolution Failures

| Risk | Mitigation |
|------|------------|
| TOML parse error | Include file path and line number in error message |
| Conflicting configs | Document merge behavior; higher priority wins |
| Invalid config values | Validate at load time with clear messages |
| No config found | Use sensible defaults; document in help text |

### Shell Function Conflicts

| Risk | Mitigation |
|------|------------|
| Function name collision | Use `__grove_` prefix for internal functions |
| Zoxide not in PATH | Detect with `command -v z` and fallback gracefully |
| Shell function differs from binary name | Document clearly; `grove` = binary, `grc` = shell function |

### Naming Edge Cases

| Risk | Mitigation |
|------|------------|
| Empty phrase input | Require minimum args; show usage |
| Phrase produces empty slug | Add hash suffix to guarantee non-empty |
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

### Phase 2: Naming/Generation Layer
- [ ] Create `internal/naming/branch.go` with BranchNameGenerator
- [ ] Create `internal/naming/branch_test.go`
- [ ] Create `internal/naming/worktree.go` with WorktreeNameGenerator
- [ ] Create `internal/naming/worktree_test.go`
- [ ] Run `make check` to validate

### Phase 3: Command Layer
- [ ] Add `github.com/spf13/cobra` dependency
- [ ] Create `cmd/root.go` with root command
- [ ] Create `cmd/create.go` with create command
- [ ] Create `cmd/create_test.go`
- [ ] Create `cmd/init.go` with shell init command
- [ ] Create `cmd/init_test.go`
- [ ] Update `main.go` to call cmd.Execute()
- [ ] Run `make check` to validate

### Phase 4: Shell Integration Layer
- [ ] Create `internal/shell/functions.go` with templates
- [ ] Create `internal/shell/functions_test.go`
- [ ] Wire shell generation into cmd/init.go
- [ ] Run `make check` to validate

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
4. **Base Ref Flag**: `--base` flag to create branch from specific ref

---

## Testing Verification

Run `make check` after each phase to verify:
1. All tests pass (`go test ./...`)
2. Linting passes (`golangci-lint run ./...`)
3. Build succeeds (`go build`)
