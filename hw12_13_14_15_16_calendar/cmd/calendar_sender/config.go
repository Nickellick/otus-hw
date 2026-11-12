package main

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type LoggerConf struct {
	Level string `yaml:"level"`
}

type RabbitMQConf struct {
	URL string `yaml:"url"`
}

type Config struct {
	Logger   LoggerConf   `yaml:"logger"`
	RabbitMQ RabbitMQConf `yaml:"rabbitmq"`
}

func NewConfig() Config {
	return Config{
		Logger: LoggerConf{
			Level: "info",
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := NewConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.RabbitMQ.URL == "" {
		return nil, fmt.Errorf("RabbitMQ URL is required")
	}

	return &cfg, nil
}
