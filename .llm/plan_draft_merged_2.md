---
name: plan-draft-merged-2
description: Merged plan draft incorporating proposal improvements
---

# Plan

Refine the draft into an implementation plan that delivers `grc <phrase>` end-to-end using existing `internal/git` and `internal/naming`, adds a config loader with the specified search order, and emits shell functions for fish/zsh/bash with zoxide-aware directory switching.

## Requirements
- Use Cobra for CLI structure and BurntSushi TOML for config parsing.
- Implement `create` to slugify input, name branch/worktree via config, create branch + worktree, and print the worktree path.
- Implement config discovery order: CWD file -> git root file -> parent dirs up to home/root -> XDG config path.
- Emit shell functions for fish/zsh/bash that `cd` (or `z` if available) into the created worktree.
- Build on existing `internal/git` and `internal/naming` packages.

## Scope
- In: config model + loader, `create` command, shell init command, minimal naming helpers for worktree name derivation.
- Out: CLI/env overrides, diagnostics/doctor command, advanced templating or config validation UX.

## Files and entry points
- `main.go` (wire Cobra root and subcommands).
- `cmd/root.go` (root Cobra command).
- `cmd/create.go` (create command).
- `cmd/init.go` (shell init output).
- `internal/config/config.go` (TOML model + defaults).
- `internal/config/loader.go` (discovery + merge logic).
- `internal/naming/branch.go` (branch name generation).
- `internal/naming/worktree.go` (worktree name derivation).
- `internal/git/*` (use existing helpers; extend only if required).

## Data model / API changes
- Config structs: `SlugifyConfig`, `BranchConfig`, `WorktreeConfig`.
- Config loader returns merged config with defaults as base; higher priority files override lower.
- Worktree naming uses `strip_branch_prefix` and `new_prefix`.

## Action items
[ ] Define config structs and defaults aligned with the spec example (`slugify`, `branch`, `worktree`).
[ ] Implement config discovery: gather candidate paths in priority order and merge all that exist.
[ ] Implement branch name generator using `internal/naming.Slugify` + branch prefix.
[ ] Implement worktree name generator that strips configured branch prefixes then applies worktree prefix.
[ ] Implement `create` command: join phrase, load config, generate names, compute worktree path relative to main worktree, call git helper, print path.
[ ] Implement `init` command to output shell-specific functions and a stable, extendable naming pattern (e.g., internal `__grove_create`, user-facing `grc`).
[ ] Add tests for config merge order, branch/worktree naming, and basic command behavior (unit tests where possible, minimal integration tests for git).

## Testing and validation
- `go test ./...`.
- Config loader tests with temp files to validate precedence.
- Manual: run `grove create "my feature"` and confirm branch/worktree names match config.
- Manual: `eval "$(grove init zsh)"` (or fish/bash equivalent) and verify directory change with and without `z`.

## Risks and edge cases
- Not in a git repo: must return a clear error before attempting worktree creation.
- Branch/worktree collisions: detect and provide actionable error messages.
- Config parsing errors: report file path and line/column when possible.
- Shell function naming collisions: ensure unique prefixing and document usage.

## Open questions
- Should `create` accept optional base ref flags now or later?
- Should the `init` command be named `init` or `shell` for consistency with future commands?
