Understood. Let’s create a flexible, dynamic, and user-friendly dev-environment-manager CLI tool that leverages Cobra and Viper for command-line interface and configuration management. This tool will allow you to run Docker containers for your repositories, automatically applying default volume bindings and other configurations without hardcoding paths in the Go code.

Overview of the Solution

	1.	Project Structure: Organize the project with a simplified structure.
	2.	Configuration Management: Use a YAML configuration file to manage volume bindings and environment variables.
	3.	CLI Commands: Implement add and start commands using Cobra.
	4.	Dynamic Volume Bindings: Use Viper to load configurations and apply them when running Docker containers.
	5.	Dockerfile: Create a Dockerfile to build the dev-environment-manager tool.

1. Project Structure

We’ll maintain a simple project structure with two main Go files and a Dockerfile:

dev-environment-manager/
├── cmd.go
├── main.go
├── config.yaml
├── Dockerfile
├── go.mod
└── go.sum

	•	main.go: Initializes the CLI application.
	•	cmd.go: Defines the CLI commands and their functionalities.
	•	config.yaml: Stores configurations for volume bindings and environment variables.
	•	Dockerfile: Builds the dev-environment-manager as a Docker image.
	•	go.mod & go.sum: Manage project dependencies.

2. Initialize the Go Module

Navigate to your project directory and initialize the Go module:

mkdir dev-environment-manager
cd dev-environment-manager
go mod init github.com/Cdaprod/dev-environment-manager
go get github.com/spf13/cobra
go get github.com/spf13/viper
go get github.com/sirupsen/logrus
go get github.com/go-git/go-git/v5
go get github.com/docker/docker/client

3. Configuration File (config.yaml)

Create a config.yaml file in the root directory. This file will store all your configurations, including default volume bindings and environment variables.

# config.yaml

# Global environment variables
environment_vars:
  - HOME=/home/cdaprod
  - CUSTOM_ENV_VAR=value

# Default volume bindings applicable to all projects
default_volume_bindings:
  - "$HOME/.config/nvim:/root/.config/nvim"
  - "$HOME/.vim:/root/.vim"
  - "$HOME/.vimrc:/root/.vimrc"
  - "$HOME/.tmux.conf:/root/.tmux.conf"
  - "$HOME/.local/share/nvim:/root/.local/share/nvim"

# Project-specific configurations
projects:
  my-project-dir:
    repos:
      my-repo:
        repo_url: "https://github.com/Cdaprod/my-repo.git"
        docker_image: "cdaprod/my-repo:latest"
        container_name: "nvim-my-repo"
        # Additional volume bindings specific to this repo
        volume_bindings:
          - "$HOME/Projects/my-project-dir/my-repo/plugins:/root/.local/share/nvim/plugins"

Explanation:

	•	environment_vars: Define any environment variables you want to pass to the Docker container.
	•	default_volume_bindings: These bindings apply to all projects and repositories.
	•	projects: Define each project directory and its associated repositories.
	•	repos: Each repository within a project can have its own settings and additional volume bindings.

4. Go Code

a. main.go

This file initializes the CLI application and sets up logging.

// main.go
package main

import (
    "os"

    "github.com/sirupsen/logrus"
)

func main() {
    // Initialize Logrus
    logrus.SetFormatter(&logrus.TextFormatter{
        FullTimestamp: true,
    })
    logrus.SetOutput(os.Stdout)
    logrus.SetLevel(logrus.InfoLevel)

    logrus.Info("Starting Development Environment Manager...")
    Execute()
}

b. cmd.go

This file defines the CLI commands (add and start) using Cobra and handles configuration using Viper.

// cmd.go
package main

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/client"
    git "github.com/go-git/go-git/v5"
    "github.com/sirupsen/logrus"
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

// RootCmd is the base command
var RootCmd = &cobra.Command{
    Use:   "ops",
    Short: "Dev Environment Manager",
    Long:  `Manage development environments using Docker with dynamic configurations.`,
}

// Execute runs the root command
func Execute() {
    if err := RootCmd.Execute(); err != nil {
        logrus.Fatal(err)
        os.Exit(1)
    }
}

func init() {
    cobra.OnInitialize(initConfig)

    // Persistent flag for config file
    RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")

    // Add subcommands
    RootCmd.AddCommand(addCmd)
    RootCmd.AddCommand(startCmd)
}

// Config file path
var cfgFile string

// initConfig reads in config file and ENV variables if set.
func initConfig() {
    if cfgFile != "" {
        // Use config file from the flag.
        viper.SetConfigFile(cfgFile)
    } else {
        // Search for config in the current directory with name "config.yaml"
        viper.AddConfigPath(".")
        viper.SetConfigName("config")
        viper.SetConfigType("yaml")
    }

    // Read environment variables that match
    viper.AutomaticEnv()
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

    // If a config file is found, read it in.
    if err := viper.ReadInConfig(); err == nil {
        logrus.Infof("Using config file: %s", viper.ConfigFileUsed())
    } else {
        logrus.Warn("No config file found; using default settings or environment variables.")
    }
}

// addCmd represents the add command
var addCmd = &cobra.Command{
    Use:   "add [project-dir-name] [repo-name] [repo_url]",
    Short: "Add a new project to the configuration",
    Args:  cobra.ExactArgs(3),
    Run: func(cmd *cobra.Command, args []string) {
        projectDirName := args[0]
        repoName := args[1]
        repoURL := args[2]

        // Derive Docker image and container name based on repo name
        dockerImage := fmt.Sprintf("cdaprod/%s:latest", strings.ToLower(repoName))
        containerName := fmt.Sprintf("nvim-%s", strings.ToLower(repoName))

        if err := AddProjectConfig(projectDirName, repoName, repoURL, dockerImage, containerName); err != nil {
            logrus.Fatalf("Error adding project: %v", err)
        }
    },
}

// startCmd represents the start command
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

// AddProjectConfig adds a new project to the configuration file
func AddProjectConfig(projectDirName, repoName, repoURL, dockerImage, containerName string) error {
    username, err := getUsername()
    if err != nil {
        return fmt.Errorf("error getting username: %v", err)
    }

    projectKey := fmt.Sprintf("projects.%s.repos.%s", projectDirName, repoName)

    // Check if repository already exists
    if viper.IsSet(projectKey) {
        return fmt.Errorf("repository %s already exists under project %s", repoName, projectDirName)
    }

    // Set configuration
    viper.Set(fmt.Sprintf("%s.repo_url", projectKey), repoURL)
    viper.Set(fmt.Sprintf("%s.docker_image", projectKey), dockerImage)
    viper.Set(fmt.Sprintf("%s.container_name", projectKey), containerName)

    // Write configuration to file
    if err := viper.WriteConfig(); err != nil {
        // If no config file exists, create a new one
        if os.IsNotExist(err) {
            if err := viper.SafeWriteConfig(); err != nil {
                return fmt.Errorf("error creating config file: %v", err)
            }
        } else {
            return fmt.Errorf("error writing config file: %v", err)
        }
    }

    logrus.Infof("Repository %s added under project %s.", repoName, projectDirName)
    return nil
}

// StartProject initiates the development environment for a specified project
func StartProject(projectDirName, repoName string) error {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return fmt.Errorf("error getting home directory: %v", err)
    }

    projectKey := fmt.Sprintf("projects.%s.repos.%s", projectDirName, repoName)

    if !viper.IsSet(projectKey) {
        return fmt.Errorf("repository %s under project %s is not configured", repoName, projectDirName)
    }

    repoURL := viper.GetString(fmt.Sprintf("%s.repo_url", projectKey))
    dockerImage := viper.GetString(fmt.Sprintf("%s.docker_image", projectKey))
    containerName := viper.GetString(fmt.Sprintf("%s.container_name", projectKey))

    projectPath := filepath.Join(homeDir, "Projects", projectDirName, repoName)
    if _, err := os.Stat(projectPath); os.IsNotExist(err) {
        err := CloneRepo(repoURL, projectPath)
        if err != nil {
            return fmt.Errorf("error cloning repository: %v", err)
        }
    } else {
        logrus.Infof("Project directory %s already exists. Skipping clone.", projectPath)
    }

    // Retrieve default and project-specific volume bindings
    defaultBinds := viper.GetStringSlice("default_volume_bindings")
    projectBinds := viper.GetStringSlice(fmt.Sprintf("%s.volume_bindings", projectKey))

    // Expand environment variables in bindings
    expandedDefaultBinds := expandBindings(defaultBinds, homeDir)
    expandedProjectBinds := expandBindings(projectBinds, homeDir)

    // Combine all binds
    binds := append(expandedDefaultBinds, expandedProjectBinds...)

    // Retrieve environment variables
    env := viper.GetStringSlice("environment_vars")
    if len(env) == 0 {
        env = []string{"HOME=/home/cdaprod"}
    }

    // Command to run Neovim
    cmdArgs := []string{"nvim"}

    // Run Docker container
    containerID, err := RunContainer(dockerImage, containerName, binds, cmdArgs, env)
    if err != nil {
        return fmt.Errorf("error running container: %v", err)
    }

    // Attach to the container
    err = AttachToContainer(containerID)
    if err != nil {
        return fmt.Errorf("error attaching to container: %v", err)
    }

    // Cleanup after exit
    err = RemoveContainer(containerID)
    if err != nil {
        return fmt.Errorf("error removing container: %v", err)
    }

    return nil
}

// expandBindings expands environment variables in volume bindings
func expandBindings(bindings []string, homeDir string) []string {
    expanded := []string{}
    for _, bind := range bindings {
        bind = strings.ReplaceAll(bind, "$HOME", homeDir)
        expanded = append(expanded, bind)
    }
    return expanded
}

// CloneRepo clones the repository to the destination path
func CloneRepo(repoURL, destPath string) error {
    logrus.Infof("Cloning repository %s into %s", repoURL, destPath)
    _, err := git.PlainClone(destPath, false, &git.CloneOptions{
        URL:      repoURL,
        Progress: os.Stdout,
    })
    if err != nil {
        logrus.Errorf("Error cloning repository: %v", err)
        return err
    }
    return nil
}

// RunContainer creates and starts a Docker container with the specified configurations
func RunContainer(imageName, containerName string, binds []string, cmdArgs []string, env []string) (string, error) {
    ctx := context.Background()
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        logrus.Errorf("Error creating Docker client: %v", err)
        return "", err
    }

    // Pull the image if not present
    logrus.Infof("Pulling Docker image %s...", imageName)
    reader, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
    if err != nil {
        logrus.Errorf("Error pulling image %s: %v", imageName, err)
        return "", err
    }
    defer reader.Close()
    io.Copy(os.Stdout, reader) // Display pull progress

    // Define container configuration
    containerConfig := &container.Config{
        Image: imageName,
        Cmd:   cmdArgs,
        Env:   env,
        Tty:   true, // Allocate a pseudo-TTY
    }

    // Define host configuration with volume bindings
    hostConfig := &container.HostConfig{
        Binds: binds,
    }

    // Check if container with the same name exists and remove it
    existingContainer, err := cli.ContainerInspect(ctx, containerName)
    if err == nil && existingContainer.ID != "" {
        logrus.Infof("Container %s already exists. Removing existing container...", containerName)
        RemoveContainer(existingContainer.ID)
    }

    // Create the container
    logrus.Infof("Creating Docker container %s...", containerName)
    resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
    if err != nil {
        logrus.Errorf("Error creating container %s: %v", containerName, err)
        return "", err
    }

    // Start the container
    logrus.Infof("Starting Docker container %s...", containerName)
    if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
        logrus.Errorf("Error starting container %s: %v", containerName, err)
        return "", err
    }

    logrus.Infof("Container %s started successfully with ID %s", containerName, resp.ID)
    return resp.ID, nil
}

// AttachToContainer attaches the user's terminal to the running container and starts Neovim
func AttachToContainer(containerID string) error {
    // Use Docker's exec to run Neovim interactively
    cmd := exec.Command("docker", "exec", "-it", containerID, "nvim")
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    logrus.Infof("Attaching to container %s with Neovim...", containerID)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("error executing Neovim: %v", err)
    }

    return nil
}

// RemoveContainer removes the Docker container after use
func RemoveContainer(containerID string) error {
    ctx := context.Background()
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return fmt.Errorf("error creating Docker client: %v", err)
    }

    logrus.Infof("Removing Docker container %s...", containerID)
    // Remove the container
    err = cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
    if err != nil {
        logrus.Errorf("Error removing container %s: %v", containerID, err)
        return err
    }

    logrus.Infof("Container %s removed successfully.", containerID)
    return nil
}

// getUsername retrieves the current user's username
func getUsername() (string, error) {
    usr, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    // Extract username from home directory path
    parts := strings.Split(usr, string(os.PathSeparator))
    if len(parts) == 0 {
        return "", fmt.Errorf("unable to determine username from home directory")
    }
    return parts[len(parts)-1], nil
}

Explanation:

	1.	Configuration Loading: The tool uses Viper to load configurations from config.yaml or a specified config file via the --config flag.
	2.	add Command: Adds a new repository under a project. It updates the configuration file with the repository URL, Docker image name, container name, and any additional volume bindings.
	3.	start Command: Starts the development environment for a specified project and repository. It performs the following steps:
	•	Cloning: Clones the repository if it doesn’t exist locally.
	•	Volume Bindings: Retrieves both default and project-specific volume bindings, expanding environment variables like $HOME.
	•	Environment Variables: Retrieves environment variables from the configuration or uses defaults.
	•	Docker Operations: Pulls the Docker image, creates, and starts the container with the specified configurations.
	•	Neovim Session: Attaches the user’s terminal to the running container and launches Neovim.
	•	Cleanup: Removes the Docker container after exiting Neovim to prevent clutter.
	4.	Dynamic Volume Bindings: The expandBindings function replaces placeholders like $HOME with the actual home directory path, ensuring flexibility across different user environments.
	5.	Error Handling: Comprehensive error handling provides clear feedback in case of issues during execution.

5. Dockerfile

We’ll create a multistage Dockerfile to build the dev-environment-manager Go application and then package it into a lightweight Docker image.

# Dockerfile

# Stage 1: Build the Go binary
FROM golang:1.20-alpine AS builder

# Install git for go-git
RUN apk update && apk add --no-cache git

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go binary
RUN go build -o ops main.go cmd.go

# Stage 2: Create a minimal image
FROM alpine:3.17

# Install Docker CLI (to allow the binary to run Docker commands)
RUN apk add --no-cache docker-cli

# Set environment variables
ENV HOME=/root

# Set the working directory
WORKDIR /usr/local/bin

# Copy the binary from the builder stage
COPY --from=builder /app/ops .

# Grant execution permissions
RUN chmod +x ops

# Copy the default configuration file
COPY config.yaml /app/config.yaml

# Set the entrypoint to the ops binary
ENTRYPOINT ["ops"]

# Default command
CMD ["--help"]

Explanation:

	1.	Stage 1 (Builder Stage):
	•	Base Image: golang:1.20-alpine for building the Go application.
	•	Dependencies: Installs git required by go-git.
	•	Working Directory: Sets to /app.
	•	Dependency Management: Copies go.mod and go.sum and runs go mod download.
	•	Source Code: Copies all source files and builds the Go binary named ops.
	2.	Stage 2 (Final Image):
	•	Base Image: alpine:3.17 for a minimal runtime environment.
	•	Docker CLI: Installs docker-cli to allow the ops binary to execute Docker commands.
	•	Environment Variables: Sets HOME to /root.
	•	Working Directory: Sets to /usr/local/bin.
	•	Binary: Copies the ops binary from the builder stage and grants execution permissions.
	•	Configuration File: Copies the config.yaml into /app/config.yaml within the container.
	•	Entrypoint: Sets the ops binary as the entrypoint.
	•	Default Command: Defaults to displaying help information.

6. Building and Running the Docker Image

a. Build the Docker Image

Run the following command in the project root directory to build the Docker image:

docker build -t cdaprod/dev-environment-manager:latest .

Explanation:

	•	-t cdaprod/dev-environment-manager:latest: Tags the image with the name cdaprod/dev-environment-manager and the latest tag.
	•	.: Specifies the current directory as the build context.

b. Running the Docker Container

To run the dev-environment-manager tool inside a Docker container with your configuration, use the following command:

docker run --rm -it \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$PWD/config.yaml:/app/config.yaml" \
  cdaprod/dev-environment-manager:latest start my-project-dir my-repo

Explanation:

	•	--rm: Automatically removes the container when it exits.
	•	-it: Runs the container in interactive mode with a pseudo-TTY.
	•	-v /var/run/docker.sock:/var/run/docker.sock: Mounts the Docker socket to allow the container to communicate with the Docker daemon on the host.
	•	-v "$PWD/config.yaml:/app/config.yaml": Mounts your local config.yaml into the container, allowing you to modify configurations without rebuilding the image.
	•	cdaprod/dev-environment-manager:latest: Specifies the Docker image to run.
	•	start my-project-dir my-repo: Executes the start command with the specified project and repository names.

Note: Mounting the Docker socket (/var/run/docker.sock) grants the container full access to the Docker daemon on the host, which can be a security risk. Ensure you understand the implications before using this approach.

7. Usage Instructions

a. Adding a New Project

To add a new project and repository to your configuration:

docker run --rm -it \
  -v "$PWD/config.yaml:/app/config.yaml" \
  cdaprod/dev-environment-manager:latest add my-project-dir my-repo https://github.com/Cdaprod/my-repo.git

Explanation:

	•	add my-project-dir my-repo https://github.com/Cdaprod/my-repo.git: Adds a new repository under the specified project with the given Git URL.

b. Starting a Project Environment

To start the development environment for a specific project and repository:

docker run --rm -it \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$PWD/config.yaml:/app/config.yaml" \
  cdaprod/dev-environment-manager:latest start my-project-dir my-repo

Explanation:

	•	Volume Bindings: The tool automatically applies both default and project-specific volume bindings as defined in config.yaml.
	•	Neovim Session: Launches Neovim inside the Docker container with the specified configurations.

8. Enhancing Flexibility and Avoiding Hardcoding

To make the tool even more flexible and avoid hardcoding paths, follow these practices:

a. Environment Variable Expansion

Ensure that your config.yaml can use environment variables like $HOME. The Go code provided already handles the expansion of $HOME in volume bindings using the expandBindings function.

b. Project-Specific Configurations

Leverage the projects section in config.yaml to define additional volume bindings and environment variables specific to each repository. This allows you to customize environments per project without altering the Go code.

Example:

projects:
  my-project-dir:
    repos:
      my-repo:
        repo_url: "https://github.com/Cdaprod/my-repo.git"
        docker_image: "cdaprod/my-repo:latest"
        container_name: "nvim-my-repo"
        volume_bindings:
          - "$HOME/Projects/my-project-dir/my-repo/plugins:/root/.local/share/nvim/plugins"
          - "$HOME/.tmux.conf:/root/.tmux.conf"

c. Adding More Configurations

To add more tools (like tmux, additional Neovim plugins, etc.), simply update the config.yaml with the necessary volume bindings and environment variables under the relevant repository.

9. Final Thoughts and Best Practices

	1.	Security Considerations:
	•	Docker Socket: Mounting /var/run/docker.sock provides the container with full access to the Docker daemon. Use this approach cautiously, especially in multi-user environments.
	•	Least Privilege: Ensure that the Docker images and containers adhere to the principle of least privilege.
	2.	Configuration Management:
	•	Version Control: Keep your config.yaml under version control to track changes and collaborate with others.
	•	Environment Variables: Use environment variables for sensitive data instead of hardcoding them in config.yaml.
	3.	Extensibility:
	•	Additional Commands: You can extend the CLI with more commands (e.g., stop, list, remove) to manage Docker containers more effectively.
	•	Plugins: Implement plugin support to allow users to extend the tool’s functionalities.
	4.	Testing:
	•	Unit Tests: Develop unit tests for critical functions to ensure reliability.
	•	Integration Tests: Test the entire workflow in a controlled environment before deploying it for daily use.
	5.	Documentation:
	•	README: Create a comprehensive README.md detailing installation, usage, configuration, and troubleshooting steps.
	•	Help Commands: Leverage Cobra’s help command features to provide users with inline assistance.
	6.	Error Handling:
	•	User-Friendly Messages: Ensure that error messages are descriptive and guide the user toward resolving issues.
	•	Logging Levels: Utilize different logging levels (Info, Warn, Error, Debug) to control verbosity based on user preference.

10. Complete Code Snippets

a. main.go

// main.go
package main

import (
    "os"

    "github.com/sirupsen/logrus"
)

func main() {
    // Initialize Logrus
    logrus.SetFormatter(&logrus.TextFormatter{
        FullTimestamp: true,
    })
    logrus.SetOutput(os.Stdout)
    logrus.SetLevel(logrus.InfoLevel)

    logrus.Info("Starting Development Environment Manager...")
    Execute()
}

b. cmd.go

// cmd.go
package main

import (
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "strings"

    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/client"
    git "github.com/go-git/go-git/v5"
    "github.com/sirupsen/logrus"
    "github.com/spf13/cobra"
    "github.com/spf13/viper"
)

// RootCmd is the base command
var RootCmd = &cobra.Command{
    Use:   "ops",
    Short: "Dev Environment Manager",
    Long:  `Manage development environments using Docker with dynamic configurations.`,
}

// Execute runs the root command
func Execute() {
    if err := RootCmd.Execute(); err != nil {
        logrus.Fatal(err)
        os.Exit(1)
    }
}

func init() {
    cobra.OnInitialize(initConfig)

    // Persistent flag for config file
    RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")

    // Add subcommands
    RootCmd.AddCommand(addCmd)
    RootCmd.AddCommand(startCmd)
}

// Config file path
var cfgFile string

// initConfig reads in config file and ENV variables if set.
func initConfig() {
    if cfgFile != "" {
        // Use config file from the flag.
        viper.SetConfigFile(cfgFile)
    } else {
        // Search for config in the current directory with name "config.yaml"
        viper.AddConfigPath(".")
        viper.SetConfigName("config")
        viper.SetConfigType("yaml")
    }

    // Read environment variables that match
    viper.AutomaticEnv()
    viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

    // If a config file is found, read it in.
    if err := viper.ReadInConfig(); err == nil {
        logrus.Infof("Using config file: %s", viper.ConfigFileUsed())
    } else {
        logrus.Warn("No config file found; using default settings or environment variables.")
    }
}

// addCmd represents the add command
var addCmd = &cobra.Command{
    Use:   "add [project-dir-name] [repo-name] [repo_url]",
    Short: "Add a new project to the configuration",
    Args:  cobra.ExactArgs(3),
    Run: func(cmd *cobra.Command, args []string) {
        projectDirName := args[0]
        repoName := args[1]
        repoURL := args[2]

        // Derive Docker image and container name based on repo name
        dockerImage := fmt.Sprintf("cdaprod/%s:latest", strings.ToLower(repoName))
        containerName := fmt.Sprintf("nvim-%s", strings.ToLower(repoName))

        if err := AddProjectConfig(projectDirName, repoName, repoURL, dockerImage, containerName); err != nil {
            logrus.Fatalf("Error adding project: %v", err)
        }
    },
}

// startCmd represents the start command
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

// AddProjectConfig adds a new project to the configuration file
func AddProjectConfig(projectDirName, repoName, repoURL, dockerImage, containerName string) error {
    projectKey := fmt.Sprintf("projects.%s.repos.%s", projectDirName, repoName)

    // Check if repository already exists
    if viper.IsSet(projectKey) {
        return fmt.Errorf("repository %s already exists under project %s", repoName, projectDirName)
    }

    // Set configuration
    viper.Set(fmt.Sprintf("%s.repo_url", projectKey), repoURL)
    viper.Set(fmt.Sprintf("%s.docker_image", projectKey), dockerImage)
    viper.Set(fmt.Sprintf("%s.container_name", projectKey), containerName)

    // Write configuration to file
    if err := viper.WriteConfig(); err != nil {
        // If no config file exists, create a new one
        if os.IsNotExist(err) {
            if err := viper.SafeWriteConfig(); err != nil {
                return fmt.Errorf("error creating config file: %v", err)
            }
        } else {
            return fmt.Errorf("error writing config file: %v", err)
        }
    }

    logrus.Infof("Repository %s added under project %s.", repoName, projectDirName)
    return nil
}

// StartProject initiates the development environment for a specified project
func StartProject(projectDirName, repoName string) error {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return fmt.Errorf("error getting home directory: %v", err)
    }

    projectKey := fmt.Sprintf("projects.%s.repos.%s", projectDirName, repoName)

    if !viper.IsSet(projectKey) {
        return fmt.Errorf("repository %s under project %s is not configured", repoName, projectDirName)
    }

    repoURL := viper.GetString(fmt.Sprintf("%s.repo_url", projectKey))
    dockerImage := viper.GetString(fmt.Sprintf("%s.docker_image", projectKey))
    containerName := viper.GetString(fmt.Sprintf("%s.container_name", projectKey))

    projectPath := filepath.Join(homeDir, "Projects", projectDirName, repoName)
    if _, err := os.Stat(projectPath); os.IsNotExist(err) {
        err := CloneRepo(repoURL, projectPath)
        if err != nil {
            return fmt.Errorf("error cloning repository: %v", err)
        }
    } else {
        logrus.Infof("Project directory %s already exists. Skipping clone.", projectPath)
    }

    // Retrieve default and project-specific volume bindings
    defaultBinds := viper.GetStringSlice("default_volume_bindings")
    projectBinds := viper.GetStringSlice(fmt.Sprintf("%s.volume_bindings", projectKey))

    // Expand environment variables in bindings
    expandedDefaultBinds := expandBindings(defaultBinds, homeDir)
    expandedProjectBinds := expandBindings(projectBinds, homeDir)

    // Combine all binds
    binds := append(expandedDefaultBinds, expandedProjectBinds...)

    // Retrieve environment variables
    env := viper.GetStringSlice("environment_vars")
    if len(env) == 0 {
        env = []string{"HOME=/home/cdaprod"}
    }

    // Command to run Neovim
    cmdArgs := []string{"nvim"}

    // Run Docker container
    containerID, err := RunContainer(dockerImage, containerName, binds, cmdArgs, env)
    if err != nil {
        return fmt.Errorf("error running container: %v", err)
    }

    // Attach to the container
    err = AttachToContainer(containerID)
    if err != nil {
        return fmt.Errorf("error attaching to container: %v", err)
    }

    // Cleanup after exit
    err = RemoveContainer(containerID)
    if err != nil {
        return fmt.Errorf("error removing container: %v", err)
    }

    return nil
}

// expandBindings expands environment variables in volume bindings
func expandBindings(bindings []string, homeDir string) []string {
    expanded := []string{}
    for _, bind := range bindings {
        bind = strings.ReplaceAll(bind, "$HOME", homeDir)
        expanded = append(expanded, bind)
    }
    return expanded
}

// CloneRepo clones the repository to the destination path
func CloneRepo(repoURL, destPath string) error {
    logrus.Infof("Cloning repository %s into %s", repoURL, destPath)
    _, err := git.PlainClone(destPath, false, &git.CloneOptions{
        URL:      repoURL,
        Progress: os.Stdout,
    })
    if err != nil {
        logrus.Errorf("Error cloning repository: %v", err)
        return err
    }
    return nil
}

// RunContainer creates and starts a Docker container with the specified configurations
func RunContainer(imageName, containerName string, binds []string, cmdArgs []string, env []string) (string, error) {
    ctx := context.Background()
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        logrus.Errorf("Error creating Docker client: %v", err)
        return "", err
    }

    // Pull the image if not present
    logrus.Infof("Pulling Docker image %s...", imageName)
    reader, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
    if err != nil {
        logrus.Errorf("Error pulling image %s: %v", imageName, err)
        return "", err
    }
    defer reader.Close()
    io.Copy(os.Stdout, reader) // Display pull progress

    // Define container configuration
    containerConfig := &container.Config{
        Image: imageName,
        Cmd:   cmdArgs,
        Env:   env,
        Tty:   true, // Allocate a pseudo-TTY
    }

    // Define host configuration with volume bindings
    hostConfig := &container.HostConfig{
        Binds: binds, // Volume bindings passed as arguments
    }

    // Check if container with the same name exists and remove it
    existingContainer, err := cli.ContainerInspect(ctx, containerName)
    if err == nil && existingContainer.ID != "" {
        logrus.Infof("Container %s already exists. Removing existing container...", containerName)
        RemoveContainer(existingContainer.ID)
    }

    // Create the container
    logrus.Infof("Creating Docker container %s...", containerName)
    resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
    if err != nil {
        logrus.Errorf("Error creating container %s: %v", containerName, err)
        return "", err
    }

    // Start the container
    logrus.Infof("Starting Docker container %s...", containerName)
    if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
        logrus.Errorf("Error starting container %s: %v", containerName, err)
        return "", err
    }

    logrus.Infof("Container %s started successfully with ID %s", containerName, resp.ID)
    return resp.ID, nil
}

// AttachToContainer attaches the user's terminal to the running container and starts Neovim
func AttachToContainer(containerID string) error {
    // Use Docker's exec to run Neovim interactively
    cmd := exec.Command("docker", "exec", "-it", containerID, "nvim")
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr

    logrus.Infof("Attaching to container %s with Neovim...", containerID)
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("error executing Neovim: %v", err)
    }

    return nil
}

// RemoveContainer removes the Docker container after use
func RemoveContainer(containerID string) error {
    ctx := context.Background()
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        return fmt.Errorf("error creating Docker client: %v", err)
    }

    logrus.Infof("Removing Docker container %s...", containerID)
    // Remove the container
    err = cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
    if err != nil {
        logrus.Errorf("Error removing container %s: %v", containerID, err)
        return err
    }

    logrus.Infof("Container %s removed successfully.", containerID)
    return nil
}

// getUsername retrieves the current user's username
func getUsername() (string, error) {
    usr, err := os.UserHomeDir()
    if err != nil {
        return "", err
    }
    // Extract username from home directory path
    parts := strings.Split(usr, string(os.PathSeparator))
    if len(parts) == 0 {
        return "", fmt.Errorf("unable to determine username from home directory")
    }
    return parts[len(parts)-1], nil
}

Key Enhancements:

	1.	Dynamic Configuration Loading: The tool uses Viper to load configurations from an external config.yaml file, allowing you to manage volume bindings and environment variables without altering the code.
	2.	Volume Bindings Expansion: The expandBindings function replaces placeholders like $HOME with the actual home directory path, ensuring flexibility across different user environments.
	3.	Project-Specific Volume Bindings: You can define additional volume bindings for each repository under a project in config.yaml, making it easy to manage complex configurations.
	4.	Docker Container Management: The tool handles pulling images, creating containers with the specified bindings, attaching to the container, and cleaning up afterward.

6. Dockerfile

We’ll create a multistage Dockerfile to build the dev-environment-manager Go application and package it into a lightweight Docker image.

# Dockerfile

# Stage 1: Build the Go binary
FROM golang:1.20-alpine AS builder

# Install git for go-git
RUN apk update && apk add --no-cache git

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go binary
RUN go build -o ops main.go cmd.go

# Stage 2: Create a minimal image
FROM alpine:3.17

# Install Docker CLI (to allow the binary to run Docker commands)
RUN apk add --no-cache docker-cli bash

# Set environment variables
ENV HOME=/root

# Set the working directory
WORKDIR /usr/local/bin

# Copy the binary from the builder stage
COPY --from=builder /app/ops .

# Grant execution permissions
RUN chmod +x ops

# Copy the default configuration file
COPY config.yaml /app/config.yaml

# Set the entrypoint to the ops binary
ENTRYPOINT ["ops"]

# Default command
CMD ["--help"]

Explanation:

	1.	Stage 1 (Builder Stage):
	•	Base Image: golang:1.20-alpine for building the Go application.
	•	Dependencies: Installs git required by go-git.
	•	Working Directory: Sets to /app.
	•	Dependency Management: Copies go.mod and go.sum and runs go mod download.
	•	Source Code: Copies all source files and builds the Go binary named ops.
	2.	Stage 2 (Final Image):
	•	Base Image: alpine:3.17 for a minimal runtime environment.
	•	Docker CLI: Installs docker-cli and bash to allow the ops binary to execute Docker commands.
	•	Environment Variables: Sets HOME to /root.
	•	Working Directory: Sets to /usr/local/bin.
	•	Binary: Copies the ops binary from the builder stage and grants execution permissions.
	•	Configuration File: Copies the config.yaml into /app/config.yaml within the container.
	•	Entrypoint: Sets the ops binary as the entrypoint.
	•	Default Command: Defaults to displaying help information.

7. Building and Running the Docker Image

a. Build the Docker Image

Run the following command in the project root directory to build the Docker image:

docker build -t cdaprod/dev-environment-manager:latest .

Explanation:

	•	-t cdaprod/dev-environment-manager:latest: Tags the image with the name cdaprod/dev-environment-manager and the latest tag.
	•	.: Specifies the current directory as the build context.

b. Running the Docker Container

To run the dev-environment-manager tool inside a Docker container with your configuration, use the following command:

docker run --rm -it \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$PWD/config.yaml:/app/config.yaml" \
  cdaprod/dev-environment-manager:latest start my-project-dir my-repo

Explanation:

	•	--rm: Automatically removes the container when it exits.
	•	-it: Runs the container in interactive mode with a pseudo-TTY.
	•	-v /var/run/docker.sock:/var/run/docker.sock: Mounts the Docker socket to allow the container to communicate with the Docker daemon on the host.
	•	-v "$PWD/config.yaml:/app/config.yaml": Mounts your local config.yaml into the container, allowing you to modify configurations without rebuilding the image.
	•	cdaprod/dev-environment-manager:latest: Specifies the Docker image to run.
	•	start my-project-dir my-repo: Executes the start command with the specified project and repository names.

Note: Mounting the Docker socket (/var/run/docker.sock) grants the container full access to the Docker daemon on the host, which can be a security risk. Ensure you understand the implications before using this approach.

8. Usage Instructions

a. Adding a New Project

To add a new project and repository to your configuration:

docker run --rm -it \
  -v "$PWD/config.yaml:/app/config.yaml" \
  cdaprod/dev-environment-manager:latest add my-project-dir my-repo https://github.com/Cdaprod/my-repo.git

Explanation:

	•	add my-project-dir my-repo https://github.com/Cdaprod/my-repo.git: Adds a new repository under the specified project with the given Git URL.

b. Starting a Project Environment

To start the development environment for a specific project and repository:

docker run --rm -it \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$PWD/config.yaml:/app/config.yaml" \
  cdaprod/dev-environment-manager:latest start my-project-dir my-repo

Explanation:

	•	Volume Bindings: The tool automatically applies both default and project-specific volume bindings as defined in config.yaml.
	•	Neovim Session: Launches Neovim inside the Docker container with the specified configurations.

c. Managing Configurations

To add additional volume bindings or environment variables, simply update the config.yaml file.

Example: Adding tmux and Neovim Plugins Bindings

# config.yaml

# Global environment variables
environment_vars:
  - HOME=/home/cdaprod
  - CUSTOM_ENV_VAR=value

# Default volume bindings applicable to all projects
default_volume_bindings:
  - "$HOME/.config/nvim:/root/.config/nvim"
  - "$HOME/.vim:/root/.vim"
  - "$HOME/.vimrc:/root/.vimrc"
  - "$HOME/.tmux.conf:/root/.tmux.conf"
  - "$HOME/.local/share/nvim:/root/.local/share/nvim"

# Project-specific configurations
projects:
  my-project-dir:
    repos:
      my-repo:
        repo_url: "https://github.com/Cdaprod/my-repo.git"
        docker_image: "cdaprod/my-repo:latest"
        container_name: "nvim-my-repo"
        # Additional volume bindings specific to this repo
        volume_bindings:
          - "$HOME/Projects/my-project-dir/my-repo/plugins:/root/.local/share/nvim/plugins"
          - "$HOME/.tmux.conf:/root/.tmux.conf"

Explanation:

	•	Adding tmux Configurations: By adding "$HOME/.tmux.conf:/root/.tmux.conf" under default_volume_bindings or project-specific volume_bindings, you ensure that tmux configurations are available inside the Docker container.
	•	Adding Neovim Plugins: Similarly, adding "$HOME/Projects/my-project-dir/my-repo/plugins:/root/.local/share/nvim/plugins" mounts your Neovim plugins into the container.

Updating Configurations:

After modifying config.yaml, rerun the add or start commands as needed. The tool will automatically apply the new bindings based on the updated configurations.

9. Final Enhancements and Best Practices

	1.	Automate Configuration Expansion:
	•	Enhance the expandBindings function to handle more environment variables or patterns as needed.
	2.	Interactive Prompts:
	•	For more user-friendly interactions, implement interactive prompts when adding projects or repositories using libraries like survey.
	3.	Error Handling:
	•	Implement more granular error handling to cover edge cases, such as missing Docker installations or invalid configurations.
	4.	Logging Enhancements:
	•	Configure Logrus to output logs in different formats (e.g., JSON) or to log to files for persistent storage.
	5.	Security Considerations:
	•	Be cautious when mounting the Docker socket. Consider implementing security measures or limiting access if necessary.
	6.	Extensibility:
	•	Add more commands (e.g., stop, list, status) to manage Docker containers more effectively.
	•	Allow for plugin architectures where users can extend the tool’s functionalities.
	7.	Testing:
	•	Develop unit tests for your functions to ensure reliability and facilitate future changes.
	8.	Documentation:
	•	Create a comprehensive README.md that covers installation, usage, configuration, and troubleshooting.

10. Example config.yaml

Here’s a complete example of config.yaml demonstrating various configurations:

# config.yaml

# Global environment variables
environment_vars:
  - HOME=/home/cdaprod
  - CUSTOM_ENV_VAR=value

# Default volume bindings applicable to all projects
default_volume_bindings:
  - "$HOME/.config/nvim:/root/.config/nvim"
  - "$HOME/.vim:/root/.vim"
  - "$HOME/.vimrc:/root/.vimrc"
  - "$HOME/.tmux.conf:/root/.tmux.conf"
  - "$HOME/.local/share/nvim:/root/.local/share/nvim"

# Project-specific configurations
projects:
  my-project-dir:
    repos:
      my-repo:
        repo_url: "https://github.com/Cdaprod/my-repo.git"
        docker_image: "cdaprod/my-repo:latest"
        container_name: "nvim-my-repo"
        # Additional volume bindings specific to this repo
        volume_bindings:
          - "$HOME/Projects/my-project-dir/my-repo/plugins:/root/.local/share/nvim/plugins"
          - "$HOME/.tmux.conf:/root/.tmux.conf"
      another-repo:
        repo_url: "https://github.com/Cdaprod/another-repo.git"
        docker_image: "cdaprod/another-repo:latest"
        container_name: "nvim-another-repo"
        volume_bindings:
          - "$HOME/Projects/my-project-dir/another-repo/plugins:/root/.local/share/nvim/plugins"
          - "$HOME/.tmux.conf:/root/.tmux.conf"

11. Testing the Tool

a. Build the Application

docker build -t cdaprod/dev-environment-manager:latest .

b. Add a New Project

docker run --rm -it \
  -v "$PWD/config.yaml:/app/config.yaml" \
  cdaprod/dev-environment-manager:latest add my-project-dir my-repo https://github.com/Cdaprod/my-repo.git

Expected Outcome:

	•	Logs indicating the addition of the project.
	•	Entry added to config.yaml under projects.my-project-dir.repos.my-repo.

c. Start the Development Environment

docker run --rm -it \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$PWD/config.yaml:/app/config.yaml" \
  cdaprod/dev-environment-manager:latest start my-project-dir my-repo

Expected Outcome:

	1.	Cloning: If not already cloned, the repository is cloned into $HOME/Projects/my-project-dir/my-repo.
	2.	Docker Image: Pulls the specified Docker image (cdaprod/my-repo:latest).
	3.	Container Creation: Creates and starts the Docker container (nvim-my-repo) with all volume bindings.
	4.	Neovim Session: Attaches your terminal to Neovim running inside the container.
	5.	Cleanup: Removes the Docker container after exiting Neovim.

12. Conclusion

You’ve now created a robust, dynamic, and user-friendly CLI tool named dev-environment-manager that:

	•	Manages Multiple Projects and Repositories: Easily add and start development environments for various projects.
	•	Dynamic Configuration Management: Utilize external YAML configuration files to manage volume bindings and environment variables, eliminating hardcoding.
	•	Leverages Cobra and Viper: Harness the power of Cobra for CLI commands and Viper for configuration management.
	•	Docker Integration: Automate Docker container management with custom volume bindings and environment settings.

Next Steps:

	1.	Enhance Functionality:
	•	Implement additional commands like stop, list, status for better Docker container management.
	•	Add support for more environment configurations or tools as needed.
	2.	Improve Security:
	•	Reevaluate the necessity of mounting the Docker socket and explore safer alternatives if possible.
	3.	Documentation:
	•	Create a detailed README.md to guide users through installation, configuration, and usage.
	4.	Testing:
	•	Develop unit and integration tests to ensure the tool’s reliability and robustness.
	5.	Continuous Integration:
	•	Set up CI/CD pipelines to automate testing and deployment of the tool.

Feel free to customize and extend the tool further to better fit your specific needs. If you encounter any issues or need assistance with additional features, don’t hesitate to ask!

Happy coding!