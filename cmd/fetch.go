package cmd

import (
	"context"
	"fmt"

	"nx-scaffolder/internal/utils"

	"github.com/spf13/cobra"
)

var fetchCmd = &cobra.Command{
	Use:   "fetch [owner] [repo] [file-path]",
	Short: "Fetch a specific file from a GitHub repository",
	Long:  "Downloads a single file from a GitHub repository using the GitHub API",
	Args:  cobra.ExactArgs(3),
	RunE:  runFetch,
}

func init() {
	rootCmd.AddCommand(fetchCmd)
}

func runFetch(cmd *cobra.Command, args []string) error {
	owner := args[0]
	repo := args[1]
	filePath := args[2]

	ctx := context.Background()

	fmt.Printf("Fetching %s from %s/%s...\n", filePath, owner, repo)

	err := utils.FetchGitHubRepo(ctx, owner, repo, filePath)
	if err != nil {
		return fmt.Errorf("failed to fetch file: %w", err)
	}

	fmt.Printf("✅ Successfully fetched %s\n", filePath)
	return nil
}
