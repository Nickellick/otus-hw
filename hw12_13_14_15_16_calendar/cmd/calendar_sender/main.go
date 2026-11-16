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
	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/sender"
	sqlstorage "github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage/sql"
)

var configFile string

func init() {
	flag.StringVar(
		&configFile,
		"config",
		"./configs/sender.yaml",
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

	consumer, err := rabbitmq.New(config.RabbitMQ.URL)
	if err != nil {
		return fmt.Errorf("connect to RabbitMQ: %w", err)
	}
	defer consumer.Close()

	db, err := sqlstorage.Connect(ctx, config.Postgres.DSN)
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(config.Postgres.MaxOpenConns)
	db.SetMaxIdleConns(config.Postgres.MaxIdleConns)

	store := sqlstorage.New(db)

	service := sender.New(consumer, store, logger)

	logger.Info("calendar sender started")

	if err := service.Run(ctx); err != nil {
		return fmt.Errorf("run sender: %w", err)
	}

	logger.Info("calendar sender stopped")

	return nil
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
