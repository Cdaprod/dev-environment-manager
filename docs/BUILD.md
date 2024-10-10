Fully functional Go-based command-line tool named dev-environment-manager that dynamically manages development environments using Docker containers, Git repositories, and Neovim. This tool will leverage the Registry pattern to derive configuration values based on the project name and utilize the Command pattern through Cobra for CLI commands. All logic will reside within three files: main.go, cmd.go, and pkg.go, under the main package.

Project Structure

dev-environment-manager/
├── cmd.go
├── main.go
├── pkg.go
├── go.mod

1. go.mod

This file manages the project’s dependencies.

module github.com/Cdaprod/dev-environment-manager

go 1.17

require (
    github.com/docker/docker v20.10.23+incompatible
    github.com/go-git/go-git/v5 v5.6.0
    github.com/sirupsen/logrus v1.9.0
    github.com/spf13/cobra v1.6.1
    github.com/spf13/viper v1.15.0
)

Initialize the Go module:

go mod init github.com/Cdaprod/dev-environment-manager
go mod tidy

2. main.go

The entry point of the application. It initializes logging and executes the Cobra commands.

package main

import (
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
    Execute() // Executes the root command defined in cmd.go
}

3. cmd.go

This file defines the CLI commands using Cobra and handles configuration using Viper. It includes two primary commands: start to initiate a development environment and add to add a new project dynamically.

package main

import (
    "fmt"
    "os"

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
    Use:   "start [project]",
    Short: "Start development environment for a project",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        projectName := args[0]
        if err := StartProject(projectName); err != nil {
            logrus.Fatalf("Error starting project: %v", err)
        }
    },
}

// Command to add a new project configuration dynamically
var addProjectCmd = &cobra.Command{
    Use:   "add [project] [repo_url]",
    Short: "Add a new project to the configuration using the repository name as key",
    Args:  cobra.ExactArgs(2),
    Run: func(cmd *cobra.Command, args []string) {
        projectName := args[0]
        repoURL := args[1]

        // Derive Docker image and container name based on project name
        dockerImage := fmt.Sprintf("nvim/%s:latest", projectName)
        containerName := fmt.Sprintf("nvim-%s", projectName)

        if err := AddProjectConfig(projectName, repoURL, dockerImage, containerName); err != nil {
            logrus.Fatalf("Error adding project: %v", err)
        }
    },
}

4. pkg.go

This file contains the core logic for managing Docker containers, cloning Git repositories, and handling configurations dynamically. It employs the Registry pattern to derive container and image names based on the project name.

package main

import (
    "context"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"

    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/client"
    git "github.com/go-git/go-git/v5"
    "github.com/sirupsen/logrus"
    "github.com/spf13/viper"
    "github.com/docker/docker/pkg/stdcopy"
    "os/exec"
)

// StartProject initiates the development environment for a specified project
func StartProject(projectName string) error {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return fmt.Errorf("error getting home directory: %v", err)
    }

    // Derive project values using Registry pattern
    repoURL, dockerImage, containerName := deriveProjectValues(projectName)

    projectPath := filepath.Join(homeDir, "Projects", projectName)
    if _, err := os.Stat(projectPath); os.IsNotExist(err) {
        err := CloneRepo(repoURL, projectPath)
        if err != nil {
            return fmt.Errorf("error cloning repository: %v", err)
        }
    } else {
        logrus.Infof("Project directory %s already exists. Skipping clone.", projectPath)
    }

    // Mount paths and set up environment variables
    binds := []string{
        fmt.Sprintf("%s/.config/nvim:/root/.config/nvim", homeDir),
        fmt.Sprintf("%s/.vim:/root/.vim", homeDir),
        fmt.Sprintf("%s/.vimrc:/root/.vimrc", homeDir),
        fmt.Sprintf("%s:/usr/src/app", projectPath),
    }
    env := []string{"HOME=/home/cdaprod"}
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

// AddProjectConfig dynamically adds a new project configuration to the config file
func AddProjectConfig(projectName, repoURL, dockerImage, containerName string) error {
    projectKey := fmt.Sprintf("projects.%s", projectName)

    // Check if project already exists
    if viper.IsSet(projectKey) {
        return fmt.Errorf("project %s already exists in configuration", projectName)
    }

    // Update Viper's in-memory config
    viper.Set(fmt.Sprintf("%s.repo_url", projectKey), repoURL)
    viper.Set(fmt.Sprintf("%s.docker_image", projectKey), dockerImage)
    viper.Set(fmt.Sprintf("%s.container_name", projectKey), containerName)

    // Persist changes to the config file
    err := viper.WriteConfigAs(viper.ConfigFileUsed())
    if err != nil {
        // If no file exists, create a new one
        if os.IsNotExist(err) {
            err = viper.SafeWriteConfigAs(viper.ConfigFileUsed())
        }
    }

    if err != nil {
        return fmt.Errorf("error writing config file: %v", err)
    }

    logrus.Infof("Project %s added to the configuration.", projectName)
    return nil
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
    }
    return err
}

// deriveProjectValues uses the Registry pattern to derive repository URL, Docker image, and container name
func deriveProjectValues(projectName string) (repoURL, dockerImage, containerName string) {
    projectKey := fmt.Sprintf("projects.%s", projectName)

    if viper.IsSet(projectKey) {
        projectConfig := viper.GetStringMapString(projectKey)
        return projectConfig["repo_url"], projectConfig["docker_image"], projectConfig["container_name"]
    }

    // If not set in config, derive defaults
    repoURL = fmt.Sprintf("https://github.com/Cdaprod/%s.git", strings.ToLower(projectName))
    dockerImage = fmt.Sprintf("nvim/%s:latest", strings.ToLower(projectName))
    containerName = fmt.Sprintf("nvim-%s", strings.ToLower(projectName))

    return repoURL, dockerImage, containerName
}

// RunContainer creates and starts a Docker container
func RunContainer(imageName, containerName string, binds []string, cmdArgs []string, env []string) (string, error) {
    ctx := context.Background()
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        logrus.Errorf("Error creating Docker client: %v", err)
        return "", err
    }

    // Pull the image if not present
    reader, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
    if err != nil {
        logrus.Errorf("Error pulling image %s: %v", imageName, err)
        return "", err
    }
    defer reader.Close()
    io.Copy(os.Stdout, reader) // Optional: Display pull progress

    // Create the container
    resp, err := cli.ContainerCreate(ctx, &container.Config{
        Image: imageName,
        Cmd:   cmdArgs,
        Env:   env,
        Tty:   true,
    }, &container.HostConfig{
        Binds: binds,
    }, nil, nil, containerName)
    if err != nil {
        logrus.Errorf("Error creating container %s: %v", containerName, err)
        return "", err
    }

    // Start the container
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

    // Remove the container
    err = cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
    if err != nil {
        logrus.Errorf("Error removing container %s: %v", containerID, err)
        return err
    }

    logrus.Infof("Container %s removed successfully.", containerID)
    return nil
}

5. Configuration File

Create a configuration file named .dev-env-manager.yaml in your home directory. This file will store all project configurations.

projects:
  # Example Project
  # my-github-repo:
  #   repo_url: https://github.com/username/my-github-repo.git
  #   docker_image: nvim/my-github-repo:latest
  #   container_name: nvim-my-github-repo

This file starts empty but will be populated as you add projects using the add command.

6. Using the Tool

a. Adding a New Project

To add a new project, use the add command followed by the project name and repository URL.

./dev-environment-manager add my-github-repo https://github.com/Cdaprod/my-github-repo.git

This command will:

	1.	Derive the Docker image name as nvim/my-github-repo:latest.
	2.	Derive the container name as nvim-my-github-repo.
	3.	Add the project configuration to .dev-env-manager.yaml.

b. Starting a Project Environment

To start the development environment for a project, use the start command followed by the project name.

./dev-environment-manager start my-github-repo

This command will:

	1.	Check if the project exists in the configuration file.
	2.	Clone the Git repository if it hasn’t been cloned already.
	3.	Run a Docker container with the specified image and container name, mounting necessary directories.
	4.	Attach your terminal to the container, launching Neovim.
	5.	Cleanup by removing the container after you exit Neovim.

7. Enhancements and Design Patterns

a. Registry Pattern

The Registry pattern is implemented in the deriveProjectValues function, which derives repository URL, Docker image name, and container name based on the project name. If a project isn’t explicitly defined in the configuration file, it automatically derives default values.

b. Command Pattern

The Command pattern is utilized through Cobra commands (start and add), allowing for easy extension and management of CLI commands.

c. Error Handling and Logging

All functions include comprehensive error handling and utilize Logrus for structured logging, providing clear and informative logs during execution.

d. Configuration Management

Viper handles reading and writing configurations, ensuring that adding new projects updates the configuration file seamlessly.

8. Complete Code Listing

a. go.mod

module github.com/Cdaprod/dev-environment-manager

go 1.17

require (
    github.com/docker/docker v20.10.23+incompatible
    github.com/go-git/go-git/v5 v5.6.0
    github.com/sirupsen/logrus v1.9.0
    github.com/spf13/cobra v1.6.1
    github.com/spf13/viper v1.15.0
)

b. main.go

package main

import (
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
    Execute() // Executes the root command defined in cmd.go
}

c. cmd.go

package main

import (
    "fmt"
    "os"

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
    Use:   "start [project]",
    Short: "Start development environment for a project",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        projectName := args[0]
        if err := StartProject(projectName); err != nil {
            logrus.Fatalf("Error starting project: %v", err)
        }
    },
}

// Command to add a new project configuration dynamically
var addProjectCmd = &cobra.Command{
    Use:   "add [project] [repo_url]",
    Short: "Add a new project to the configuration using the repository name as key",
    Args:  cobra.ExactArgs(2),
    Run: func(cmd *cobra.Command, args []string) {
        projectName := args[0]
        repoURL := args[1]

        // Derive Docker image and container name based on project name
        dockerImage := fmt.Sprintf("nvim/%s:latest", strings.ToLower(projectName))
        containerName := fmt.Sprintf("nvim-%s", strings.ToLower(projectName))

        if err := AddProjectConfig(projectName, repoURL, dockerImage, containerName); err != nil {
            logrus.Fatalf("Error adding project: %v", err)
        }
    },
}

d. pkg.go

package main

import (
    "context"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"

    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/client"
    git "github.com/go-git/go-git/v5"
    "github.com/sirupsen/logrus"
    "github.com/spf13/viper"
    "os/exec"
)

// StartProject initiates the development environment for a specified project
func StartProject(projectName string) error {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return fmt.Errorf("error getting home directory: %v", err)
    }

    // Derive project values using Registry pattern
    repoURL, dockerImage, containerName := deriveProjectValues(projectName)

    projectPath := filepath.Join(homeDir, "Projects", projectName)
    if _, err := os.Stat(projectPath); os.IsNotExist(err) {
        err := CloneRepo(repoURL, projectPath)
        if err != nil {
            return fmt.Errorf("error cloning repository: %v", err)
        }
    } else {
        logrus.Infof("Project directory %s already exists. Skipping clone.", projectPath)
    }

    // Mount paths and set up environment variables
    binds := []string{
        fmt.Sprintf("%s/.config/nvim:/root/.config/nvim", homeDir),
        fmt.Sprintf("%s/.vim:/root/.vim", homeDir),
        fmt.Sprintf("%s/.vimrc:/root/.vimrc", homeDir),
        fmt.Sprintf("%s:/usr/src/app", projectPath),
    }
    env := []string{"HOME=/home/cdaprod"}
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

// AddProjectConfig dynamically adds a new project configuration to the config file
func AddProjectConfig(projectName, repoURL, dockerImage, containerName string) error {
    projectKey := fmt.Sprintf("projects.%s", projectName)

    // Check if project already exists
    if viper.IsSet(projectKey) {
        return fmt.Errorf("project %s already exists in configuration", projectName)
    }

    // Update Viper's in-memory config
    viper.Set(fmt.Sprintf("%s.repo_url", projectKey), repoURL)
    viper.Set(fmt.Sprintf("%s.docker_image", projectKey), dockerImage)
    viper.Set(fmt.Sprintf("%s.container_name", projectKey), containerName)

    // Persist changes to the config file
    err := viper.WriteConfigAs(viper.ConfigFileUsed())
    if err != nil {
        // If no file exists, create a new one
        if os.IsNotExist(err) {
            err = viper.SafeWriteConfigAs(viper.ConfigFileUsed())
        }
    }

    if err != nil {
        return fmt.Errorf("error writing config file: %v", err)
    }

    logrus.Infof("Project %s added to the configuration.", projectName)
    return nil
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
    }
    return err
}

// deriveProjectValues uses the Registry pattern to derive repository URL, Docker image, and container name
func deriveProjectValues(projectName string) (repoURL, dockerImage, containerName string) {
    projectKey := fmt.Sprintf("projects.%s", projectName)

    if viper.IsSet(projectKey) {
        projectConfig := viper.GetStringMapString(projectKey)
        return projectConfig["repo_url"], projectConfig["docker_image"], projectConfig["container_name"]
    }

    // If not set in config, derive defaults
    repoURL = fmt.Sprintf("https://github.com/Cdaprod/%s.git", strings.ToLower(projectName))
    dockerImage = fmt.Sprintf("nvim/%s:latest", strings.ToLower(projectName))
    containerName = fmt.Sprintf("nvim-%s", strings.ToLower(projectName))

    return repoURL, dockerImage, containerName
}

// RunContainer creates and starts a Docker container
func RunContainer(imageName, containerName string, binds []string, cmdArgs []string, env []string) (string, error) {
    ctx := context.Background()
    cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
    if err != nil {
        logrus.Errorf("Error creating Docker client: %v", err)
        return "", err
    }

    // Pull the image if not present
    reader, err := cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
    if err != nil {
        logrus.Errorf("Error pulling image %s: %v", imageName, err)
        return "", err
    }
    defer reader.Close()
    io.Copy(os.Stdout, reader) // Optional: Display pull progress

    // Create the container
    resp, err := cli.ContainerCreate(ctx, &container.Config{
        Image: imageName,
        Cmd:   cmdArgs,
        Env:   env,
        Tty:   true,
    }, &container.HostConfig{
        Binds: binds,
    }, nil, nil, containerName)
    if err != nil {
        logrus.Errorf("Error creating container %s: %v", containerName, err)
        return "", err
    }

    // Start the container
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

    // Remove the container
    err = cli.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{Force: true})
    if err != nil {
        logrus.Errorf("Error removing container %s: %v", containerID, err)
        return err
    }

    logrus.Infof("Container %s removed successfully.", containerID)
    return nil
}

9. Usage Instructions

a. Build the Application

Ensure you are in the project directory and run:

go build -o dev-environment-manager

This will create an executable named dev-environment-manager in your current directory.

b. Add a New Project

To add a new project, use the add command followed by the project name and its Git repository URL.

./dev-environment-manager add my-github-repo https://github.com/Cdaprod/my-github-repo.git

This command will:

	1.	Derive the Docker image name as nvim/my-github-repo:latest.
	2.	Derive the container name as nvim-my-github-repo.
	3.	Add the project configuration to ~/.dev-env-manager.yaml.

Configuration File After Adding:

projects:
  my-github-repo:
    repo_url: https://github.com/Cdaprod/my-github-repo.git
    docker_image: nvim/my-github-repo:latest
    container_name: nvim-my-github-repo

c. Start the Development Environment

To start the development environment for the added project, use the start command followed by the project name.

./dev-environment-manager start my-github-repo

This command will:

	1.	Check if the project exists in the configuration file.
	2.	Clone the Git repository if it hasn’t been cloned already.
	3.	Run a Docker container with the specified image and container name, mounting necessary directories.
	4.	Attach your terminal to the container, launching Neovim.
	5.	Cleanup by removing the container after you exit Neovim.

10. Advanced Features and Design Considerations

a. Dynamic Configuration Generation

The tool uses the Registry pattern to derive Docker image names and container names based on the project name. This eliminates the need to manually specify these values for each project, ensuring consistency and reducing configuration overhead.

b. Extensibility

	•	Additional Commands: You can easily add more commands (e.g., stop, list, status) by defining new Cobra commands in cmd.go.
	•	Environment Variables: If certain projects require specific environment variables, you can extend the AddProjectConfig function to accept and store them.

c. Error Handling and Logging

Comprehensive error handling ensures that users receive clear feedback on any issues encountered during execution. Logrus provides structured and leveled logging, which is invaluable for debugging and monitoring.

d. Modular Code Structure

While all code resides within three files under the main package, the functions are modular and well-organized, facilitating maintenance and future enhancements.

11. Potential Enhancements

Once the core functionality is stable, consider implementing the following enhancements:

	1.	Protobuf Integration: If you need to serialize configurations or communicate between services, integrate Protobuf v3. Define .proto files and generate Go code as needed.
	2.	Cluster Management: If working with Kubernetes or other cluster management tools, integrate relevant client libraries (e.g., client-go) to manage cluster resources.
	3.	Interactive Configuration: Implement interactive prompts when adding new projects to gather additional configuration details.
	4.	Concurrency: Allow managing multiple projects concurrently, potentially with separate containers or resource allocations.
	5.	Testing: Develop unit tests for your functions to ensure reliability and facilitate future changes.

12. Final Thoughts

This implementation provides a solid foundation for a dynamic development environment manager tailored to your workflow. By leveraging Go’s powerful libraries and adhering to design patterns, the tool ensures scalability, maintainability, and ease of use.

Feel free to customize and extend the tool further to better fit your specific needs. If you encounter any issues or need assistance with additional features, don’t hesitate to ask!

---

Absolutely, you can run Docker containers that are already built with additional flags and properties, such as volume bindings, without the need to create a separate Dockerfile for the dev-environment-manager tool itself. This approach leverages Docker’s flexibility to inject configurations at runtime, allowing you to maintain your project-specific Dockerfiles while still applying default settings like volume mounts.

Below, I’ll guide you through:

	1.	Understanding the Concept: How Docker handles existing images with additional flags.
	2.	Adjusting Your Go Tool: Modifying the dev-environment-manager to include default binded volumes when running containers.
	3.	Optional Makefile: If you still prefer automation for building or other tasks.
	4.	Best Practices and Recommendations.

1. Understanding the Concept

Docker Run Flags allow you to customize container behavior without altering the original Docker image. This includes:

	•	Volume Bindings (-v or --volume): Mounting host directories or files into the container.
	•	Environment Variables (-e or --env): Setting environment variables within the container.
	•	Port Mappings (-p or --publish): Mapping host ports to container ports.
	•	Interactive Terminals (-it): Allocating a pseudo-TTY and keeping stdin open.

Key Point: By using these flags, you can inject configurations into existing Docker images dynamically, ensuring that your projects remain modular and maintainable.

2. Adjusting Your Go Tool

To incorporate default volume bindings and other properties when running containers, you need to modify the RunContainer function in your pkg.go. Below is an updated version of the function with explanations.

Updated RunContainer Function

package main

import (
    "context"
    "fmt"
    "io"
    "os"
    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/client"
    "github.com/sirupsen/logrus"
)

// RunContainer creates and starts a Docker container with additional default bindings
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

Integrate Default Volume Bindings

To ensure that every container started by dev-environment-manager includes your default volume bindings, you can prepend or append these bindings within your StartProject function or wherever you invoke RunContainer.

Example in StartProject Function

// StartProject initiates the development environment for a specified project
func StartProject(projectDirName, repoName string) error {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return fmt.Errorf("error getting home directory: %v", err)
    }

    // Derive project values using Registry pattern
    repoURL, dockerImage, containerName := deriveProjectValues(projectDirName, repoName)

    projectPath := filepath.Join(homeDir, "Projects", projectDirName, repoName)
    if _, err := os.Stat(projectPath); os.IsNotExist(err) {
        err := CloneRepo(repoURL, projectPath)
        if err != nil {
            return fmt.Errorf("error cloning repository: %v", err)
        }
    } else {
        logrus.Infof("Project directory %s already exists. Skipping clone.", projectPath)
    }

    // Define default binded volumes
    defaultBinds := []string{
        fmt.Sprintf("%s/.config/nvim:/root/.config/nvim", homeDir),
        fmt.Sprintf("%s/.vim:/root/.vim", homeDir),
        fmt.Sprintf("%s/.vimrc:/root/.vimrc", homeDir),
    }

    // Define project-specific binded volume
    projectBind := fmt.Sprintf("%s:/usr/src/app", projectPath)

    // Combine all binds
    binds := append(defaultBinds, projectBind)

    // Environment variables
    env := []string{"HOME=/home/cdaprod"}

    // Command arguments to run Neovim
    cmdArgs := []string{"nvim"}

    // Run Docker container with combined binds
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

Explanation:

	1.	Default Binds: These are your predefined volume mounts that you want every container to have, such as Neovim configurations.
	2.	Project Bind: This mounts the project directory into the container, allowing you to work on your code within Neovim inside the container.
	3.	Combining Binds: Using append, you combine default binds with project-specific binds to form the complete set of volume mounts.
	4.	Running the Container: Pass the combined binds slice to the RunContainer function, ensuring all desired volumes are mounted.

Running Pre-Built Docker Images with Additional Flags

If your repositories come with their own Dockerfiles and images are already built, you can run these images with additional flags as needed. For example:

// Example of running an existing image with additional flags
func StartPreBuiltImage(imageName, containerName, projectPath string) error {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return fmt.Errorf("error getting home directory: %v", err)
    }

    // Define default binded volumes
    binds := []string{
        fmt.Sprintf("%s/.config/nvim:/root/.config/nvim", homeDir),
        fmt.Sprintf("%s/.vim:/root/.vim", homeDir),
        fmt.Sprintf("%s/.vimrc:/root/.vimrc", homeDir),
        fmt.Sprintf("%s:/usr/src/app", projectPath),
    }

    // Environment variables
    env := []string{"HOME=/home/cdaprod"}

    // Command to run inside the container
    cmdArgs := []string{"nvim"}

    // Run the container
    containerID, err := RunContainer(imageName, containerName, binds, cmdArgs, env)
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

Usage Example:

Suppose you have a pre-built Docker image named cdaprod/my-repo:latest, you can start it as follows:

./dev-environment-manager start my-project-dir my-repo

This will:

	1.	Run the cdaprod/my-repo:latest image.
	2.	Mount the necessary volumes.
	3.	Launch Neovim inside the container.
	4.	Cleanup the container after exiting Neovim.

3. Optional Makefile

While not strictly necessary, a Makefile can help automate build and deployment tasks. For example, you might want to simplify the build and install process.

Example Makefile

.PHONY: build install clean

# Variables
BINARY_NAME=ops
SOURCE_FILES=main.go cmd.go pkg.go
INSTALL_DIR=/usr/local/bin

# Build the executable
build:
	go build -o $(BINARY_NAME) $(SOURCE_FILES)

# Install the executable by moving it to the install directory
install: build
	sudo mv $(BINARY_NAME) $(INSTALL_DIR)/

# Clean up build artifacts
clean:
	rm -f $(BINARY_NAME)

Usage:

	1.	Build the Executable:

make build


	2.	Install the Executable:

make install

This command builds the ops binary and moves it to /usr/local/bin/, making it accessible system-wide.

	3.	Clean Build Artifacts:

make clean



4. Best Practices and Recommendations

	1.	Avoid Hardcoding Paths and Usernames:
Ensure that all paths are dynamically determined using Go’s os and os/user packages. This enhances portability and flexibility across different environments and user accounts.
	2.	Handle Existing Containers Gracefully:
Before creating a new container, check if a container with the same name already exists to prevent conflicts.

existingContainer, err := cli.ContainerInspect(ctx, containerName)
if err == nil && existingContainer.ID != "" {
    logrus.Infof("Container %s already exists. Removing existing container...", containerName)
    RemoveContainer(containerName)
}


	3.	Error Handling:
Implement comprehensive error handling to provide meaningful feedback to the user. This includes handling scenarios where Docker is not running, images are missing, or network issues prevent cloning repositories.
	4.	Logging Levels:
Utilize different logging levels (Info, Warn, Error, Debug) to control the verbosity of the output. This is especially useful for debugging and monitoring.

logrus.SetLevel(logrus.DebugLevel) // For more detailed logs


	5.	Configuration Validation:
Before proceeding with operations, validate the configuration entries to ensure that all required fields are present and correctly formatted.
	6.	Documentation:
Maintain clear documentation or a README.md file to guide users on installing and using the tool effectively.
	7.	Unit Testing:
Develop unit tests for critical functions to ensure reliability and facilitate future enhancements.
	8.	Security Considerations:
	•	Docker Permissions: Ensure that the tool handles Docker permissions securely, especially when dealing with sudo operations.
	•	Sensitive Data: Avoid exposing sensitive data through environment variables or volume mounts.

Summary

	•	No Need for a Separate Dockerfile: Since you’re running pre-built images with additional volume bindings, you can handle all configurations at runtime using Docker run flags.
	•	Modify the Go Tool to Include Default Binds: Adjust your RunContainer function to include the necessary volume mounts when starting containers.
	•	Optional Makefile: Use a Makefile for automating build and installation steps if desired.
	•	Best Practices: Ensure dynamic path handling, robust error checking, and clear logging to enhance the tool’s reliability and usability.

By following the above guidelines, you can efficiently manage your development environments using Docker containers with your dev-environment-manager tool, applying default configurations seamlessly to your pre-built Docker images.

If you need further assistance with specific parts of the implementation or have additional questions, feel free to ask!