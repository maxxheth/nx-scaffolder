package utils

import (
	"context"
	"encoding/json"
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
	fmt.Printf("Creating new React app with Vite: %s\n", appName)

	// Create the app directory path
	// appPath := filepath.Join(workspacePath, "apps", appName)

	// Ensure the apps directory exists
	appsDir := filepath.Join(workspacePath, "apps")
	err := os.MkdirAll(appsDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create apps directory: %w", err)
	}

	// Use create-nx-workspace to generate a standalone React app with Vite
	cmd := exec.Command("npx", "create-nx-workspace@latest", appName,
		"--preset=react-standalone",
		"--bundler=vite",
		"--interactive=false")
	cmd.Dir = appsDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		fmt.Printf("Nx workspace creation failed, falling back to manual creation: %v\n", err)
		return createReactAppManually(workspacePath, appName)
	}

	// Move the generated workspace into the apps directory structure
	generatedPath := filepath.Join(appsDir, appName)
	if _, err := os.Stat(generatedPath); err == nil {
		// The workspace was created successfully
		fmt.Printf("✅ Successfully created React app with Vite: %s\n", appName)
		return nil
	}

	// If something went wrong, fall back to manual creation
	return createReactAppManually(workspacePath, appName)
}

// createTsConfigBase creates the base TypeScript configuration file
// func createTsConfigBase(workspacePath string) error {
// 	tsconfigBase := `{
//   "compileOnSave": false,
//   "compilerOptions": {
//     "rootDir": ".",
//     "sourceMap": true,
//     "declaration": false,
//     "moduleResolution": "node",
//     "emitDecoratorMetadata": true,
//     "experimentalDecorators": true,
//     "importHelpers": true,
//     "target": "es2015",
//     "module": "esnext",
//     "lib": ["es2020", "dom"],
//     "skipLibCheck": true,
//     "skipDefaultLibCheck": true,
//     "baseUrl": ".",
//     "paths": {}
//   },
//   "exclude": ["node_modules", "tmp"]
// }`

// 	tsconfigBasePath := filepath.Join(workspacePath, "tsconfig.base.json")
// 	return os.WriteFile(tsconfigBasePath, []byte(tsconfigBase), 0644)
// }

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
	// Create project.json for the imported app
	err := createProjectJsonForImportedApp(appPath, appName)
	if err != nil {
		return fmt.Errorf("failed to create project.json: %w", err)
	}

	// Create vite.config.ts for build configuration
	err = createViteConfigForImportedApp(appPath, appName)
	if err != nil {
		return fmt.Errorf("failed to create vite.config.ts: %w", err)
	}

	// Update package.json to remove conflicting configurations
	packageJSONPath := filepath.Join(appPath, "package.json")
	if _, err := os.Stat(packageJSONPath); err == nil {
		err = updateImportedPackageJSON(packageJSONPath, appName)
		if err != nil {
			return fmt.Errorf("failed to update package.json: %w", err)
		}
	}

	// Create TypeScript configuration files
	err = createTsConfigForImportedApp(appPath, appName)
	if err != nil {
		return fmt.Errorf("failed to create TypeScript config: %w", err)
	}

	// Remove conflicting config files
	filesToRemove := []string{
		"webpack.config.js",
		"craco.config.js",
		".env.local",
		".env.development.local",
		".env.production.local",
	}

	for _, file := range filesToRemove {
		filePath := filepath.Join(appPath, file)
		if _, err := os.Stat(filePath); err == nil {
			os.Remove(filePath)
		}
	}

	return nil
}

// createTsConfigForImportedApp creates TypeScript configuration for imported apps
func createTsConfigForImportedApp(appPath, appName string) error {
	fmt.Printf("Generating TypeScript configuration for app: %s\n", appName)
	// Main tsconfig.json
	tsConfig := `{
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
    },
    {
      "path": "./tsconfig.spec.json"
    }
  ]
}`

	// App-specific tsconfig
	tsConfigApp := `{
  "extends": "./tsconfig.json",
  "compilerOptions": {
	"outDir": "../../dist/out-tsc",
	"types": ["node", "vite/client"]
  },
  "files": [
	"../../node_modules/@nx/react/typings/cssmodule.d.ts",
	"../../node_modules/@nx/react/typings/image.d.ts",
	"vite-env.d.ts"
  ],
  "exclude": [
	"**/*.spec.ts",
	"**/*.test.ts",
	"**/*.spec.tsx",
	"**/*.test.tsx",
	"**/*.spec.js",
	"**/*.test.js",
	"**/*.spec.jsx",
	"**/*.test.jsx"
  ],
  "include": ["src/**/*"]
}`

	// Test-specific tsconfig
	tsConfigSpec := `{
  "extends": "./tsconfig.json",
  "compilerOptions": {
    "outDir": "../../dist/out-tsc",
    "types": ["vitest/globals", "vitest/importMeta", "vite/client", "node"]
  },
  "include": [
    "vite.config.ts",
    "src/**/*.test.ts",
    "src/**/*.spec.ts",
    "src/**/*.test.tsx",
    "src/**/*.spec.tsx",
    "src/**/*.test.js",
    "src/**/*.spec.js",
    "src/**/*.test.jsx",
    "src/**/*.spec.jsx"
  ]
}`

	// Write config files
	configs := map[string]string{
		"tsconfig.json":      tsConfig,
		"tsconfig.app.json":  tsConfigApp,
		"tsconfig.spec.json": tsConfigSpec,
	}

	for filename, content := range configs {
		configPath := filepath.Join(appPath, filename)
		err := os.WriteFile(configPath, []byte(content), 0644)
		if err != nil {
			return fmt.Errorf("failed to write %s: %w", filename, err)
		}
	}

	// Create vite-env.d.ts
	viteEnv := `/// <reference types="vite/client" />`
	viteEnvPath := filepath.Join(appPath, "vite-env.d.ts")
	return os.WriteFile(viteEnvPath, []byte(viteEnv), 0644)
}

// createViteConfigForImportedApp creates a Vite config for imported apps
func createViteConfigForImportedApp(appPath, appName string) error {
	viteConfig := fmt.Sprintf(`/// <reference types='vitest' />
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { nxViteTsPaths } from '@nx/vite/plugins/nx-tsconfig-paths.plugin';
import { nxCopyAssetsPlugin } from '@nx/vite/plugins/nx-copy-assets.plugin';

export default defineConfig(() => ({
  root: __dirname,
  cacheDir: './node_modules/.vite/%s',
  server: {
    port: 4200,
    host: 'localhost',
  },
  preview: {
    port: 4300,
    host: 'localhost',
  },
  plugins: [react(), nxViteTsPaths(), nxCopyAssetsPlugin(['*.md'])],
  // Uncomment this if you are using workers.
  // worker: {
  //  plugins: [ nxViteTsPaths() ],
  // },
  build: {
    outDir: './dist/%s',
    emptyOutDir: true,
    reportCompressedSize: true,
    commonjsOptions: {
      transformMixedEsModules: true,
    },
  },
  test: {
    watch: false,
    globals: true,
    environment: 'jsdom',
    include: ['{src,tests}/**/*.{test,spec}.{js,mjs,cjs,ts,mts,cts,jsx,tsx}'],
    reporters: ['default'],
    coverage: {
      reportsDirectory: './coverage/%s',
      provider: 'v8' as const,
    },
  },
}));
`, appName, appName, appName)

	viteConfigPath := filepath.Join(appPath, "vite.config.ts")
	return os.WriteFile(viteConfigPath, []byte(viteConfig), 0644)
}

// createProjectJsonForImportedApp creates a proper project.json for imported React apps
func createProjectJsonForImportedApp(appPath, appName string) error {
	projectJSON := map[string]interface{}{
		"name":        appName,
		"$schema":     "../../node_modules/nx/schemas/project-schema.json",
		"projectType": "application",
		"sourceRoot":  fmt.Sprintf("apps/%s/src", appName),
		"targets": map[string]interface{}{
			"build": map[string]interface{}{
				"executor": "@nx/vite:build",
				"outputs":  []string{"{options.outputPath}"},
				"options": map[string]interface{}{
					"outputPath": fmt.Sprintf("dist/apps/%s", appName),
				},
			},
			"serve": map[string]interface{}{
				"executor": "@nx/vite:dev-server",
				"options": map[string]interface{}{
					"buildTarget": fmt.Sprintf("%s:build", appName),
					"hmr":         true,
				},
				"configurations": map[string]interface{}{
					"development": map[string]interface{}{
						"buildTarget": fmt.Sprintf("%s:build:development", appName),
						"hmr":         true,
					},
				},
			},
			"preview": map[string]interface{}{
				"executor": "@nx/vite:preview-server",
				"options": map[string]interface{}{
					"buildTarget": fmt.Sprintf("%s:build", appName),
				},
			},
			"test": map[string]interface{}{
				"executor": "@nx/vite:test",
				"outputs":  []string{"{options.reportsDirectory}"},
				"options": map[string]interface{}{
					"passWithNoTests":  true,
					"reportsDirectory": fmt.Sprintf("../../coverage/apps/%s", appName),
				},
			},
			"lint": map[string]interface{}{
				"executor": "@nx/eslint:lint",
				"outputs":  []string{"{options.outputFile}"},
				"options": map[string]interface{}{
					"lintFilePatterns": []string{fmt.Sprintf("apps/%s/**/*.{ts,tsx,js,jsx}", appName)},
				},
			},
		},
		"tags": []string{},
	}

	projectJSONPath := filepath.Join(appPath, "project.json")
	data, err := json.MarshalIndent(projectJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project.json: %w", err)
	}

	err = os.WriteFile(projectJSONPath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write project.json: %w", err)
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

// updateRootPackageJSON updates the root package.json with workspace information
func updateRootPackageJSON(packageJSONPath string, instructions []InjectionInstruction) error {
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return err
	}

	var packageJSON map[string]interface{}
	err = json.Unmarshal(data, &packageJSON)
	if err != nil {
		return err
	}

	// Ensure scripts section exists
	if _, exists := packageJSON["scripts"]; !exists {
		packageJSON["scripts"] = make(map[string]interface{})
	}

	scripts := packageJSON["scripts"].(map[string]interface{})

	// Add useful workspace scripts
	scripts["build"] = "nx build"
	scripts["test"] = "nx test"
	scripts["lint"] = "nx lint"
	scripts["serve"] = "nx serve"
	scripts["graph"] = "nx graph"

	// Add app-specific scripts for each instruction
	for _, instruction := range instructions {
		appName := instruction.AppName
		scripts[fmt.Sprintf("build:%s", appName)] = fmt.Sprintf("nx build %s", appName)
		scripts[fmt.Sprintf("serve:%s", appName)] = fmt.Sprintf("nx serve %s", appName)
		scripts[fmt.Sprintf("test:%s", appName)] = fmt.Sprintf("nx test %s", appName)
		scripts[fmt.Sprintf("lint:%s", appName)] = fmt.Sprintf("nx lint %s", appName)
	}

	// Write back to file
	updatedData, err := json.MarshalIndent(packageJSON, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(packageJSONPath, updatedData, 0644)
}
