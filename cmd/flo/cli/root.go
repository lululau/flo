package cli

import (
	"github.com/spf13/cobra"

	"flo/internal/config"
)

// RunTUI is set by main to allow the root command to launch the TUI.
var RunTUI func(cfg *config.Config) error

var rootCmd = &cobra.Command{
	Use:   "flo",
	Short: "Aliyun DevOps pipeline manager",
	Long:  "Flo is a TUI and CLI tool for managing Aliyun DevOps (云效) pipelines.",
}

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive TUI",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return err
		}
		if err := cfg.Validate(); err != nil {
			return err
		}
		return RunTUI(cfg)
	},
}

func loadConfig() (*config.Config, error) {
	if configPath != "" {
		return config.LoadConfigFrom(configPath)
	}
	return config.LoadConfig()
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table, json")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Config file path (default: ~/.flo/config.yml)")
	rootCmd.PersistentFlags().StringVar(&orgID, "org", "", "Organization ID (overrides config)")

	rootCmd.AddCommand(pipelineCmd)
	rootCmd.AddCommand(tuiCmd)
}

var (
	outputFormat string
	configPath   string
	orgID        string
)
