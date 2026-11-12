package main

import (
	"bytes"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var value string

	if err := node.Decode(&value); err != nil {
		return err
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", value, err)
	}

	d.Duration = duration
	return nil
}

type LoggerConf struct {
	Level string `yaml:"level"`
}

type StorageConf struct {
	Type string `yaml:"type"`
}

type PostgresConf struct {
	DSN          string `yaml:"dsn"`
	MaxOpenConns int    `yaml:"maxOpenConns"`
	MaxIdleConns int    `yaml:"maxIdleConns"`
}

type RabbitMQConf struct {
	URL string `yaml:"url"`
}

type SchedulerConf struct {
	Interval  Duration `yaml:"interval"`
	BatchSize int      `yaml:"batchSize"`
}

type Config struct {
	Logger    LoggerConf    `yaml:"logger"`
	Storage   StorageConf   `yaml:"storage"`
	Postgres  PostgresConf  `yaml:"postgres"`
	RabbitMQ  RabbitMQConf  `yaml:"rabbitmq"`
	Scheduler SchedulerConf `yaml:"scheduler"`
}

func NewConfig() Config {
	return Config{
		Logger:  LoggerConf{Level: "info"},
		Storage: StorageConf{Type: "sql"},
		Postgres: PostgresConf{
			MaxOpenConns: 10,
			MaxIdleConns: 5,
		},
		Scheduler: SchedulerConf{
			Interval:  Duration{5 * time.Second},
			BatchSize: 100,
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
		return nil, fmt.Errorf(
			"unsupported storage type: %q",
			cfg.Storage.Type,
		)
	}

	if cfg.RabbitMQ.URL == "" {
		return nil, fmt.Errorf("RabbitMQ URL is required")
	}

	if cfg.Scheduler.Interval.Duration <= 0 {
		return nil, fmt.Errorf("scheduler interval must be positive")
	}

	if cfg.Scheduler.BatchSize <= 0 {
		return nil, fmt.Errorf("scheduler batch size must be positive")
	}

	if cfg.Postgres.MaxOpenConns < 0 ||
		cfg.Postgres.MaxIdleConns < 0 {
		return nil, fmt.Errorf(
			"postgres connection limits cannot be negative",
		)
	}

	if cfg.Storage.Type == "sql" &&
		cfg.Postgres.MaxOpenConns > 0 &&
		cfg.Postgres.MaxIdleConns > cfg.Postgres.MaxOpenConns {
		return nil, fmt.Errorf(
			"max idle connections exceeds max open connections",
		)
	}

	return &cfg, nil
}
