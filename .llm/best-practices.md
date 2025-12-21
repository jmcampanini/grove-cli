# Best Practices Reference

This document contains best practices to follow during implementation of the grove-cli. Reference this document when implementing each phase.

---

## Go Best Practices

Based on Effective Go and the Google Go Style Guide.

### Error Handling

1. **Idiomatic Error Checking:**
   - Use the pattern: `if err := doSomething(); err != nil { ... }`
   - This keeps the error handling close to the operation and limits variable scope

2. **Error Types:**
   - Return the `error` interface type, not concrete error types
   - Avoid returning specific types like `*os.PathError` in function signatures
   - This allows implementation flexibility and better abstraction

3. **Success Returns:**
   - Return `nil` explicitly for success cases, not a typed nil pointer
   - Example: `return nil` instead of `return (*MyError)(nil)`

### Testing

1. **Table-Driven Tests:**
   - Use table-driven tests for functions with multiple input/output combinations
   - This pattern is especially useful for naming generators, config merging, and validation logic
   - Example structure:
     ```go
     tests := []struct {
         name    string
         input   string
         want    string
         wantErr bool
     }{
         {name: "simple case", input: "foo", want: "bar", wantErr: false},
         // ... more cases
     }
     for _, tt := range tests {
         t.Run(tt.name, func(t *testing.T) {
             got, err := functionUnderTest(tt.input)
             if (err != nil) != tt.wantErr {
                 t.Errorf("unexpected error state")
             }
             if got != tt.want {
                 t.Errorf("got %v, want %v", got, tt.want)
             }
         })
     }
     ```

2. **Test Helpers:**
   - Test helpers should call `t.Helper()` to improve error reporting
   - This ensures test failures point to the actual test case, not the helper

3. **Error Testing Pattern:**
   - Use `wantErr bool` field pattern for testing error presence
   - Don't compare error strings; check for error existence or use errors.Is/errors.As

4. **Test Doubles:**
   - Create fake implementations of interfaces for testing (test doubles)
   - Example: FileSystem interface in config loader enables testing without real files

5. **Goroutine Testing:**
   - Don't use `t.Fatal` in goroutines - use `t.Errorf` instead
   - `t.Fatal` calls runtime.Goexit which only stops the current goroutine

### Interfaces

1. **Interface Location:**
   - Define interfaces where they are used (consumer side), not where implemented
   - This is the "interface segregation" principle
   - Example: `Loader` defines the `FileSystem` interface it needs

2. **Testability:**
   - Use interface types for dependencies to enable testability
   - Allows injecting test doubles without modifying production code

---

## Cobra CLI Best Practices

For command layer implementation (Phase 3).

### Command Design

1. **Use `RunE` instead of `Run`:**
   - Always use `RunE` to return errors from command execution
   - This allows proper error handling by the caller
   - Cobra automatically displays errors with "Error:" prefix
   - Example:
     ```go
     RunE: func(cmd *cobra.Command, args []string) error {
         if err := doSomething(); err != nil {
             return fmt.Errorf("failed to do something: %w", err)
         }
         return nil
     }
     ```

2. **Argument Validation:**
   - Use built-in validators in the `Args` field:
     - `cobra.ExactArgs(n)` - require exactly n arguments
     - `cobra.MinimumNArgs(n)` - require at least n arguments
     - `cobra.MaximumNArgs(n)` - require at most n arguments
     - `cobra.RangeArgs(min, max)` - require between min and max arguments
     - `cobra.MatchAll(v1, v2)` - combine multiple validators
   - Custom `Args` function for complex validation:
     ```go
     Args: func(cmd *cobra.Command, args []string) error {
         if len(args) < 1 {
             return errors.New("requires a phrase argument")
         }
         if strings.TrimSpace(args[0]) == "" {
             return errors.New("phrase cannot be empty")
         }
         return nil
     }
     ```

3. **Command Structure:**
   - Root command defines the application and adds subcommands via `AddCommand()`
   - Each subcommand handles a specific action
   - Group related commands for better help output using command annotations
   - Keep command logic in `RunE` functions; extract complex logic to internal packages

4. **PreRunE/PostRunE Hooks:**
   - Use for setup (loading config) and cleanup operations
   - `PersistentPreRunE` is inherited by all subcommands
   - Example:
     ```go
     PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
         // Load config once for all commands
         cfg, err := config.Load()
         if err != nil {
             return fmt.Errorf("failed to load config: %w", err)
         }
         cmd.SetContext(context.WithValue(cmd.Context(), "config", cfg))
         return nil
     }
     ```

5. **Shell Completions:**
   - Define `ValidArgs` for simple static completions
   - Use `ValidArgsFunction` for dynamic completions (file paths, git branches, etc.)
   - Enable completion command generation:
     ```go
     ValidArgs: []string{"fish", "zsh", "bash"},
     ```
   - Note: Shell completion support will be added in future iterations

6. **Error Messages:**
   - Errors returned from `RunE` are automatically displayed with "Error:" prefix
   - Include context in error messages using `fmt.Errorf` with `%w`
   - Provide actionable error messages that suggest solutions
   - Example: `return fmt.Errorf("branch %q already exists: use --force to override", branchName)`

7. **Command Help:**
   - Use `Short` for one-line description in command list
   - Use `Long` for detailed usage in `command --help`
   - Provide examples in `Long` using the `Example:` section format
