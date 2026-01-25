package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/user/edi/internal/config"
	"gopkg.in/yaml.v3"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage EDI configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show merged configuration",
	RunE:  runConfigShow,
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Open configuration in editor",
	RunE:  runConfigEdit,
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show configuration file paths",
	Run:   runConfigPath,
}

func init() {
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configEditCmd)
	configCmd.AddCommand(configPathCmd)

	configEditCmd.Flags().Bool("global", false, "Edit global config")
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	fmt.Println("# Merged configuration (global + project)")
	fmt.Println(string(data))
	return nil
}

func runConfigEdit(cmd *cobra.Command, args []string) error {
	global, _ := cmd.Flags().GetBool("global")

	var path string
	if global {
		path = config.GlobalConfigPath()
	} else {
		path = config.ProjectConfigPath()
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	c := exec.Command(editor, path)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	return c.Run()
}

func runConfigPath(cmd *cobra.Command, args []string) {
	fmt.Printf("Global:  %s\n", config.GlobalConfigPath())
	fmt.Printf("Project: %s\n", config.ProjectConfigPath())
}
