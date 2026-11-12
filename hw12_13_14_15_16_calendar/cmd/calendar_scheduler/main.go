package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/queue/rabbitmq"
	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/scheduler"
	memorystorage "github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage/memory"
	sqlstorage "github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage/sql"
)

var configFile string

func init() {
	flag.StringVar(
		&configFile,
		"config",
		"./configs/scheduler.yaml",
		"Path to configuration file",
	)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()

	config, err := LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger := newLogger(config.Logger.Level)

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGHUP,
	)
	defer cancel()

	store, cleanupStorage, err := createStorage(ctx, config)
	if err != nil {
		return err
	}
	defer cleanupStorage()

	publisher, err := rabbitmq.New(config.RabbitMQ.URL)
	if err != nil {
		return fmt.Errorf("connect to RabbitMQ: %w", err)
	}
	defer publisher.Close()

	service, err := scheduler.New(
		store,
		publisher,
		logger,
		config.Scheduler.Interval.Duration,
		config.Scheduler.BatchSize,
	)
	if err != nil {
		return fmt.Errorf("create scheduler: %w", err)
	}

	logger.Info(
		"calendar scheduler started",
		"storage", config.Storage.Type,
		"interval", config.Scheduler.Interval.Duration,
		"batch_size", config.Scheduler.BatchSize,
	)

	if err := service.Run(ctx); err != nil {
		return fmt.Errorf("run scheduler: %w", err)
	}

	logger.Info("calendar scheduler stopped")

	return nil
}

func createStorage(
	ctx context.Context,
	config *Config,
) (scheduler.Storage, func(), error) {
	switch config.Storage.Type {
	case "memory":
		return memorystorage.New(), func() {}, nil

	case "sql":
		db, err := sqlstorage.Connect(ctx, config.Postgres.DSN)
		if err != nil {
			return nil, nil, fmt.Errorf(
				"connect to PostgreSQL: %w",
				err,
			)
		}

		db.SetMaxOpenConns(config.Postgres.MaxOpenConns)
		db.SetMaxIdleConns(config.Postgres.MaxIdleConns)

		cleanup := func() {
			_ = db.Close()
		}

		return sqlstorage.New(db), cleanup, nil

	default:
		return nil, nil, fmt.Errorf(
			"unsupported storage type: %s",
			config.Storage.Type,
		)
	}
}

func newLogger(level string) *slog.Logger {
	var logLevel slog.Level

	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn", "warning":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	return slog.New(slog.NewTextHandler(
		os.Stdout,
		&slog.HandlerOptions{Level: logLevel},
	))
}
