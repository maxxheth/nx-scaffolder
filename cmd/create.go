package cmd

import (
	"context"
	"fmt"
	"path/filepath"

	"nx-scaffolder/internal/utils"

	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create [app-name]",
	Short: "Create a new Nx React workspace",
	Long:  "Downloads an Nx template repository and configures it for a React application",
	Args:  cobra.ExactArgs(1),
	RunE:  runCreate,
}

var (
	owner    string
	repo     string
	branch   string
	template string
)

func init() {
	rootCmd.AddCommand(createCmd)

	createCmd.Flags().StringVarP(&owner, "owner", "", "nrwl", "GitHub repository owner")
	createCmd.Flags().StringVarP(&repo, "repo", "r", "nx", "GitHub repository name")
	createCmd.Flags().StringVarP(&branch, "branch", "b", "master", "Git branch to download")
	createCmd.Flags().StringVarP(&template, "template", "t", "react", "Template type (react, angular, etc.)")
}

func runCreate(cmd *cobra.Command, args []string) error {
	appName := args[0]
	outputDir, _ := cmd.Flags().GetString("output")

	ctx := context.Background()
	destPath := filepath.Join(outputDir, appName)

	fmt.Printf("Creating Nx React workspace '%s'...\n", appName)
	fmt.Printf("Downloading from %s/%s (branch: %s)\n", owner, repo, branch)

	// Download the Nx template
	err := utils.FetchNxTemplate(ctx, owner, repo, branch, destPath)
	if err != nil {
		return fmt.Errorf("failed to download template: %w", err)
	}

	fmt.Printf("Template downloaded to: %s\n", destPath)

	// Configure the React app
	err = utils.ConfigureReactApp(destPath, appName)
	if err != nil {
		return fmt.Errorf("failed to configure React app: %w", err)
	}

	fmt.Printf("✅ Successfully created Nx React workspace '%s'\n", appName)
	return nil
}
