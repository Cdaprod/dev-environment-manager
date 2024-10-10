// pkg.go
// This file contains helper functions and packages for Docker, Git, and other operations.
package main

import (
    "context"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "strings"
    "os/exec"

    "github.com/docker/docker/api/types"
    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/client"
    git "github.com/go-git/go-git/v5"
    "github.com/sirupsen/logrus"
    "github.com/spf13/viper"
    "os/user"
)

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

    // Automatically detect and set volume bindings
    binds := getVolumeBindings(homeDir, projectPath)

    // Environment variables
    env := []string{"HOME=/home/cdaprod"}

    // Command to run Neovim
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

// getVolumeBindings dynamically generates volume bindings
func getVolumeBindings(homeDir, projectPath string) []string {
    // Default binds for config files
    binds := []string{
        fmt.Sprintf("%s/.config/nvim:/root/.config/nvim", homeDir),
        fmt.Sprintf("%s/.vim:/root/.vim", homeDir),
        fmt.Sprintf("%s/.vimrc:/root/.vimrc", homeDir),
        fmt.Sprintf("%s:/usr/src/app", projectPath),
    }
    return binds
}

// AddProjectConfig dynamically adds a new project configuration to the config file
func AddProjectConfig(projectDirName, repoName, repoURL, dockerImage, containerName string) error {
    username, err := getUsername()
    if err != nil {
        return fmt.Errorf("error getting username: %v", err)
    }

    projectKey := fmt.Sprintf("users.%s.projects.%s.repos.%s", username, projectDirName, repoName)

    // Check if repository already exists
    if viper.IsSet(projectKey) {
        return fmt.Errorf("repository %s already exists under project %s for user %s", repoName, projectDirName, username)
    }

    // Update Viper's in-memory config
    viper.Set(fmt.Sprintf("%s.repo_url", projectKey), repoURL)
    viper.Set(fmt.Sprintf("%s.docker_image", projectKey), dockerImage)
    viper.Set(fmt.Sprintf("%s.container_name", projectKey), containerName)

    // Persist changes to the config file
    err = viper.WriteConfigAs(viper.ConfigFileUsed())
    if err != nil {
        // If no file exists, create a new one
        if os.IsNotExist(err) {
            err = viper.SafeWriteConfigAs(viper.ConfigFileUsed())
            if err != nil {
                return fmt.Errorf("error creating config file: %v", err)
            }
        } else {
            return fmt.Errorf("error writing config file: %v", err)
        }
    }

    logrus.Infof("Repository %s added under project %s for user %s.", repoName, projectDirName, username)
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
func deriveProjectValues(projectDirName, repoName string) (repoURL, dockerImage, containerName string) {
    username, err := getUsername()
    if err != nil {
        logrus.Warnf("Unable to get username, deriving defaults: %v", err)
    }

    projectKey := fmt.Sprintf("users.%s.projects.%s.repos.%s", username, projectDirName, repoName)

    if viper.IsSet(projectKey) {
        projectConfig := viper.GetStringMapString(projectKey)
        return projectConfig["repo_url"], projectConfig["docker_image"], projectConfig["container_name"]
    }

    // If not set in config, derive defaults
    repoURL = fmt.Sprintf("https://github.com/Cdaprod/%s.git", strings.ToLower(repoName))
    dockerImage = fmt.Sprintf("cdaprod/%s:latest", strings.ToLower(repoName))
    containerName = fmt.Sprintf("nvim-%s", strings.ToLower(repoName))

    return repoURL, dockerImage, containerName
}

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
    usr, err := user.Current()
    if err != nil {
        return "", err
    }
    // On Unix, usr.Username might include the path like "/home/cdaprod", so extract the actual username
    username := usr.Username
    if strings.Contains(username, "/") {
        parts := strings.Split(username, "/")
        username = parts[len(parts)-1]
    }
    return username, nil
}