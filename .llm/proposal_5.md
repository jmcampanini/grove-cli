
# Plan

Approach the workflow as a UX-focused command suite: add a `create` command, a `shell` generator, and a `doctor`/`explain` command to surface config resolution and workspace assumptions. This emphasizes discoverability and reliability for a complex workflow.

## Pros
- Best developer experience with clear diagnostics.
- Easier onboarding through `doctor`/`explain` output.
- Reduces support burden by surfacing config resolution details.

## Cons
- Largest scope and more commands to maintain.
- Higher risk of scope creep before core workflow ships.
- Additional tests and docs required to keep UX coherent.

## Requirements
- Use Cobra and BurntSushi TOML.
- Implement `create` for branch/worktree creation plus friendly output.
- Generate shell functions for fish/zsh/bash with zoxide fallback.
- Add a diagnostics command to show config resolution and naming previews.

## Scope
- In: create command, shell generation, config resolver, diagnostics command.
- Out: CLI/env overrides (not yet), advanced templating for naming.

## Files and entry points
- `main.go` (root command, persistent flags).
- New: `internal/config/config.go` (model + loader).
- New: `internal/cmd/create.go`, `internal/cmd/shell.go`, `internal/cmd/doctor.go`.
- `internal/git/git.go`, `internal/git/git_cli.go` (worktree/branch ops).
- `internal/naming/slugify.go` (slug config).

## Data model / API changes
- Config structs for `slugify`, `branch`, `worktree`.
- Diagnostics output includes chosen config path and computed names.

## Action items
[ ] Implement config loader with the specified search order and return the source path.
[ ] Implement `grc create <phrase>` with clear output: branch name, worktree dir, and next steps.
[ ] Implement `grc shell --shell {bash|zsh|fish}` to emit functions and document usage.
[ ] Implement `grc doctor` (or `grc explain`) to print config source and preview generated names for a sample phrase.
[ ] Add tests for config loading and naming preview; add minimal CLI tests for `doctor` output.

## Testing and validation
- `go test ./...`.
- Manual: `grc doctor` inside/outside repo to validate messaging.
- Manual: `source <(grc shell --shell fish)` and run `grc` function.

## Risks and edge cases
- Adding `doctor` increases surface area; may be postponed if time is tight.
- Confusion between `grc` binary and function name; require explicit docs.
- Worktree naming collisions need clear errors and remediation advice.

## Open questions
- Should `doctor` accept `--phrase` to preview naming deterministically?
- Should the function name include command name or be versioned to avoid upgrades breaking shells?
