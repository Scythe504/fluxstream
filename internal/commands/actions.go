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
	printHeader("STARTING FLUXSTREAM")

	// Check if Docker is installed
	printStep("Verifying system dependencies...")
	if !IsDockerInstalled() {
		printError("Docker is not installed.")
		fmt.Println("\nPlease install Docker first:")
		printInfo("https://docs.docker.com/desktop/#next-steps")
		printInfo("https://docs.docker.com/engine/")
		return fmt.Errorf("docker not installed")
	}
	printSubstep("Docker engine is installed")

	// Check if Docker daemon is running
	if err := DockerInfo(); err != nil {
		printError("Docker daemon is not running.")
		fmt.Println("\nPlease start Docker and try again:")
		printSubstep("macOS/Windows: Open Docker Desktop")
		printSubstep("Linux: sudo systemctl start docker")
		return fmt.Errorf("docker daemon not running: %v", err)
	}
	printSubstep("Docker daemon is active and running")

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
	printStep("Launching containers via Docker Compose...")
	err = DockerCompose("up", "-d")
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
	printHeader("FLUXSTREAM SETUP")

	printStep("Verifying system dependencies...")
	if !IsDockerInstalled() {
		printError("Docker is not installed.")
		fmt.Println("\nPlease install Docker first:")
		printInfo("https://docs.docker.com/desktop/#next-steps")
		printInfo("https://docs.docker.com/engine/")
		return fmt.Errorf("docker not installed")
	}

	if err := DockerInfo(); err != nil {
		printError("Docker daemon is not running.")
		fmt.Println("\nPlease start Docker and try again:")
		printSubstep("macOS/Windows: Open Docker Desktop")
		printSubstep("Linux: sudo systemctl start docker")
		return fmt.Errorf("docker daemon not running: %v", err)
	}
	printSubstep("Docker connection verified")

	printStep("Creating application directories...")
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

	// Create database directory under configDir
	dbDataDir := filepath.Join(configDir, "data")
	if err := os.MkdirAll(dbDataDir, 0o755); err != nil {
		return fmt.Errorf("failed to create database directory: %v", err)
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
	printSubstep("Downloads directory: " + dataDir)

	printStep("Writing docker configuration...")
	// Normalize Windows paths to use forward slashes for Docker
	normalizedDataDir := strings.ReplaceAll(dataDir, "\\", "/")

	// Replace the placeholder in docker-compose.yml template
	replaced := strings.ReplaceAll(dockerComposeTemplate, "{{DOWNLOAD_PATH}}", normalizedDataDir)

	composeFile := filepath.Join(configDir, "docker-compose.yml")

	if err := os.WriteFile(composeFile, []byte(replaced), 0o644); err != nil {
		return fmt.Errorf("failed to write docker-compose.yml: %v", err)
	}

	printSuccess("FluxStream setup completed!")
	fmt.Printf("\n%s %s\n", colorize(colorBlue, "Config file:"), composeFile)
	fmt.Printf("%s %s\n", colorize(colorBlue, "Downloads:  "), normalizedDataDir)
	fmt.Printf("\n%s\n", colorize(colorGreen, "Ready to start! Run: fluxstream start"))
	fmt.Printf("%s\n\n", colorize(colorCyan, "Docs:        https://docs.fluxstream.app"))

	return nil
}

// Status checks whether FluxStream's Docker containers and backend are running.
func Status() error {
	printHeader("FLUXSTREAM STATUS")

	// Check if Docker is installed
	printStep("Checking system services...")
	if !IsDockerInstalled() {
		printError("Docker is not installed.")
		fmt.Println("\nPlease install Docker before running FluxStream.")
		return fmt.Errorf("docker not installed")
	}

	// Check if Docker daemon is running
	if err := DockerInfo(); err != nil {
		printError("Docker daemon is not running.")
		fmt.Println("\nStart Docker and try again:")
		printSubstep("macOS/Windows: Open Docker Desktop")
		printSubstep("Linux: sudo systemctl start docker")
		return fmt.Errorf("docker daemon not running: %v", err)
	}
	printSubstep("Docker service is active")

	// Check if FluxStream containers are running
	printStep("Querying container status...")
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

	printSuccess("Active containers:")
	lines := strings.Split(output, "\n")
	for _, l := range lines {
		printSubstep(l)
	}

	// Check backend health endpoint
	printStep("Testing streaming API endpoint...")
	resp, err := http.Get("http://localhost:8080/health")
	if err != nil {
		printError("Streaming API not responding at http://localhost:8080/health")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		printSuccess("Backend API is online and fully healthy.")
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
	fmt.Printf("  %s %s\n", colorize(colorCyan, "Local:  "), colorize(colorGreen, localURL))
	fmt.Printf("  %s %s\n\n", colorize(colorCyan, "Network:"), colorize(colorGreen, lanURL))
	if strings.HasPrefix(lanIP, "172.") {
		fmt.Printf(" %s \n\n", colorize(colorYellow, "WARN: Network IP 172.* (Docker bridge) will not open on other devices."))
	}
}

func Where() error {
	printHeader("FLUXSTREAM ENDPOINTS")

	// 1. Check if Docker is installed
	if !IsDockerInstalled() {
		printError("Docker is not installed.")
		fmt.Println("\nPlease install Docker first:")
		printInfo("https://docs.docker.com/desktop/#next-steps")
		return fmt.Errorf("docker not installed")
	}

	// 2. Check if Docker daemon is running
	if err := DockerInfo(); err != nil {
		printError("Docker daemon is not running.")
		fmt.Println("Start Docker and try again:")
		printSubstep("macOS/Windows: Open Docker Desktop")
		printSubstep("Linux: sudo systemctl start docker")
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
		printError("FluxStream is not currently running.")
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
	printHeader("STOPPING FLUXSTREAM")

	// Check if Docker is installed
	if !IsDockerInstalled() {
		printError("Docker is not installed.")
		fmt.Println("\nPlease install Docker first:")
		printInfo("https://docs.docker.com/desktop/#next-steps")
		printInfo("https://docs.docker.com/engine/")
		return fmt.Errorf("docker not installed")
	}

	// Check if Docker daemon is running
	if err := DockerInfo(); err != nil {
		printError("Docker daemon is not running.")
		fmt.Println("\nPlease start Docker and try again:")
		printSubstep("macOS/Windows: Open Docker Desktop")
		printSubstep("Linux: sudo systemctl start docker")
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
	printStep("Stopping docker containers...")
	err = DockerCompose("down")

	if err != nil {
		printError(fmt.Sprintf("Failed to stop FluxStream: %v", err))
		return err
	}

	printSuccess("All FluxStream containers gracefully stopped.")

	return nil
}