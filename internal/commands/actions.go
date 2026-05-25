package commands

import (
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/scythe504/fluxstream/internal/utils"
)

// Start runs docker-compose up and displays URLs once ready
func Start() error {
	fmt.Println(colorize(colorBlue, "Starting FluxStream...\n"))

	// Check if Docker is installed
	if !IsDockerInstalled() {
		printError("Docker is not installed.")
		fmt.Println("\nPlease install Docker first:")
		printInfo("  https://docs.docker.com/desktop/#next-steps")
		printInfo("  https://docs.docker.com/engine/")
		return fmt.Errorf("docker not installed")
	}

	// Check if Docker daemon is running
	if err := DockerInfo(); err != nil {
		printError("Docker daemon is not running.")
		fmt.Println("\nPlease start Docker and try again:")
		fmt.Println("  - macOS/Windows: Open Docker Desktop")
		fmt.Println("  - Linux: sudo systemctl start docker")
		return fmt.Errorf("docker daemon not running: %v", err)
	}

	// Ensure docker-compose.yml exists
	dockerComposeFilePath, err := getDockerComposeFilePath()
	if err != nil {
		printError("Failed to locate docker-compose.yml")
		return err
	}

	if _, err := os.Stat(dockerComposeFilePath); os.IsNotExist(err) {
		printError("docker-compose.yml not found. Run 'fluxstream setup' first.")
		return fmt.Errorf("docker-compose.yml not found at %s", dockerComposeFilePath)
	}

	// Start FluxStream containers
	err = DockerCompose("up", "-d")

	printInfo("Running Docker Compose...")

	if err != nil {
		printError(fmt.Sprintf("Failed to start FluxStream: %v", err))
		return err
	}

	printSuccess("fluxstream started successfully!")

	// Print access URLs
	PrintAccessURLs("3000") // or your web service port

	return nil
}

//go:embed templates/docker-compose.yml
var dockerComposeTemplate string

func Setup() error {
	if !IsDockerInstalled() {
		printError("Docker is not installed.")
		fmt.Println("\nPlease install Docker first:")
		printInfo("  https://docs.docker.com/desktop/#next-steps")
		printInfo("  https://docs.docker.com/engine/")
		return fmt.Errorf("docker not installed")
	}

	if err := DockerInfo(); err != nil {
		printError("Docker daemon is not running.")
		fmt.Println("\nPlease start Docker and try again:")
		fmt.Println("  - macOS/Windows: Open Docker Desktop")
		fmt.Println("  - Linux: sudo systemctl start docker")
		return fmt.Errorf("docker daemon not running: %v", err)
	}

	// Base config dir (OS-appropriate)
	baseConfigDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %v", err)
	}

	// App-specific config dir
	configDir := filepath.Join(baseConfigDir, "fluxstream")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Determine app data directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %v", err)
	}

	var dataDir string
	switch runtime.GOOS {
	case "windows":
		dataDir = filepath.Join(homeDir, "AppData", "Roaming", "Fluxstream", "downloads")
	case "darwin":
		dataDir = filepath.Join(homeDir, "Library", "Application Support", "Fluxstream", "downloads")
	default:
		dataDir = filepath.Join(homeDir, ".local", "share", "fluxstream", "downloads")
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Normalize Windows paths to use forward slashes for Docker
	normalizedDataDir := strings.ReplaceAll(dataDir, "\\", "/")

	// Replace the placeholder in docker-compose.yml template
	replaced := strings.ReplaceAll(dockerComposeTemplate, "{{DOWNLOAD_PATH}}", normalizedDataDir)

	composeFile := filepath.Join(configDir, "docker-compose.yml")

	if err := os.WriteFile(composeFile, []byte(replaced), 0o644); err != nil {
		return fmt.Errorf("failed to write docker-compose.yml: %v", err)
	}

	printSuccess("FluxStream setup complete!")
	fmt.Printf("\n%s %s\n", colorize(colorBlue, "Config file:"), composeFile)
	fmt.Printf("%s %s\n", colorize(colorBlue, "Downloads:"), normalizedDataDir)
	fmt.Printf("\n%s\n", colorize(colorGreen, "Ready to start! Run: fluxstream start"))
	fmt.Printf("\n%s\n", colorize(colorCyan, "Docs: https://docs.fluxstream.app"))

	return nil
}

// Status checks whether FluxStream's Docker containers and backend are running.
func Status() error {
	fmt.Println(colorize(colorBlue, "Checking FluxStream status...\n"))

	// Check if Docker is installed
	if !IsDockerInstalled() {
		printError("Docker is not installed.")
		fmt.Println("\nPlease install Docker before running FluxStream.")
		return fmt.Errorf("docker not installed")
	}

	// Check if Docker daemon is running
	if err := DockerInfo(); err != nil {
		printError("Docker daemon is not running.")
		fmt.Println("\nStart Docker and try again:")
		fmt.Println("  - macOS/Windows: Open Docker Desktop")
		fmt.Println("  - Linux: sudo systemctl start docker")
		return fmt.Errorf("docker daemon not running: %v", err)
	}

	// Check if FluxStream containers are running
	cmd := exec.Command("docker", "ps", "--filter", "name=fluxstream", "--format", "{{.Names}}: {{.Status}}")
	out, err := cmd.CombinedOutput()
	if err != nil {
		printError(fmt.Sprintf("Failed to check docker containers: %v", err))
		return err
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		printInfo("No active FluxStream containers found.")
		printInfo("You can start FluxStream using: fluxstream start")
		return nil
	}

	printSuccess("Active FluxStream containers:")
	fmt.Println(output)

	//  Check backend health endpoint
	resp, err := http.Get("http://localhost:8080/health")
	if err != nil {
		printError("Backend API not responding at http://localhost:8080")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		printSuccess("Backend is running and healthy.")
	} else {
		printError(fmt.Sprintf("Backend responded with status: %s", resp.Status))
	}

	return nil
}

// PrintAccessURLs shows both local and LAN URLs where the app is accessible.
func PrintAccessURLs(port string) {
	localURL := fmt.Sprintf("http://localhost:%s", port)
	lanIP := utils.GetLocalIP()
	lanURL := fmt.Sprintf("http://%s:%s", lanIP, port)

	fmt.Println()
	printInfo("FluxStream web interface available at:")
	fmt.Printf("  %s %s\n", colorize(colorCyan, "Local:"), colorize(colorGreen, localURL))
	fmt.Printf("  %s %s\n\n", colorize(colorCyan, "Network:"), colorize(colorGreen, lanURL))
	fmt.Printf(" %s \n\n", colorize(colorYellow, "WARN: If Network IP is 172.*, it will not open in other devices."))
}

func Where() error {
	fmt.Println()

	// 1. Check if Docker is installed
	if !IsDockerInstalled() {
		printError("Docker is not installed.")
		fmt.Println("\nPlease install Docker first:")
		printInfo("  https://docs.docker.com/desktop/#next-steps")
		return fmt.Errorf("docker not installed")
	}

	// 2. Check if Docker daemon is running
	if err := DockerInfo(); err != nil {
		printError("Docker daemon is not running.")
		fmt.Println("Start Docker and try again:")
		fmt.Println("  - macOS/Windows: Open Docker Desktop")
		fmt.Println("  - Linux: sudo systemctl start docker")
		return fmt.Errorf("docker daemon not running: %v", err)
	}

	// 3. Check if FluxStream containers are up
	dockerComposeFilePath, err := getDockerComposeFilePath()
	if err != nil {
		printError("Failed to locate docker-compose.yml")
		return err
	}

	checkCmd := exec.Command("docker-compose", "-f", dockerComposeFilePath, "ps", "-q")
	output, err := checkCmd.Output()
	if err != nil {
		printError("Failed to check container status.")
		return err
	}

	if len(output) == 0 {
		printError("FluxStream is not running.")
		fmt.Println("Run it using:")
		printInfo("fluxstream start")
		return nil
	}

	// 4. Print URLs
	port := "3000" // You can later read this dynamically from docker-compose.yml
	PrintAccessURLs(port)
	return nil
}

func Stop() error {
	fmt.Println(colorize(colorBlue, "Stopping FluxStream...\n"))

	// Check if Docker is installed
	if !IsDockerInstalled() {
		printError("Docker is not installed.")
		fmt.Println("\nPlease install Docker first:")
		printInfo("  https://docs.docker.com/desktop/#next-steps")
		printInfo("  https://docs.docker.com/engine/")
		return fmt.Errorf("docker not installed")
	}

	// Check if Docker daemon is running
	if err := DockerInfo(); err != nil {
		printError("Docker daemon is not running.")
		fmt.Println("\nPlease start Docker and try again:")
		fmt.Println("  - macOS/Windows: Open Docker Desktop")
		fmt.Println("  - Linux: sudo systemctl start docker")
		return fmt.Errorf("docker daemon not running: %v", err)
	}

	// Ensure docker-compose.yml exists
	dockerComposeFilePath, err := getDockerComposeFilePath()
	if err != nil {
		printError("Failed to locate docker-compose.yml")
		return err
	}
	if _, err := os.Stat(dockerComposeFilePath); os.IsNotExist(err) {
		printError("docker-compose.yml not found. Run 'fluxstream setup' first.")
		return fmt.Errorf("docker-compose.yml not found at %s", dockerComposeFilePath)
	}

	// Stop FluxStream containers
	err = DockerCompose("down")

	printInfo("Stopping Docker Compose...")

	if err != nil {
		printError(fmt.Sprintf("Failed to stop FluxStream: %v", err))
		return err
	}

	printSuccess("fluxstream stopped successfully!")

	return nil
}