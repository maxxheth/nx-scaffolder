package utils

import (
	"fmt"
)

// Help provides detailed information about the CLI's features via the --help flag.

func Help() {
	fmt.Println(`
Nx Scaffolder CLI
Usage:
  nx-scaffolder [command] [options]
Commands:
  create [app-name]   Create a new Nx React workspace
  fetch [owner] [repo] [file-path]  Fetch a specific file from a GitHub repository
Options:
  --output, -o        Output directory for the scaffolded project (default: current directory)
  --owner, -o        GitHub repository owner (default: nrwl)
  --repo, -r         GitHub repository name (default: nx)
  --branch, -b       Git branch to download (default: master)
  --template, -t     Template type (default: react)
  --help, -h         Show this help message
Examples:
  nx-scaffolder create my-app --owner nrwl --repo nx --branch master --template react
  nx-scaffolder fetch nrwl nx .github/workflows/ci.yml
  nx-scaffolder --help`)
}
