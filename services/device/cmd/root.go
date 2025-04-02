package cmd

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"example.com/backstage/services/device/config"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const (
	// Version represents the current service version
	Version = "1.0.0"
	// BuildTimeStamp is set during build process
	BuildTimeStamp = "undefined"
	// GitCommit is set during build process
	GitCommit = "undefined"
)

var (
	// Used for flags
	cfgFile     string
	logLevel    string
	logFormat   string
	showVersion bool
	jsonOutput  bool

	// Logger instance for all commands
	log = logrus.New()
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "device-service",
	Short: "Device management service",
	Long: `Device service for managing IoT devices, firmware updates,
and message processing for the backstage platform.

This service provides APIs for:
- Device registration and management
- Firmware upload and distribution
- OTA updates
- Device telemetry processing`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Handle version flag
		if showVersion {
			displayVersion()
			os.Exit(0)
		}

		// Set up logging based on command line flags
		setupLogging()

		// Initialize configuration
		if err := config.InitConfig(cfgFile); err != nil {
			log.Fatalf("Error initializing configuration: %v", err)
		}

		// Log startup information
		logStartupInfo()
	},
	Run: func(cmd *cobra.Command, args []string) {
		// If no subcommand is provided, show help
		cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Config file flag
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./config.yaml)")

	// Logging flags
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "json", "log format (json, text)")

	// Version flag with short form option
	rootCmd.PersistentFlags().BoolVarP(&showVersion, "version", "v", false, "display version information")

	// JSON output for scripting
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output version as JSON (only with --version)")
}

// setupLogging configures the global logger based on command line flags
func setupLogging() {
	// Set log level
	level, err := logrus.ParseLevel(strings.ToLower(logLevel))
	if err != nil {
		fmt.Printf("Invalid log level '%s'. Defaulting to 'info'\n", logLevel)
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	// Set log format
	if logFormat == "json" {
		log.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339Nano,
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	} else {
		log.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05.000",
			DisableColors:   os.Getenv("NO_COLOR") != "",
		})
	}

	// Output to stderr
	log.SetOutput(os.Stderr)

	// Add default fields to all log entries
	log.WithFields(logrus.Fields{
		"service": "device-service",
		"version": Version,
	})
}

// logStartupInfo logs basic information about the service at startup
func logStartupInfo() {
	hostname, _ := os.Hostname()
	log.WithFields(logrus.Fields{
		"version":     Version,
		"git_commit":  GitCommit,
		"go_version":  runtime.Version(),
		"os_arch":     fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
		"hostname":    hostname,
		"num_cpu":     runtime.NumCPU(),
		"max_procs":   runtime.GOMAXPROCS(0),
		"config_file": cfgFile,
		"log_level":   log.GetLevel().String(),
		"log_format":  logFormat,
	}).Info("Device service starting")
}

// displayVersion prints version information
func displayVersion() {
	if jsonOutput {
		jsonVersion := fmt.Sprintf(`{
  "service": "device-service",
  "version": "%s",
  "build_timestamp": "%s",
  "git_commit": "%s",
  "go_version": "%s",
  "os": "%s",
  "arch": "%s"
}`, Version, BuildTimeStamp, GitCommit, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		fmt.Println(jsonVersion)
		return
	}

	fmt.Printf("Device Service v%s\n", Version)
	if GitCommit != "undefined" {
		fmt.Printf("Git commit: %s\n", GitCommit)
	}
	if BuildTimeStamp != "undefined" {
		fmt.Printf("Build time: %s\n", BuildTimeStamp)
	}
	fmt.Printf("Go version: %s\n", runtime.Version())
	fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}
