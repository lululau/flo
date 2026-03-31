package main

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"flo/internal/api"
	"flo/internal/config"
	"flo/internal/tui"
)

// RunTUIFunc creates an API client and launches the TUI.
var RunTUIFunc = func(cfg *config.Config) error {
	var client *api.Client
	var err error
	if cfg.UsePersonalAccessToken() {
		client, err = api.NewClientWithToken(cfg.GetEndpoint(), cfg.PersonalAccessToken)
	} else {
		client, err = api.NewClient(cfg.AccessKeyID, cfg.AccessKeySecret, cfg.GetRegionID())
	}
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	model := tui.New(cfg, client)
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
