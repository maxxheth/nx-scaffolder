package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"nx-scaffolder/internal/utils"

	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create [workspace-name]",
	Short: "Create a new Nx React monorepo workspace",
	Long:  "Creates an Nx monorepo and configures multiple React applications from existing repos or new apps",
	Args:  cobra.ExactArgs(1),
	RunE:  runCreate,
}

var (
	owner    string
	repo     string
	branch   string
	template string
	inject   string
)

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringVarP(&owner, "owner", "", "nrwl", "GitHub repository owner")
	createCmd.Flags().StringVarP(&repo, "repo", "r", "nx", "GitHub repository name")
	createCmd.Flags().StringVarP(&branch, "branch", "b", "master", "Git branch to download")
	createCmd.Flags().StringVarP(&template, "template", "t", "react", "Template type (react, angular, etc.)")
	createCmd.Flags().StringVarP(&inject, "inject", "i", "", "Pipe-delimited list of repos to inject or {create-new} expressions")
}

func runCreate(cmd *cobra.Command, args []string) error {
	workspaceName := args[0]
	outputDir, _ := cmd.Flags().GetString("output")

	ctx := context.Background()
	destPath := filepath.Join(outputDir, workspaceName)

	fmt.Printf("Creating Nx React monorepo '%s'...\n", workspaceName)

	// Create base Nx workspace
	fmt.Printf("Downloading base template from %s/%s (branch: %s)\n", owner, repo, branch)
	err := utils.FetchNxTemplate(ctx, owner, repo, branch, destPath)
	if err != nil {
		return fmt.Errorf("failed to download base template: %w", err)
	}

	// Configure base workspace
	err = utils.ConfigureMonorepo(destPath, workspaceName)
	if err != nil {
		return fmt.Errorf("failed to configure base workspace: %w", err)
	}

	// Process injection instructions
	if inject != "" {
		instructions, err := parseInjectInstructions(inject)
		if err != nil {
			return fmt.Errorf("failed to parse inject instructions: %w", err)
		}

		err = utils.ProcessInjectionInstructions(ctx, destPath, instructions)
		if err != nil {
			return fmt.Errorf("failed to process injection instructions: %w", err)
		}
	}

	fmt.Printf("✅ Successfully created Nx React monorepo '%s'\n", workspaceName)
	return nil
}

// parseInjectInstructions parses the inject string and returns a list of instructions
func parseInjectInstructions(injectStr string) ([]utils.InjectionInstruction, error) {
	parts := strings.Split(injectStr, "|")
	var instructions []utils.InjectionInstruction

	createNewRegex := regexp.MustCompile(`^{create-new([+*])(\d+)}$`)

	for i, part := range parts {
		part = strings.TrimSpace(part)

		if part == "{create-new}" {
			instructions = append(instructions, utils.InjectionInstruction{
				Type:    "create-new",
				AppName: fmt.Sprintf("app-%d", i+1),
			})
		} else if matches := createNewRegex.FindStringSubmatch(part); matches != nil {
			operator := matches[1]
			count, err := strconv.Atoi(matches[2])
			if err != nil {
				return nil, fmt.Errorf("invalid number in expression %s: %w", part, err)
			}

			var numApps int
			switch operator {
			case "+":
				numApps = count + 1 // +5 means create 6 apps total
			case "*":
				numApps = count // *3 means create 3 apps
			default:
				return nil, fmt.Errorf("unsupported operator %s in expression %s", operator, part)
			}

			for range numApps {
				instructions = append(instructions, utils.InjectionInstruction{
					Type:    "create-new",
					AppName: fmt.Sprintf("app-%d", len(instructions)+1),
				})
			}
		} else if strings.HasPrefix(part, "http") {
			// Extract repo name from URL for app naming
			appName := extractRepoName(part)
			if appName == "" {
				appName = fmt.Sprintf("imported-app-%d", i+1)
			}

			instructions = append(instructions, utils.InjectionInstruction{
				Type:    "import-repo",
				RepoURL: part,
				AppName: appName,
			})
		} else {
			return nil, fmt.Errorf("invalid injection instruction: %s", part)
		}
	}

	return instructions, nil
}

// extractRepoName extracts repository name from GitHub URL
func extractRepoName(url string) string {
	// Handle GitHub URLs like https://github.com/owner/repo or https://github.com/owner/repo.git
	urlRegex := regexp.MustCompile(`github\.com/[^/]+/([^/\.]+)`)
	matches := urlRegex.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
