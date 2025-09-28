package config

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

var global_config *Config

type Config struct {
	LogLevel   string           `yaml:"log_level"`
	Outputs    []OutputConfig   `yaml:"outputs"`
	Processing ProcessingConfig `yaml:"processing"`
	SFlow      SFlowConfig      `yaml:"sflow"`

	LogrusLevel logrus.Level
}

type OutputConfig struct {
	Type   string                 `yaml:"type"`
	Config map[string]interface{} `yaml:"config"`
}

type ProcessingConfig struct {
	ChannelBuffer   int  `yaml:"channel_buffer"`
	ConcurrentSends bool `yaml:"concurrent_sends"`
}

type SFlowConfig struct {
	AgentIP string `yaml:"agent_ip"`
	// We don't grab the rate from XDP itself so it needs to be from config (for multipliers)
	SampleRate uint64 `yaml:"sample_rate"`
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

	if config.LogLevel != "debug" && config.LogLevel != "info" && config.LogLevel != "warning" && config.LogLevel != "error" {
		return nil, fmt.Errorf("invalid log level: %s", config.LogLevel)
	}

	switch config.LogLevel {
	case "debug":
		config.LogrusLevel = logrus.DebugLevel
	case "info":
		config.LogrusLevel = logrus.InfoLevel
	case "warning":
		config.LogrusLevel = logrus.WarnLevel
	case "error":
		config.LogrusLevel = logrus.ErrorLevel
	}

	global_config = &config

	return &config, nil
}

func GetConfig() *Config {
	return global_config
}

func GetLevel() logrus.Level {
	return global_config.LogrusLevel
}
