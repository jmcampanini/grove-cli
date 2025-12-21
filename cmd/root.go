package cmd

import "github.com/spf13/cobra"

// Version is set at build time via ldflags.
var Version = "n/a"

var rootCmd = &cobra.Command{
	Use:   "grove",
	Short: "Git worktree workspace manager",
	Long:  `Grove manages git worktrees in a workspace structure.`,
}

func init() {
	rootCmd.Version = Version
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
