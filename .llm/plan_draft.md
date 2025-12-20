# Implementation Plan: Grove CLI - Bottom-Up Layered Architecture

## Overview

This plan implements the grove-cli `create` command using a bottom-up layered architecture. The implementation builds foundational layers first, with each layer being independently testable and having clear dependency directions.

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
    NewPrefix string // e.g., "feature/"
}

// SlugifyConfig configures slug generation (matches existing SlugifyOptions)
type SlugifyConfig struct {
    CollapseDashes     bool
    HashLength         int
    Lowercase          bool
    MaxLength          int
    ReplaceNonAlphanum bool
    TrimDashes         bool
}

// WorktreeConfig configures worktree naming
type WorktreeConfig struct {
    NewPrefix         string   // e.g., "wt-"
    StripBranchPrefix []string // e.g., ["feature/"]
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

// Loader handles configuration loading and merging
type Loader struct {
    fs FileSystem // interface for testability
}

// Load reads and merges all config files in priority order
// Returns merged config with defaults as base
func (l *Loader) Load(paths []string) (Config, error)

// loadFile reads a single TOML config file
func (l *Loader) loadFile(path string) (*Config, error)

// merge combines two configs, with overlay taking precedence for non-zero values
func merge(base, overlay Config) Config
```

### 1.5 Config Testing Strategy

**File:** `internal/config/config_test.go`

Tests following existing patterns from `slugify_test.go`:

- `TestDefaultConfig` - verify defaults are sensible
- `TestConfigPaths` - verify path ordering
- `TestLoadFile` - parse various TOML configs
- `TestMerge` - verify overlay behavior
- `TestLoad_Integration` - full loading with temp files

**Dependencies:** Add to `go.mod`:
```
github.com/BurntSushi/toml v1.x.x
```

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
}

func runCreate(cmd *cobra.Command, args []string) error {
    // 1. Join args into phrase
    // 2. Load config
    // 3. Generate branch name
    // 4. Generate worktree name
    // 5. Determine worktree path (workspace root + worktree name)
    // 6. Call git.CreateWorktreeForNewBranch
    // 7. Output the worktree path for shell integration
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

## Testing Verification

Run `make check` after each phase to verify:
1. All tests pass (`go test ./...`)
2. Linting passes (`golangci-lint run ./...`)
3. Build succeeds (`go build`)

---

## Pros and Cons

### Pros

1. **Independent Testability**: Each layer can be tested in isolation. Config loading doesn't need git. Naming generators don't need the filesystem. Commands can mock lower layers.

2. **Clear Dependency Direction**: Dependencies flow downward only. Upper layers depend on lower layers, never the reverse. This prevents circular dependencies and makes the codebase easier to understand.

3. **Configuration as First-Class Citizen**: By building config first, all subsequent layers have access to properly typed, validated configuration. No string-based config lookups scattered through code.

4. **Incremental Deliverables**: Each phase produces working, testable code. The config layer works before commands exist. Naming generators work before shell integration exists.

5. **Easy to Extend**: Adding new config options flows naturally down through layers. Adding new commands just requires using existing lower layers. Adding new shells only requires new template.

6. **Matches Go Idioms**: Follows Go's preference for explicit dependencies, interfaces for testing, and package-level organization.

### Cons

1. **Upfront Investment**: Must build foundational layers before seeing end-to-end functionality. The create command only works after config, naming, and git integration are complete.

2. **Potential Over-Engineering**: For a relatively simple CLI, this structure may feel heavy. A single-file implementation would work for the current scope.

3. **Indirection Overhead**: Data flows through multiple layers (config -> naming -> command). Each layer adds a small amount of complexity and cognitive overhead.

4. **Testing Duplication Risk**: May end up with similar tests at multiple layers (e.g., testing slugify behavior both in unit tests and integration tests).

5. **Rigid Structure**: The layered architecture makes it harder to implement features that don't fit neatly into existing layers. Cross-cutting concerns require careful consideration.

### When to Prefer This Approach

**Use bottom-up layered architecture when:**
- The application will grow significantly over time
- Multiple entry points will use the same core logic (CLI, library, API)
- Configuration is complex with multiple sources
- Team has multiple developers who can work in parallel on different layers
- Long-term maintainability is more important than speed-to-first-feature

**Consider alternatives when:**
- Building a quick prototype or MVP
- The feature set is well-defined and unlikely to change
- Single developer working alone
- Configuration is simple or non-existent
- Speed to delivery is the primary concern
