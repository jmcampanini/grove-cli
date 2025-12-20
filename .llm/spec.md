i want to type in

```
grc the name of a feature im working on
```

and that will:

1. create a branch with the naming convention defined in my config
2. create a worktree with the naming convention defined in config using the branch
3. switch the current working directory to it, using zoxide if available

this will roughly require:

1. a `create` command that takes a phrase and creates the branch and config
2. a config that defines how to name branches and worktrees
3. the ability to load configs in a reasonable way
4. shell function generation to make a simple function for a user to alias/use for the example

---

build on top of:

1. what currently exists in internal/git and internal/naming
2. use cobra cli as the library for implementing cli commands
3. use burntsushi toml for the toml loading

---

creating a branch

inputs necessary:

- slug config
- branch naming config
  - prefix
- worktree naming config
  - prefix
  - branch strip prefix

---

how does config loading work

highest priority:

- eventually, but not yet: overrides from cli (including env vars)
- file in cwd
- file in git root tree dir
- file in dirs all the way up to user home or root
- file in XDG config dir/grove/grove.toml

---

config example

```toml
[slugify]
max_length = 20
hash_length = 4
lowercase = true
replace_non_alphanum = true
collapse_dashes = true
trim_dashes = true

[branch]
new_prefix = "feature/"

[worktree]
new_prefix = "wt-"
strip_branch_prefix = [ "feature/" ]
```

---

the shell function thats created should work on fish, zsh, and bash

it should change directory using zoxide if it is detected

name it something that will be unique and canonical for each shell. we will be creating more functions so make the
naming pattern extendable for more functionality as we add it.

the user should be required to run source on a command in grove that will output shell compliant functions