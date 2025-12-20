
# Plan

Implement a minimal, incremental workflow that adds a `create` command and shell-function output while reusing `internal/git` and `internal/naming`. Focus on shipping the core `grc <feature>` flow with small, localized changes.

## Pros
- Fastest path to a working end-to-end flow.
- Minimal architectural changes; low risk of regressions.
- Easy to review and merge in small PRs.

## Cons
- Config loading logic may be tightly coupled to commands.
- Harder to extend when CLI/env overrides are added later.
- Less testability around config resolution and git interactions.

## Requirements
- Use Cobra for CLI structure and BurntSushi TOML for config loading.
- Implement `create` to slugify, name branch/worktree via config, create branch, add worktree, and output the path.
- Generate shell functions for fish/zsh/bash that `cd` (or `zoxide`) into the new worktree.
- Implement config search order as specified.

## Scope
- In: config loader, `create` command, shell function output command, updates to `internal/git` and `internal/naming` as needed.
- Out: CLI/env overrides (not yet), advanced config validation UI.

## Files and entry points
- `main.go` (wire Cobra root and subcommands).
- `internal/git/git.go`, `internal/git/git_cli.go` (branch/worktree helpers).
- `internal/naming/slugify.go` (slug config usage).
- New: `internal/config/config.go` (TOML model + load/search).
- New: `internal/cmd/create.go`, `internal/cmd/shell.go` (commands).

## Data model / API changes
- New config structs: `SlugifyConfig`, `BranchConfig`, `WorktreeConfig`.
- Loader returns merged config from first-found file in search order.

## Action items
[ ] Add `internal/config` with TOML structs and a `Load()` that searches CWD → git root → parents → XDG.
[ ] Implement `create` Cobra command to slugify input, build branch/worktree names, call git helpers, and print worktree path.
[ ] Extend `internal/git` helpers for branch create + worktree add as needed (CLI wrapper).
[ ] Add `shell` Cobra command that prints per-shell functions (with zoxide detection), naming them `__grove_<shell>_create`.
[ ] Wire Cobra in `main.go` and add basic error handling and usage text.
[ ] Add lightweight tests for config load order and slugify application.

## Testing and validation
- Run existing Go tests.
- Add unit tests for config loader path precedence.
- Manual: run `grc create "my feature"` and ensure branch/worktree names match config.
- Manual: `grc shell --shell zsh` then `source <(grc shell --shell zsh)` and confirm directory change.

## Risks and edge cases
- Git root discovery may fail outside a repo; return clear error.
- Worktree name collision handling (existing dir or branch) must be explicit.
- Shell function naming collisions; ensure unique prefix and documented usage.

## Open questions
- Should `create` accept explicit `--shell` vs a separate `shell` command only?
- How should errors from `git worktree add` be surfaced (raw vs friendly)?
