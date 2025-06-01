package utils

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// InjectionInstruction represents a single injection operation
type InjectionInstruction struct {
	Type    string // "create-new" or "import-repo"
	RepoURL string // For import-repo type
	AppName string // Name for the app
	Branch  string // Branch to use (optional, defaults to main/master)
}

// ProcessInjectionInstructions processes all injection instructions for the monorepo
func ProcessInjectionInstructions(ctx context.Context, workspacePath string, instructions []InjectionInstruction) error {
	fmt.Printf("Processing %d injection instructions...\n", len(instructions))

	for i, instruction := range instructions {
		fmt.Printf("[%d/%d] Processing %s: %s\n", i+1, len(instructions), instruction.Type, instruction.AppName)

		switch instruction.Type {
		case "create-new":
			err := createNewReactApp(workspacePath, instruction.AppName)
			if err != nil {
				return fmt.Errorf("failed to create new React app %s: %w", instruction.AppName, err)
			}
		case "import-repo":
			err := importExistingRepo(ctx, workspacePath, instruction)
			if err != nil {
				return fmt.Errorf("failed to import repo %s: %w", instruction.RepoURL, err)
			}
		default:
			return fmt.Errorf("unknown instruction type: %s", instruction.Type)
		}
	}

	// Update workspace configuration after all apps are added
	err := updateMonorepoConfig(workspacePath, instructions)
	if err != nil {
		return fmt.Errorf("failed to update monorepo configuration: %w", err)
	}

	return nil
}

// createNewReactApp creates a new React application in the monorepo
func createNewReactApp(workspacePath, appName string) error {
	fmt.Printf("Creating new React app: %s\n", appName)

	// Check if node_modules exists, if not install dependencies first
	nodeModulesPath := filepath.Join(workspacePath, "node_modules")
	if _, err := os.Stat(nodeModulesPath); os.IsNotExist(err) {
		fmt.Println("Installing workspace dependencies...")
		installCmd := exec.Command("npm", "install")
		installCmd.Dir = workspacePath
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr

		if err := installCmd.Run(); err != nil {
			fmt.Printf("Warning: npm install failed, falling back to manual creation: %v\n", err)
			return createReactAppManually(workspacePath, appName)
		}
	}

	// Use Nx CLI to generate React application
	cmd := exec.Command("npx", "nx", "generate", "@nx/react:application", appName, "--directory=apps")
	cmd.Dir = workspacePath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		// Fallback: create basic React app structure manually
		return createReactAppManually(workspacePath, appName)
	}

	return nil
}

// createReactAppManually creates a basic React app structure when Nx CLI is not available
func createReactAppManually(workspacePath, appName string) error {
	appPath := filepath.Join(workspacePath, "apps", appName)

	// Create directory structure
	dirs := []string{
		"src",
		"src/app",
		"src/assets",
		"public",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(appPath, dir), 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create basic files
	files := map[string]string{
		"project.json":      generateProjectJSON(appName),
		"src/main.tsx":      generateMainTsx(appName),
		"src/app/app.tsx":   generateAppTsx(appName),
		"src/styles.css":    generateStylesCSS(),
		"public/index.html": generateIndexHTML(appName),
		"tsconfig.json":     generateTsConfig(appName),
		"tsconfig.app.json": generateTsConfigApp(),
		"vite.config.ts":    generateViteConfig(appName),
	}

	for filePath, content := range files {
		fullPath := filepath.Join(appPath, filePath)
		err := os.WriteFile(fullPath, []byte(content), 0644)
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", filePath, err)
		}
	}

	return nil
}

// importExistingRepo imports an existing React repository into the monorepo
func importExistingRepo(_ context.Context, workspacePath string, instruction InjectionInstruction) error {
	fmt.Printf("Importing existing repo: %s as %s\n", instruction.RepoURL, instruction.AppName)

	appPath := filepath.Join(workspacePath, "apps", instruction.AppName)

	// Clone the repository
	branch := instruction.Branch
	if branch == "" {
		branch = "main" // Default branch
	}

	// Try main first, then master
	err := cloneRepo(instruction.RepoURL, appPath, "main")
	if err != nil {
		err = cloneRepo(instruction.RepoURL, appPath, "master")
		if err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
	}

	// Remove .git directory to integrate into monorepo
	gitDir := filepath.Join(appPath, ".git")
	err = os.RemoveAll(gitDir)
	if err != nil {
		fmt.Printf("Warning: failed to remove .git directory: %v\n", err)
	}

	// Convert to Nx project structure
	err = convertToNxProject(appPath, instruction.AppName)
	if err != nil {
		return fmt.Errorf("failed to convert to Nx project: %w", err)
	}

	return nil
}

// cloneRepo clones a Git repository
func cloneRepo(repoURL, destPath, branch string) error {
	cmd := exec.Command("git", "clone", "--branch", branch, "--depth", "1", repoURL, destPath)
	return cmd.Run()
}

// convertToNxProject converts an existing React app to Nx project structure
func convertToNxProject(appPath, appName string) error {
	// Create project.json if it doesn't exist
	projectJSONPath := filepath.Join(appPath, "project.json")
	if _, err := os.Stat(projectJSONPath); os.IsNotExist(err) {
		projectJSON := generateProjectJSON(appName)
		err = os.WriteFile(projectJSONPath, []byte(projectJSON), 0644)
		if err != nil {
			return fmt.Errorf("failed to create project.json: %w", err)
		}
	}

	// Update package.json to remove scripts that conflict with Nx
	packageJSONPath := filepath.Join(appPath, "package.json")
	if _, err := os.Stat(packageJSONPath); err == nil {
		err = updateImportedPackageJSON(packageJSONPath, appName)
		if err != nil {
			return fmt.Errorf("failed to update package.json: %w", err)
		}
	}

	return nil
}

// updateMonorepoConfig updates the workspace configuration after all apps are added
func updateMonorepoConfig(workspacePath string, instructions []InjectionInstruction) error {
	// Update nx.json to include all apps
	nxJSONPath := filepath.Join(workspacePath, "nx.json")
	err := updateNxJSONForMonorepo(nxJSONPath, instructions)
	if err != nil {
		return fmt.Errorf("failed to update nx.json: %w", err)
	}

	// Update root package.json
	packageJSONPath := filepath.Join(workspacePath, "package.json")
	err = updateRootPackageJSON(packageJSONPath, instructions)
	if err != nil {
		return fmt.Errorf("failed to update root package.json: %w", err)
	}

	return nil
}

// File template generators
func generateProjectJSON(appName string) string {
	return fmt.Sprintf(`{
  "name": "%s",
  "sourceRoot": "apps/%s/src",
  "projectType": "application",
  "targets": {
    "build": {
      "executor": "@nx/vite:build",
      "outputs": ["{options.outputPath}"],
      "options": {
        "outputPath": "dist/apps/%s"
      }
    },
    "serve": {
      "executor": "@nx/vite:dev-server",
      "defaultConfiguration": "development",
      "options": {
        "buildTarget": "%s:build"
      }
    },
    "test": {
      "executor": "@nx/jest:jest",
      "outputs": ["{workspaceRoot}/coverage/apps/%s"],
      "options": {
        "jestConfig": "apps/%s/jest.config.ts",
        "passWithNoTests": true
      }
    },
    "lint": {
      "executor": "@nx/eslint:lint",
      "outputs": ["{options.outputFile}"],
      "options": {
        "lintFilePatterns": ["apps/%s/**/*.{ts,tsx,js,jsx}"]
      }
    }
  },
  "tags": []
}`, appName, appName, appName, appName, appName, appName, appName)
}

func generateMainTsx(appName string) string {
	log.Default().Println("Generating main.tsx for app:", appName)
	return `import { StrictMode } from 'react';
import * as ReactDOM from 'react-dom/client';

import App from './app/app';

const root = ReactDOM.createRoot(
  document.getElementById('root') as HTMLElement
);
root.render(
  <StrictMode>
    <App />
  </StrictMode>
);`
}

func generateAppTsx(appName string) string {
	caser := cases.Title(language.English)
	capitalizedName := caser.String(strings.ReplaceAll(appName, "-", " "))
	return fmt.Sprintf(`import './app.css';

export function App() {
  return (
    <div className="app">
      <header>
        <h1>Welcome to %s!</h1>
      </header>
      <main>
        <p>This is your React application running in an Nx monorepo.</p>
      </main>
    </div>
  );
}

export default App;`, capitalizedName)
}

func generateStylesCSS() string {
	return `/* You can add global styles to this file, and also import other style files */
body {
  margin: 0;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'Roboto', 'Oxygen',
    'Ubuntu', 'Cantarell', 'Fira Sans', 'Droid Sans', 'Helvetica Neue',
    sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

.app {
  text-align: center;
  padding: 2rem;
}

.app header {
  background-color: #282c34;
  padding: 2rem;
  color: white;
  border-radius: 8px;
  margin-bottom: 2rem;
}

.app main {
  font-size: 1.2rem;
}`
}

func generateIndexHTML(appName string) string {
	caser := cases.Title(language.English)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <title>%s</title>
    <base href="/" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <link rel="icon" type="image/x-icon" href="favicon.ico" />
  </head>
  <body>
    <div id="root"></div>
  </body>
</html>`, caser.String(appName))
}

func generateTsConfig(appName string) string {
	fmt.Fprintf(os.Stdout, "Generating tsconfig.json for app: %s\n", appName)
	return `{
  "extends": "../../tsconfig.base.json",
  "compilerOptions": {
    "jsx": "react-jsx",
    "allowJs": true,
    "esModuleInterop": true,
    "allowSyntheticDefaultImports": true,
    "forceConsistentCasingInFileNames": true,
    "strict": true,
    "noImplicitOverride": true,
    "noPropertyAccessFromIndexSignature": true,
    "noImplicitReturns": true,
    "noFallthroughCasesInSwitch": true
  },
  "files": [],
  "include": [],
  "references": [
    {
      "path": "./tsconfig.app.json"
    }
  ]
}`
}

func generateTsConfigApp() string {
	return `{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../dist/out-tsc",
    "types": ["node"]
  },
  "files": [
    "../../node_modules/@nx/react/typings/cssmodule.d.ts",
    "../../node_modules/@nx/react/typings/image.d.ts"
  ],
  "exclude": [
    "jest.config.ts",
    "src/**/*.spec.ts",
    "src/**/*.test.ts",
    "src/**/*.spec.tsx",
    "src/**/*.test.tsx",
    "src/**/*.spec.js",
    "src/**/*.test.js",
    "src/**/*.spec.jsx",
    "src/**/*.test.jsx"
  ],
  "include": [
    "src/**/*.js",
    "src/**/*.jsx",
    "src/**/*.ts",
    "src/**/*.tsx"
  ]
}`
}

func generateViteConfig(appName string) string {
	return fmt.Sprintf(`/// <reference types='vitest' />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { nxViteTsPaths } from '@nx/vite/plugins/nx-tsconfig-paths.plugin';

export default defineConfig({
  root: __dirname,
  cacheDir: '../../node_modules/.vite/%s',

  server: {
    port: 4200,
    host: 'localhost',
  },

  preview: {
    port: 4300,
    host: 'localhost',
  },

  plugins: [react(), nxViteTsPaths()],

  // Uncomment this if you are using workers.
  // worker: {
  //  plugins: [ nxViteTsPaths() ],
  // },

  build: {
    outDir: '../../dist/apps/%s',
    reportCompressedSize: true,
    commonjsOptions: {
      transformMixedEsModules: true,
    },
  },

  test: {
    globals: true,
    cache: {
      dir: '../../node_modules/.vitest',
    },
    environment: 'jsdom',
    include: ['src/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],

    reporters: ['default'],
    coverage: {
      reportsDirectory: '../../coverage/apps/%s',
      provider: 'v8',
    },
  },
});`, appName, appName, appName)
}
