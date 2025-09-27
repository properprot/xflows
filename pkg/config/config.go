package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	LogLevel string         `yaml:"log_level"`
	Outputs  []OutputConfig `yaml:"outputs"`
	SFlow    SFlowConfig    `yaml:"sflow"`
}

type OutputConfig struct {
	Type   string                 `yaml:"type"`
	Config map[string]interface{} `yaml:"config"`
}

type SFlowConfig struct {
	AgentIP string `yaml:"agent_ip"`
	// We don't grab the rate from XDP itself so it needs to be from config (for multipliers)
	SampleRate uint32 `yaml:"sample_rate"`
	// Incase the user wants to sep out as needed (if collector primarily)
	Version uint32 `yaml:"version"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}
