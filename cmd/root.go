package cmd

import (
    "fmt"
    "os"

    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "nx-scaffolder",
    Short: "A CLI tool to automate Nx React monorepo setup",
    Long: `nx-scaffolder is a CLI utility that automates the process of fetching,
downloading, configuring, and deploying Nx monorepos specifically for React applications.`,
}

func Execute() {
    if err := rootCmd.Execute(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}

func init() {
    // Global flags can be added here
    rootCmd.PersistentFlags().StringP("output", "o", ".", "Output directory for the scaffolded project")
}