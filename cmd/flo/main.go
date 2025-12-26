package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"flo/internal/api"
	"flo/internal/config"
	"flo/internal/tui"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nPlease create a configuration file at ~/.flo/config.yml with the following format:\n")
		fmt.Fprintf(os.Stderr, `
organization_id: "your_organization_id"
personal_access_token: "your_personal_access_token"  # Recommended

# Or use AccessKey authentication:
# access_key_id: "your_access_key_id"
# access_key_secret: "your_access_key_secret"

# Optional settings:
# editor: "vim"
# pager: "less"
`)
		os.Exit(1)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Create API client
	var client *api.Client
	var clientErr error
	if cfg.UsePersonalAccessToken() {
		client, clientErr = api.NewClientWithToken(cfg.GetEndpoint(), cfg.PersonalAccessToken)
	} else {
		client, clientErr = api.NewClient(cfg.AccessKeyID, cfg.AccessKeySecret, cfg.GetRegionID())
	}
	if clientErr != nil {
		fmt.Fprintf(os.Stderr, "Failed to create API client: %v\n", clientErr)
		os.Exit(1)
	}

	// Create and run the TUI application
	model := tui.New(cfg, client)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}

