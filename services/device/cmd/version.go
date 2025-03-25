package cmd

import (
	"fmt"
	"runtime"
	"time"

	"example.com/backstage/pkg/common"
	
	"github.com/spf13/cobra"
)

// BuildInfo contains information about the build
var BuildInfo struct {
	GitCommit string
	BuildTime string
	GoVersion string
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display version information",
	Long:  `Display the version, build information, and runtime environment of the device service.`,
	Run: func(cmd *cobra.Command, args []string) {
		displayVersion()
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	
	// Initialize build info if not set
	if BuildInfo.BuildTime == "" {
		BuildInfo.BuildTime = time.Now().Format(time.RFC3339)
	}
	if BuildInfo.GoVersion == "" {
		BuildInfo.GoVersion = runtime.Version()
	}
}

// displayVersion shows detailed version information
func displayVersion() {
	fmt.Println("Device Service")
	fmt.Println("==============")
	fmt.Printf("Version:    %s\n", common.Version)
	fmt.Printf("Git Commit: %s\n", BuildInfo.GitCommit)
	fmt.Printf("Built:      %s\n", BuildInfo.BuildTime)
	fmt.Printf("Go Version: %s\n", BuildInfo.GoVersion)
	fmt.Printf("OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
}