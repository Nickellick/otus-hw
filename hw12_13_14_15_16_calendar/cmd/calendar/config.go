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

type HTTPConf struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type StorageConf struct {
	Type string `yaml:"type"`
}

type PostgresConf struct {
	DSN          string `yaml:"dsn"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

type Config struct {
	Logger   LoggerConf   `yaml:"logger"`
	HTTP     HTTPConf     `yaml:"http"`
	Storage  StorageConf  `yaml:"storage"`
	Postgres PostgresConf `yaml:"postgres"`
}

func NewConfig() Config {
	return Config{
		Logger: LoggerConf{
			Level: "info",
		},
		HTTP: HTTPConf{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Storage: StorageConf{
			Type: "memory",
		},
		Postgres: PostgresConf{
			MaxOpenConns: 10,
			MaxIdleConns: 5,
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

	switch cfg.Storage.Type {
	case "memory":
	case "sql":
		if cfg.Postgres.DSN == "" {
			return nil, fmt.Errorf("postgres DSN is required")
		}
	default:
		return nil, fmt.Errorf("unsupported storage type: %q", cfg.Storage.Type)
	}

	if cfg.HTTP.Port <= 0 || cfg.HTTP.Port > 65535 {
		return nil, fmt.Errorf("invalid HTTP port: %d", cfg.HTTP.Port)
	}

	if cfg.Postgres.MaxOpenConns < 0 || cfg.Postgres.MaxIdleConns < 0 {
		return nil, fmt.Errorf("postgres connection limits cannot be negative")
	}

	if cfg.Storage.Type == "sql" &&
		cfg.Postgres.MaxOpenConns > 0 &&
		cfg.Postgres.MaxIdleConns > cfg.Postgres.MaxOpenConns {
		return nil, fmt.Errorf("max idle connections exceeds max open connections")
	}

	return &cfg, nil
}
