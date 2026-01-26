package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/anthropics/aef/edi/internal/agents"
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available agents",
	RunE:  runAgentList,
}

var agentShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show agent details",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentShow,
}

func init() {
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentShowCmd)
}

func runAgentList(cmd *cobra.Command, args []string) error {
	agentNames, err := agents.ListAgents()
	if err != nil {
		return err
	}

	if len(agentNames) == 0 {
		fmt.Println("No agents found. Run 'edi init --global' to install default agents.")
		return nil
	}

	fmt.Println("Available Agents:")
	fmt.Println()

	for _, name := range agentNames {
		agent, err := agents.Load(name)
		if err != nil {
			fmt.Printf("  %-12s  (error loading)\n", name)
			continue
		}
		fmt.Printf("  %-12s  %s\n", name, agent.Description)
	}

	fmt.Println()
	fmt.Println("Use 'edi agent show <name>' for details.")

	return nil
}

func runAgentShow(cmd *cobra.Command, args []string) error {
	name := args[0]

	agent, err := agents.Load(name)
	if err != nil {
		return fmt.Errorf("agent not found: %s", name)
	}

	fmt.Printf("Agent: %s\n", agent.Name)
	fmt.Printf("Description: %s\n", agent.Description)
	fmt.Println()

	if len(agent.Tools) > 0 {
		fmt.Println("Tools:")
		for _, tool := range agent.Tools {
			fmt.Printf("  - %s\n", tool)
		}
		fmt.Println()
	}

	if len(agent.Skills) > 0 {
		fmt.Println("Skills:")
		for _, skill := range agent.Skills {
			fmt.Printf("  - %s\n", skill)
		}
		fmt.Println()
	}

	// Show location
	home, _ := os.UserHomeDir()
	cwd, _ := os.Getwd()

	projectPath := filepath.Join(cwd, ".edi", "agents", name+".md")
	globalPath := filepath.Join(home, ".edi", "agents", name+".md")

	if _, err := os.Stat(projectPath); err == nil {
		fmt.Printf("Location: %s (project override)\n", projectPath)
	} else if _, err := os.Stat(globalPath); err == nil {
		fmt.Printf("Location: %s\n", globalPath)
	}

	fmt.Println()
	fmt.Println("System Prompt:")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println(agent.SystemPrompt)

	return nil
}
