package utils

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v53/github"
	"golang.org/x/oauth2"
)

// FetchGitHubRepo fetches the contents of a GitHub repository and writes it to a file.
func FetchGitHubRepo(ctx context.Context, owner, repo, filePath string) error {
	// Create a new GitHub client
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// Get the file content from the repository
	fileContent, _, _, err := client.Repositories.GetContents(ctx, owner, repo, filePath, nil)
	if err != nil {
		return fmt.Errorf("error fetching file from GitHub: %w", err)
	}

	// Decode the content
	content, err := fileContent.GetContent()
	if err != nil {
		return fmt.Errorf("error decoding file content: %w", err)
	}

	// Write the content to a local file
	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("error writing file to disk: %w", err)
	}

	return nil
}

// FetchNxTemplate downloads an entire Nx workspace template
func FetchNxTemplate(ctx context.Context, owner, repo, branch string, destPath string) error {
	// Download repository as ZIP archive
	url := fmt.Sprintf("https://github.com/%s/%s/archive/refs/heads/%s.zip", owner, repo, branch)

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download repository: %w", err)
	}
	defer resp.Body.Close()

	// Save and extract the ZIP file
	return extractZipArchive(resp.Body, destPath)
}

// extractZipArchive saves and extracts a ZIP archive from the response body to the destination path.
func extractZipArchive(reader io.Reader, destPath string) error {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "*.zip")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name()) // Clean up

	// Write the ZIP file to the temporary location
	_, err = io.Copy(tmpFile, reader)
	if err != nil {
		return fmt.Errorf("failed to save ZIP file: %w", err)
	}

	// Close the temporary file
	err = tmpFile.Close()
	if err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	// Open the ZIP file
	zipReader, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		return fmt.Errorf("failed to open ZIP file: %w", err)
	}
	defer zipReader.Close()

	// Extract each file in the ZIP archive
	for _, file := range zipReader.File {
		err := extractFile(file, destPath)
		if err != nil {
			return err
		}
	}

	return nil
}

// extractFile extracts a single file from the ZIP archive.
func extractFile(file *zip.File, destPath string) error {
	// Open the file inside the ZIP archive
	rc, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open file in ZIP archive: %w", err)
	}
	defer rc.Close()

	// Create the destination file path
	destFilePath := filepath.Join(destPath, file.Name)

	// Ensure the directory exists
	err = os.MkdirAll(filepath.Dir(destFilePath), 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create the destination file
	destFile, err := os.Create(destFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer destFile.Close()

	// Copy the content to the destination file
	_, err = io.Copy(destFile, rc)
	if err != nil {
		return fmt.Errorf("failed to extract file: %w", err)
	}

	return nil
}

// ConfigureReactApp customizes the downloaded Nx workspace for React
func ConfigureReactApp(workspacePath, appName string) error {
	// Update package.json with new app name
	err := updatePackageJSON(workspacePath, appName)
	if err != nil {
		return fmt.Errorf("failed to update package.json: %w", err)
	}

	// Update workspace configuration
	err = updateWorkspaceConfig(workspacePath, appName)
	if err != nil {
		return fmt.Errorf("failed to update workspace config: %w", err)
	}

	return nil
}

func updatePackageJSON(workspacePath, appName string) error {
	packageJSONPath := filepath.Join(workspacePath, "package.json")

	// Read existing package.json
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return err
	}

	var packageJSON map[string]interface{}
	err = json.Unmarshal(data, &packageJSON)
	if err != nil {
		return err
	}

	// Update name
	packageJSON["name"] = appName

	// Write back to file
	updatedData, err := json.MarshalIndent(packageJSON, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(packageJSONPath, updatedData, 0644)
}

func updateWorkspaceConfig(workspacePath, appName string) error {
	// Update nx.json configuration
	err := updateNxJson(workspacePath, appName)
	if err != nil {
		return fmt.Errorf("failed to update nx.json: %w", err)
	}

	// Update workspace.json if it exists (older Nx versions)
	workspaceJsonPath := filepath.Join(workspacePath, "workspace.json")
	if _, err := os.Stat(workspaceJsonPath); err == nil {
		err = updateWorkspaceJson(workspacePath, appName)
		if err != nil {
			return fmt.Errorf("failed to update workspace.json: %w", err)
		}
	}

	// Update project.json files for the apps
	err = updateProjectConfigs(workspacePath, appName)
	if err != nil {
		return fmt.Errorf("failed to update project configs: %w", err)
	}

	return nil
}

func updateNxJson(workspacePath, appName string) error {
	nxJsonPath := filepath.Join(workspacePath, "nx.json")

	// Read existing nx.json
	data, err := os.ReadFile(nxJsonPath)
	if err != nil {
		return err
	}

	var nxConfig map[string]interface{}
	err = json.Unmarshal(data, &nxConfig)
	if err != nil {
		return err
	}

	// Update default project if it exists
	if _, exists := nxConfig["defaultProject"]; exists {
		nxConfig["defaultProject"] = appName
	}

	// Ensure proper task runners and plugins for React
	if nxConfig["plugins"] == nil {
		nxConfig["plugins"] = []interface{}{}
	}

	plugins := nxConfig["plugins"].([]interface{})
	hasReactPlugin := false
	for _, plugin := range plugins {
		if pluginStr, ok := plugin.(string); ok && strings.Contains(pluginStr, "react") {
			hasReactPlugin = true
			break
		}
	}

	if !hasReactPlugin {
		nxConfig["plugins"] = append(plugins, "@nx/react/plugin")
	}

	// Write back to file
	updatedData, err := json.MarshalIndent(nxConfig, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(nxJsonPath, updatedData, 0644)
}

func updateWorkspaceJson(workspacePath, appName string) error {
	workspaceJsonPath := filepath.Join(workspacePath, "workspace.json")

	data, err := os.ReadFile(workspaceJsonPath)
	if err != nil {
		return err
	}

	var workspaceConfig map[string]interface{}
	err = json.Unmarshal(data, &workspaceConfig)
	if err != nil {
		return err
	}

	// Update default project
	workspaceConfig["defaultProject"] = appName

	// Write back to file
	updatedData, err := json.MarshalIndent(workspaceConfig, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(workspaceJsonPath, updatedData, 0644)
}

func updateProjectConfigs(workspacePath, appName string) error {
	// Find and update project.json files in apps directory
	appsDir := filepath.Join(workspacePath, "apps")

	entries, err := os.ReadDir(appsDir)
	if err != nil {
		// Apps directory might not exist in some templates
		return nil
	}

	for _, entry := range entries {
		if entry.IsDir() {
			projectJsonPath := filepath.Join(appsDir, entry.Name(), "project.json")
			if _, err := os.Stat(projectJsonPath); err == nil {
				err = updateSingleProjectJson(projectJsonPath, appName, entry.Name())
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func updateSingleProjectJson(projectJsonPath, appName, projectDir string) error {
	data, err := os.ReadFile(projectJsonPath)
	if err != nil {
		return err
	}

	var projectConfig map[string]interface{}
	err = json.Unmarshal(data, &projectConfig)
	if err != nil {
		return err
	}

	// Update project name
	projectConfig["name"] = fmt.Sprintf("%s-%s", appName, projectDir)

	// Update sourceRoot if it exists
	if sourceRoot, exists := projectConfig["sourceRoot"]; exists {
		if sourceRootStr, ok := sourceRoot.(string); ok {
			// Replace any template placeholders with actual app name
			updatedSourceRoot := strings.ReplaceAll(sourceRootStr, "my-app", appName)
			projectConfig["sourceRoot"] = updatedSourceRoot
		}
	}

	// Update targets that might reference the app name
	if targets, exists := projectConfig["targets"]; exists {
		if targetsMap, ok := targets.(map[string]interface{}); ok {
			updateTargetConfigs(targetsMap, appName)
		}
	}

	// Write back to file
	updatedData, err := json.MarshalIndent(projectConfig, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(projectJsonPath, updatedData, 0644)
}

func updateTargetConfigs(targets map[string]interface{}, appName string) {
	for _, target := range targets {
		if targetMap, ok := target.(map[string]interface{}); ok {
			// Update options that might contain app-specific paths
			if options, exists := targetMap["options"]; exists {
				if optionsMap, ok := options.(map[string]interface{}); ok {
					updateOptionsMap(optionsMap, appName)
				}
			}

			// Update configurations
			if configurations, exists := targetMap["configurations"]; exists {
				if configMap, ok := configurations.(map[string]interface{}); ok {
					for _, config := range configMap {
						if configOptions, ok := config.(map[string]interface{}); ok {
							updateOptionsMap(configOptions, appName)
						}
					}
				}
			}
		}
	}
}

func updateOptionsMap(options map[string]interface{}, appName string) {
	// Update common paths that might reference the app name
	pathFields := []string{"outputPath", "main", "polyfills", "tsConfig", "index"}

	for _, field := range pathFields {
		if value, exists := options[field]; exists {
			if valueStr, ok := value.(string); ok {
				// Replace template app names with actual app name
				updated := replaceTemplateName(valueStr, appName)
				options[field] = updated
			}
		}
	}
}

func replaceTemplateName(path, appName string) string {
	// Common template names to replace
	templateNames := []string{"my-app", "myapp", "template-app", "nx-app"}

	result := path
	for _, templateName := range templateNames {
		result = strings.ReplaceAll(result, templateName, appName)
	}

	return result
}

// Example usage:
// func main() {
// 	ctx := context.Background()
// 	err := FetchGitHubRepo(ctx, "owner", "repo", "path/to/file.txt")
// 	if err != nil {
// 		fmt.Printf("Error fetching GitHub repo: %v\n", err)
// 	} else {
// 		fmt.Println("File fetched and written successfully.")
// 	}
