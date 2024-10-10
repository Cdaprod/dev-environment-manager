// cmd.go
// This file contains command definitions and configuration setup using Cobra and Viper.
package main

import (
    "fmt"
    "os"
    "strings"

    "github.com/sirupsen/logrus"
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

// Root command for the CLI
var rootCmd = &cobra.Command{
    Use:   "dev-environment-manager",
    Short: "Manage development environments using Docker and Neovim",
}

// Execute runs the root command
func Execute() {
    if err := rootCmd.Execute(); err != nil {
        logrus.Fatal(err)
        os.Exit(1)
    }
}

func init() {
    cobra.OnInitialize(initConfig)

    // Global flags
    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.dev-env-manager.yaml)")

    // Add subcommands
    rootCmd.AddCommand(startCmd)
    rootCmd.AddCommand(addProjectCmd)
}

// Config file path
var cfgFile string

// Initialize configuration using Viper
func initConfig() {
    if cfgFile != "" {
        viper.SetConfigFile(cfgFile)
    } else {
        home, err := os.UserHomeDir()
        cobra.CheckErr(err)

        viper.AddConfigPath(home)
        viper.SetConfigName(".dev-env-manager")
        viper.SetConfigType("yaml")
    }

    viper.AutomaticEnv() // Read in environment variables that match

    if err := viper.ReadInConfig(); err == nil {
        logrus.Infof("Using config file: %s", viper.ConfigFileUsed())
    } else {
        logrus.Warn("No config file found; a new one will be created upon adding projects.")
    }
}

// Command to start a project environment
var startCmd = &cobra.Command{
    Use:   "start [project-dir-name] [repo-name]",
    Short: "Start development environment for a project",
    Args:  cobra.ExactArgs(2),
    Run: func(cmd *cobra.Command, args []string) {
        projectDirName := args[0]
        repoName := args[1]
        if err := StartProject(projectDirName, repoName); err != nil {
            logrus.Fatalf("Error starting project: %v", err)
        }
    },
}

// Command to add a new project configuration dynamically
var addProjectCmd = &cobra.Command{
    Use:   "add [project-dir-name] [repo-name] [repo_url]",
    Short: "Add a new project to the configuration",
    Args:  cobra.ExactArgs(3),
    Run: func(cmd *cobra.Command, args []string) {
        projectDirName := args[0]
        repoName := args[1]
        repoURL := args[2]

        // Derive Docker image and container name based on project name using Registry pattern
        dockerImage := fmt.Sprintf("cdaprod/%s:latest", strings.ToLower(repoName))
        containerName := fmt.Sprintf("nvim-%s", strings.ToLower(repoName))

        if err := AddProjectConfig(projectDirName, repoName, repoURL, dockerImage, containerName); err != nil {
            logrus.Fatalf("Error adding project: %v", err)
        }
    },
}