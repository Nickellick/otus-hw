package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/app"
	"github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/logger"
	internalhttp "github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/server/http"
	memorystorage "github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage/memory"
	sqlstorage "github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/storage/sql"
)

var configFile string

func init() {
	flag.StringVar(
		&configFile,
		"config",
		"./configs/config.yaml",
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

	if flag.Arg(0) == "version" {
		printVersion()
		return nil
	}

	config, err := LoadConfig(configFile)
	if err != nil {
		return err
	}

	logg := logger.New(config.Logger.Level)

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()

	storage, cleanupStorage, err := createStorage(ctx, config)
	if err != nil {
		return err
	}

	defer cleanupStorage()

	calendar := app.New(logg, storage)

	server := internalhttp.NewServer(
		logg,
		calendar,
		config.HTTP.Host,
		config.HTTP.Port,
	)

	shutdownDone := make(chan error, 1)

	go func() {
		defer close(shutdownDone)

		<-ctx.Done()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second*3)
		defer shutdownCancel()

		shutdownDone <- server.Stop(shutdownCtx)
	}()

	logg.Info("calendar is running...")

	err = server.Start(ctx)

	cancel()
	<-shutdownDone

	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logg.Error("failed to start http server: " + err.Error())
		return err
	}

	return nil
}

func createStorage(
	ctx context.Context,
	config *Config,
) (app.Storage, func(), error) {
	switch config.Storage.Type {
	case "memory":
		return memorystorage.New(), func() {}, nil

	case "sql":
		db, err := sqlstorage.Connect(ctx, config.Postgres.DSN)
		if err != nil {
			return nil, nil, fmt.Errorf("connect to PostgreSQL: %w", err)
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
