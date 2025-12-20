
# Plan

Design a config-first architecture that isolates configuration resolution and git operations behind interfaces, making the complex workflow testable and extensible. This favors correctness and future CLI override support over speed of delivery.

## Pros
- Strong testability for config resolution and git interactions.
- Clear separation of concerns for future CLI/env overrides.
- Easier to add new commands without duplicating config logic.

## Cons
- Higher upfront complexity and refactor cost.
- More files and interfaces to maintain.
- Slower to deliver the first working version.

## Requirements
- Use Cobra for CLI and BurntSushi TOML for config parsing.
- Implement explicit config resolution layers with the specified search order.
- Provide a `create` command that uses a resolved config object and a git abstraction.
- Provide a `shell` command to emit portable fish/zsh/bash functions with zoxide support.

## Scope
- In: config resolver module, git interface + adapter, `create` and `shell` commands, tests for config discovery and naming.
- Out: CLI/env overrides (stub the interface), workspace auto-detection improvements beyond git root.

## Files and entry points
- `main.go` (wire Cobra root + dependencies).
- New: `internal/config/loader.go`, `internal/config/model.go`, `internal/config/resolver.go`.
- New: `internal/git/adapter.go` (interface + CLI adapter).
- New: `internal/cmd/create.go`, `internal/cmd/shell.go`.
- `internal/naming/slugify.go` (use config-driven options).

## Data model / API changes
- Config model with `Slugify`, `Branch`, `Worktree` sections.
- Resolver returns `(config, sourcePath)` to aid debugging and future UX.
- Git interface methods: `CreateBranch`, `AddWorktree`, `CurrentRepoRoot`.

## Action items
[ ] Implement config model + TOML parse, plus resolver that walks: CWD → git root → parents → XDG.
[ ] Add git interface and adapter wrapping existing `internal/git` CLI helpers.
[ ] Build `create` command using injected config + git adapter; ensure slugify and name generation lives in a dedicated naming service.
[ ] Build `shell` command to output functions per shell with zoxide detection; define stable naming pattern (e.g., `__grove_fn_create_<shell>`).
[ ] Add unit tests for resolver precedence, naming output, and git adapter invocation.
[ ] Add integration test for `create` using a temp repo + worktree.

## Testing and validation
- `go test ./...`.
- Config resolver tests with temp directories and dummy TOML files.
- Integration: create branch/worktree in a temp git repo to validate CLI behavior.

## Risks and edge cases
- Resolver performance walking directories; ensure short-circuit on first match.
- Git adapter may differ from real git behavior; integration tests mitigate.
- Template drift for shell output across shells.

## Open questions
- Should we expose `grc config print` or `grc config which` to debug resolution?
- Where should default config live (embedded or absent)?
