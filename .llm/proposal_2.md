# Implementation Plan: grove-cli `create` Command

## Command-Centric / Use-Case Driven Approach

This plan is organized around commands as the primary organizing principle. Each command is a self-contained "use case" that owns its full workflow, with shared utilities extracted only when duplication actually occurs.

---

## Executive Summary

The goal is to implement the `grc <phrase>` command that:
1. Creates a git branch with a configured naming convention
2. Creates a worktree with a configured naming convention
3. Outputs shell commands that can be sourced to change directory (using zoxide if available)

This plan follows a command-centric approach where:
- The `create` command owns its complete workflow
- Config loading happens inline within the command
- Shell function generation is per-command
- Shared code is extracted only when actually duplicated

---

## Phase 1: The `create` Command (Primary Use Case)

### 1.1 Command Structure

Create the command file at `cmd/create.go` that owns the entire workflow:

```
cmd/
  config.go          # Config types and loading (inline with create)
  create.go          # The create command - self-contained use case
  root.go            # Root command setup (Cobra boilerplate)
  shell.go           # Shell function generation command
```

The `create` command will:
1. Parse the input phrase
2. Load config inline (no separate config package yet)
3. Generate branch name using slugify + branch config
4. Generate worktree path using slugify + worktree config
5. Call git operations to create branch + worktree
6. Output the worktree path (for shell integration)

### 1.2 Config Loading Within Create Command

Config is loaded inline in `cmd/create.go`. The config struct lives in a simple `config.go` file adjacent to the command:

```go
// cmd/config.go - minimal config types for create command

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

Config loading function (inline in `cmd/config.go`):

```go
func loadConfig() (*Config, error) {
    // Load from paths in priority order (highest to lowest):
    // 1. cwd/grove.toml
    // 2. git root/grove.toml
    // 3. parent dirs up to home or root
    // 4. XDG config dir/grove/grove.toml

    // Merge configs with higher priority overwriting lower
    // Return merged config
}
```

### 1.3 Create Command Implementation

```go
// cmd/create.go

func init() {
    rootCmd.AddCommand(createCmd)
}

var createCmd = &cobra.Command{
    Use:   "create <phrase>",
    Short: "Create a new branch and worktree from a phrase",
    Args:  cobra.MinimumNArgs(1),
    RunE:  runCreate,
}

func runCreate(cmd *cobra.Command, args []string) error {
    phrase := strings.Join(args, " ")

    // 1. Load config inline
    cfg, err := loadConfig()
    if err != nil {
        return fmt.Errorf("loading config: %w", err)
    }

    // 2. Generate branch name
    slugOpts := naming.SlugifyOptions{
        CollapseDashes:     cfg.Slugify.CollapseDashes,
        HashLength:         cfg.Slugify.HashLength,
        Lowercase:          cfg.Slugify.Lowercase,
        MaxLength:          cfg.Slugify.MaxLength,
        ReplaceNonAlphaNum: cfg.Slugify.ReplaceNonAlphanum,
        TrimDashes:         cfg.Slugify.TrimDashes,
    }
    slug := naming.Slugify(phrase, slugOpts)
    branchName := cfg.Branch.NewPrefix + slug

    // 3. Generate worktree directory name
    worktreeName := branchName
    for _, prefix := range cfg.Worktree.StripBranchPrefix {
        worktreeName = strings.TrimPrefix(worktreeName, prefix)
    }
    worktreeName = cfg.Worktree.NewPrefix + worktreeName

    // 4. Determine worktree path (sibling to main worktree)
    g := git.New(false, ".", 30*time.Second)
    mainPath, err := g.GetMainWorktreePath()
    if err != nil {
        return fmt.Errorf("getting main worktree path: %w", err)
    }
    worktreePath := filepath.Join(filepath.Dir(mainPath), worktreeName)

    // 5. Create branch + worktree atomically
    if err := g.CreateWorktreeForNewBranch(branchName, worktreePath); err != nil {
        return fmt.Errorf("creating worktree: %w", err)
    }

    // 6. Output the path for shell integration
    fmt.Println(worktreePath)
    return nil
}
```

### 1.4 Dependencies to Add

Update `go.mod`:
```
github.com/BurntSushi/toml  # TOML parsing
github.com/spf13/cobra      # CLI framework
```

---

## Phase 2: Shell Function Generation Command

### 2.1 Shell Command Structure

Create `cmd/shell.go` for generating shell functions:

```go
// cmd/shell.go

var shellCmd = &cobra.Command{
    Use:   "shell",
    Short: "Generate shell functions for grove integration",
}

var shellInitCmd = &cobra.Command{
    Use:   "init <shell>",
    Short: "Output shell initialization script",
    Args:  cobra.ExactArgs(1),
    RunE:  runShellInit,
}

func init() {
    shellCmd.AddCommand(shellInitCmd)
    rootCmd.AddCommand(shellCmd)
}
```

### 2.2 Shell Function Templates

Each shell gets its own function template embedded in `cmd/shell.go`:

```go
const fishInit = `
# Grove CLI shell integration for fish
function __grove_create
    set -l result (grove create $argv)
    if test $status -eq 0
        if command -v zoxide >/dev/null 2>&1
            zoxide add $result
            cd $result
        else
            cd $result
        end
    end
end

# Alias for quick access
function grc
    __grove_create $argv
end
`

const zshInit = `
# Grove CLI shell integration for zsh
__grove_create() {
    local result
    result=$(grove create "$@")
    if [[ $? -eq 0 ]]; then
        if command -v zoxide &>/dev/null; then
            zoxide add "$result"
            cd "$result"
        else
            cd "$result"
        fi
    fi
}

# Alias for quick access
alias grc='__grove_create'
`

const bashInit = `
# Grove CLI shell integration for bash
__grove_create() {
    local result
    result=$(grove create "$@")
    if [[ $? -eq 0 ]]; then
        if command -v zoxide &>/dev/null; then
            zoxide add "$result"
            cd "$result"
        else
            cd "$result"
        fi
    fi
}

# Alias for quick access
alias grc='__grove_create'
`

func runShellInit(cmd *cobra.Command, args []string) error {
    shell := args[0]
    switch shell {
    case "fish":
        fmt.Print(fishInit)
    case "zsh":
        fmt.Print(zshInit)
    case "bash":
        fmt.Print(bashInit)
    default:
        return fmt.Errorf("unsupported shell: %s (use fish, zsh, or bash)", shell)
    }
    return nil
}
```

### 2.3 Usage Instructions

Users add to their shell config:
- Fish: `source (grove shell init fish | psub)`
- Zsh: `eval "$(grove shell init zsh)"`
- Bash: `eval "$(grove shell init bash)"`

---

## Phase 3: Config Loading Implementation

### 3.1 Config File Search Order

The config loading function in `cmd/config.go`:

```go
func loadConfig() (*Config, error) {
    cfg := defaultConfig()

    paths := configSearchPaths()
    // Reverse order so higher priority overwrites lower
    for i := len(paths) - 1; i >= 0; i-- {
        if err := mergeConfigFromFile(cfg, paths[i]); err != nil {
            // Log warning but continue - missing files are OK
        }
    }

    return cfg, nil
}

func configSearchPaths() []string {
    var paths []string

    // 1. XDG config dir (lowest priority)
    if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
        paths = append(paths, filepath.Join(xdgConfig, "grove", "grove.toml"))
    } else if home, err := os.UserHomeDir(); err == nil {
        paths = append(paths, filepath.Join(home, ".config", "grove", "grove.toml"))
    }

    // 2. Parent directories up to home/root
    cwd, _ := os.Getwd()
    home, _ := os.UserHomeDir()
    for dir := cwd; dir != "/" && dir != home; dir = filepath.Dir(dir) {
        paths = append(paths, filepath.Join(dir, "grove.toml"))
    }

    // 3. Git root (if different from cwd)
    if gitRoot := getGitRoot(); gitRoot != "" && gitRoot != cwd {
        paths = append(paths, filepath.Join(gitRoot, "grove.toml"))
    }

    // 4. Current working directory (highest priority)
    paths = append(paths, filepath.Join(cwd, "grove.toml"))

    return paths
}

func defaultConfig() *Config {
    return &Config{
        Branch: BranchConfig{
            NewPrefix: "feature/",
        },
        Slugify: SlugifyConfig{
            CollapseDashes:     true,
            HashLength:         4,
            Lowercase:          true,
            MaxLength:          20,
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

### 3.2 Config Merging

```go
func mergeConfigFromFile(base *Config, path string) error {
    data, err := os.ReadFile(path)
    if os.IsNotExist(err) {
        return nil // Missing config file is OK
    }
    if err != nil {
        return fmt.Errorf("reading %s: %w", path, err)
    }

    var overlay Config
    if _, err := toml.Decode(string(data), &overlay); err != nil {
        return fmt.Errorf("parsing %s: %w", path, err)
    }

    // Merge overlay into base (non-zero values only)
    mergeConfigs(base, &overlay)
    return nil
}
```

---

## Phase 4: Root Command and CLI Setup

### 4.1 Root Command

```go
// cmd/root.go

var rootCmd = &cobra.Command{
    Use:   "grove",
    Short: "A CLI for managing git worktrees",
    Long:  `Grove helps you easily create and manage multiple git worktrees.`,
}

func Execute() error {
    return rootCmd.Execute()
}
```

### 4.2 Main Entry Point

Update `main.go`:

```go
package main

import (
    "fmt"
    "os"

    "github.com/jmcampanini/grove-cli/cmd"
)

func main() {
    if err := cmd.Execute(); err != nil {
        fmt.Fprintln(os.Stderr, err)
        os.Exit(1)
    }
}
```

---

## File Structure Summary

```
grove-cli/
├── cmd/
│   ├── config.go          # Config types and loading (inline with create)
│   ├── create.go          # Create command - the primary use case
│   ├── root.go            # Cobra root command setup
│   └── shell.go           # Shell init command
├── internal/
│   ├── git/               # Existing - unchanged
│   │   ├── git.go
│   │   ├── git_cli.go
│   │   └── ...
│   └── naming/            # Existing - unchanged
│       ├── slugify.go
│       └── slugify_test.go
├── main.go                # Entry point, calls cmd.Execute()
├── go.mod
└── go.sum
```

---

## Implementation Sequence

### Step 1: Add Dependencies
1. Add `github.com/spf13/cobra` to go.mod
2. Add `github.com/BurntSushi/toml` to go.mod
3. Run `go mod tidy`

### Step 2: Create Root Command
1. Create `cmd/root.go` with basic Cobra setup
2. Update `main.go` to call `cmd.Execute()`
3. Verify `grove --help` works

### Step 3: Implement Config (Inline)
1. Create `cmd/config.go` with:
   - Config struct definitions (alpha-sorted fields per CLAUDE.md)
   - `loadConfig()` function with path search
   - Default config values
   - Config merging logic
2. Add tests for config loading in `cmd/config_test.go`

### Step 4: Implement Create Command
1. Create `cmd/create.go` with full workflow
2. Integrate with existing `internal/git` and `internal/naming`
3. Add tests in `cmd/create_test.go`
4. Run `make check` to validate

### Step 5: Implement Shell Init
1. Create `cmd/shell.go` with init subcommand
2. Add templates for fish, zsh, bash
3. Test shell function output

### Step 6: Integration Testing
1. Create test scenarios for full workflow
2. Test with different config file locations
3. Verify shell integration works as expected

---

## Testing Strategy

### Unit Tests
- `cmd/config_test.go`: Config loading, merging, path resolution
- `cmd/create_test.go`: Name generation, path calculation (mock git)

### Integration Tests
- Full workflow with temporary git repos (follow pattern in `internal/git/test_helpers_test.go`)
- Shell function verification

### Manual Testing
```bash
# Setup
make build

# Test create command
./build/grove create "my new feature"

# Test shell integration
eval "$(./build/grove shell init zsh)"
grc "test feature"
```

---

## Pros and Cons of Command-Centric Approach

### Pros

1. **Locality of Behavior**: Everything needed to understand the `create` command is in `cmd/create.go` and `cmd/config.go`. No need to trace through multiple abstraction layers.

2. **Simple Mental Model**: Commands are the primary organizing concept - matches how users think about the CLI.

3. **Easy to Add Commands**: New commands can be added without touching shared infrastructure. Each command is a self-contained unit.

4. **Avoids Premature Abstraction**: Config loading is simple today. If it needs to become more complex (CLI overrides, env vars), it can be extracted then.

5. **Faster Initial Development**: Less upfront design required. Get the feature working, then refactor if patterns emerge.

6. **Flat Structure**: Only 2 levels of package nesting (`cmd/` and `internal/`). Easy to navigate.

7. **Pragmatic Testing**: Can test commands as black boxes without mocking complex dependency graphs.

### Cons

1. **Potential Duplication**: If multiple commands need similar config loading, there may be copy-paste initially. This is intentional - extract when the pattern is clear.

2. **Larger Command Files**: `create.go` will be larger than if we split into many small packages. Trade-off: easier to understand vs more scrolling.

3. **Less Type Safety for Config**: Without a separate config package, config types are coupled to commands. If config structure changes, may need to update multiple places.

4. **Shell Templates in Go Code**: Embedding shell scripts as Go strings is less IDE-friendly. Could extract to files if needed.

5. **Testing Config Loading**: Without dependency injection, config loading is harder to mock. May need to use temp files or test-specific config paths.

### When to Prefer This Approach

- **Small-to-medium CLI projects** with 5-10 commands
- **Rapid prototyping** where requirements may change
- **Solo or small team** where coordination overhead is low
- **When the domain is simple** (commands are the natural unit of work)
- **When you value "working code" over "elegant architecture"**

### When to Consider Alternatives

- **Large CLIs** with 20+ commands sharing significant logic
- **Plugin architectures** where extensibility is paramount
- **When config becomes complex** (multiple sources, validation, hot-reload)
- **When multiple binaries share the same core** (library + CLI pattern)
