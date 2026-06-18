// @tag-block architecture
// @tag layer: "config"
// @tag domain: "tagdoc-core"
// @tag depends-on: []
// @tag description: "Loads .marionetterc.json configuration with rules, templates, constraints, and repository definitions"
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Version      string              `json:"version"`
	Index        IndexConfig         `json:"index,omitempty"`
	Repositories []RepoConfig        `json:"repositories,omitempty"`
	Rules        []RuleConfig        `json:"rules,omitempty"`
	Constraints  []ConstraintConfig  `json:"constraints,omitempty"`
	Output       OutputConfig        `json:"output,omitempty"`
	Templates    []TemplateConfig    `json:"templates,omitempty"`
}

type IndexConfig struct {
	Paths          []string `json:"paths,omitempty"`
	Exclude        []string `json:"exclude,omitempty"`
	FollowSymlinks bool     `json:"follow_symlinks,omitempty"`
}

type RepoConfig struct {
	Path     string `json:"path"`
	Label    string `json:"label,omitempty"`
	Language string `json:"language,omitempty"`
}

// ConstraintConfig defines a code/tag/arch constraint rule.
type ConstraintConfig struct {
	Type        string `json:"type"`        // "tag", "code", "arch"
	Name        string `json:"name"`
	Rule        string `json:"rule"`
	Severity    string `json:"severity,omitempty"` // "error", "warning", "info"
	Description string `json:"description,omitempty"`
}

type RuleConfig struct {
	TagType        string   `json:"tag_type,omitempty"`
	Kind           string   `json:"kind,omitempty"`
	Key            string   `json:"key"`
	Required       bool     `json:"required,omitempty"`
	AllowedValues  []string `json:"allowed_values,omitempty"`
	AllowedPattern string   `json:"allowed_pattern,omitempty"`
	Type           string   `json:"type,omitempty"`
}

type TemplateConfig struct {
	Name   string            `json:"name"`
	Tags   map[string]string `json:"tags"`
}

type OutputConfig struct {
	Format string `json:"format,omitempty"`
	Color  bool   `json:"color,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		Version: "2.0",
		Index: IndexConfig{
			Exclude: []string{"node_modules/**", "__pycache__/**", ".git/**", "target/**", "vendor/**", ".svn/**"},
		},
		Output: OutputConfig{Format: "json"},
	}
}

func LoadConfig(dir string) (*Config, error) {
	currentDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	for {
		configPath := filepath.Join(currentDir, ".marionetterc.json")
		if _, err := os.Stat(configPath); err == nil {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read config: %w", err)
			}
			var cfg Config
			if err := json.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config: %w", err)
			}
			return &cfg, nil
		}
		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			break
		}
		currentDir = parent
	}
	return DefaultConfig(), nil
}

func SaveConfig(cfg *Config, dir string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(filepath.Join(dir, ".marionetterc.json"), data, 0644)
}
