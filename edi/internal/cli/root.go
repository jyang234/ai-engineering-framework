package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	verbose bool
	rootCmd *cobra.Command
)

func init() {
	rootCmd = &cobra.Command{
		Use:   "edi",
		Short: "EDI - Enhanced Development Intelligence",
		Long: `EDI is a harness for Claude Code that provides continuity, knowledge, and specialized behaviors.

It configures Claude Code with context, agents, and RECALL knowledge retrieval before launching.`,
		RunE:          runLaunch, // Default action is launch
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
}

// Execute runs the root command
func Execute(version string) error {
	// Add subcommands here to ensure proper initialization order
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(recallServerCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(recallCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(ralphCmd)

	rootCmd.Version = version
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return err
	}
	return nil
}
