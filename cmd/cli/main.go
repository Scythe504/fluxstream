package main

import (
	"os"

	"github.com/scythe504/fluxstream/internal/commands"
	"github.com/spf13/cobra"
)

var (
	version = "0.2.2"
)

var rootCmd = &cobra.Command{
	Use:     "fluxstream",
	Short:   "fluxstream - Torrent media streamer",
	Long:    `fluxstream-cli is a tool for running the fluxstream server and web on your desktop`,
	Version: version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		commands.PrintLogo()
	},
}
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the Fluxstream server",
	Long:  `Start the Fluxstream server using docker-compose with neat logs and network info`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.Start()
	},
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Sets up required configs and docker engine (if not installed)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.Setup()
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Shows the status of the server whether running or not",
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.Status()
	},
}

var whereCmd = &cobra.Command{
	Use:   "where",
	Short: "Prints the url for the web app",
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.Where()
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stops the Fluxstream server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.Stop()
	},
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Updates the Fluxstream CLI binary to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.Update()
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstalls Fluxstream containers, volumes, and configuration directories",
	RunE: func(cmd *cobra.Command, args []string) error {
		return commands.Uninstall()
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(whereCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(uninstallCmd)

	rootCmd.SetHelpTemplate(commands.GetLogoString() + "\n" + rootCmd.HelpTemplate())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
