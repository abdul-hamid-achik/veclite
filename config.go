package veclite

import (
	"time"

	"github.com/abdul-hamid-achik/veclite/config"
)

type Config = config.Config

type EmbedderConfig = config.EmbedderConfig

type OpenAIConfig = config.OpenAIConfig

type OllamaConfig = config.OllamaConfig

type ONNXConfig = config.ONNXConfig

func DefaultConfig() *Config {
	return config.DefaultConfig()
}

func LoadConfig(path string) (*Config, error) {
	return config.LoadConfig(path)
}

func ExpandPath(path string) string {
	return config.ExpandPath(path)
}

func parseDuration(s string, defaultDur time.Duration) time.Duration {
	return config.ParseDuration(s, defaultDur)
}

func expandEnvVars(s string) string {
	return config.ExpandEnvVars(s)
}
