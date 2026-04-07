package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	// 云效服务接入点域名
	Endpoint string `yaml:"endpoint"`
	// 个人访问令牌 (推荐的认证方式)
	PersonalAccessToken string `yaml:"personal_access_token"`
	// 企业 ID（组织 ID）
	OrganizationID string `yaml:"organization_id"`
	// AccessKey 认证方式 (备用方式)
	AccessKeyID     string `yaml:"access_key_id"`
	AccessKeySecret string `yaml:"access_key_secret"`
	RegionID        string `yaml:"region_id"`
	// 编辑器和分页器配置
	Editor string `yaml:"editor,omitempty"`
	Pager  string `yaml:"pager,omitempty"`
	// 每页显示的条目数，默认 30
	PerPage int `yaml:"per_page,omitempty"`
	// 书签配置
	Bookmarks []string `yaml:"bookmarks,omitempty"`
	// 默认排序方式: last_run_time, name, create_time, update_time, bookmark
	DefaultSort string `yaml:"default_sort,omitempty"`
	// 通知命令 - 可选，流水线结束时执行
	// 支持 text/template 语法，可用占位符: .PipelineName, .Result, .Duration, .Branch
	NotifyCommand string `yaml:"notify_command,omitempty"`
}

// GetPerPage returns the number of items per page, defaulting to 30
func (c *Config) GetPerPage() int {
	if c.PerPage <= 0 {
		return 30
	}
	return c.PerPage
}

// LoadConfig loads configuration from ~/.flo/config.yml
func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".flo", "config.yml")

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found at %s", configPath)
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}

// LoadConfigFrom loads configuration from a specific file path.
func LoadConfigFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// SaveConfig saves configuration to ~/.flo/config.yml
func SaveConfig(config *Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".flo", "config.yml")

	// Ensure config directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal config to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config to YAML: %w", err)
	}

	// Write config file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.OrganizationID == "" {
		return fmt.Errorf("organization_id is required in configuration")
	}

	// 检查认证方式：优先使用个人访问令牌，其次使用AccessKey
	hasPersonalToken := c.PersonalAccessToken != ""
	hasAccessKey := c.AccessKeyID != "" && c.AccessKeySecret != ""

	if !hasPersonalToken && !hasAccessKey {
		return fmt.Errorf("either personal_access_token or both access_key_id and access_key_secret are required")
	}

	return nil
}

// GetEditor returns the editor command to use, following the priority:
// 1. Config file "editor" field
// 2. VISUAL environment variable
// 3. EDITOR environment variable
// 4. Default to "vim"
func (c *Config) GetEditor() string {
	// First check config file
	if c.Editor != "" {
		return c.Editor
	}

	// Then check VISUAL environment variable
	if visual := os.Getenv("VISUAL"); visual != "" {
		return visual
	}

	// Then check EDITOR environment variable
	if editor := os.Getenv("EDITOR"); editor != "" {
		return editor
	}

	// Default to vim
	return "vim"
}

// GetPager returns the pager command to use, following the priority:
// 1. Config file "pager" field
// 2. PAGER environment variable
// 3. Default to "less"
func (c *Config) GetPager() string {
	// First check config file
	if c.Pager != "" {
		return c.Pager
	}

	// Then check PAGER environment variable
	if pager := os.Getenv("PAGER"); pager != "" {
		return pager
	}

	// Default to less
	return "less"
}

// AddBookmark adds a pipeline name to bookmarks if not already present
func (c *Config) AddBookmark(pipelineName string) bool {
	for _, bookmark := range c.Bookmarks {
		if bookmark == pipelineName {
			return false // Already bookmarked
		}
	}
	c.Bookmarks = append(c.Bookmarks, pipelineName)
	return true // Added
}

// RemoveBookmark removes a pipeline name from bookmarks
func (c *Config) RemoveBookmark(pipelineName string) bool {
	for i, bookmark := range c.Bookmarks {
		if bookmark == pipelineName {
			c.Bookmarks = append(c.Bookmarks[:i], c.Bookmarks[i+1:]...)
			return true // Removed
		}
	}
	return false // Not found
}

// ToggleBookmark toggles a pipeline name in bookmarks
// Returns true if the bookmark was added, false if it was removed
func (c *Config) ToggleBookmark(pipelineName string) bool {
	if c.RemoveBookmark(pipelineName) {
		return false // Removed
	}
	c.AddBookmark(pipelineName)
	return true // Added
}

// IsBookmarked checks if a pipeline name is bookmarked
func (c *Config) IsBookmarked(pipelineName string) bool {
	for _, bookmark := range c.Bookmarks {
		if bookmark == pipelineName {
			return true
		}
	}
	return false
}

// GetEndpoint returns the API endpoint, defaulting to the standard endpoint
func (c *Config) GetEndpoint() string {
	if c.Endpoint != "" {
		return c.Endpoint
	}
	return "openapi-rdc.aliyuncs.com"
}

// GetRegionID returns the region ID, defaulting to cn-hangzhou
func (c *Config) GetRegionID() string {
	if c.RegionID != "" {
		return c.RegionID
	}
	return "cn-hangzhou"
}

// UsePersonalAccessToken returns whether to use personal access token authentication
func (c *Config) UsePersonalAccessToken() bool {
	return c.PersonalAccessToken != ""
}

// GetDefaultSort returns the default sort configuration value
func (c *Config) GetDefaultSort() string {
	if c.DefaultSort == "" {
		return "name"
	}
	return c.DefaultSort
}

