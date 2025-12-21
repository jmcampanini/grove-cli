package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use:   "grove",
	Short: "Git worktree workspace manager",
	Long:  `Grove manages git worktrees in a workspace structure.`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
