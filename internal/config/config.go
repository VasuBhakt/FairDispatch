package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Weights struct {
	Fairness  float64 `yaml:"fairness"`
	Proximity float64 `yaml:"proximity"`
}

type DomainConfig struct {
	Domain         string  `yaml:"domain"`
	ResourceType   string  `yaml:"resource_type"`
	HoldTTLSeconds int     `yaml:"hold_ttl_seconds"`
	Weights        Weights `yaml:"weights"`
}

// LoadConfig reads the YAML config file for a specific domain.
func LoadConfig(path string) (*DomainConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg DomainConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
