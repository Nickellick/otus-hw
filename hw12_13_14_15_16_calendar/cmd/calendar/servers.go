package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	internalgrpc "github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/server/grpc"
	internalhttp "github.com/Nickellick/otus-hw/hw12_13_14_15_16_calendar/internal/server/http"
)

type Logger interface {
	Info(msg string, args ...any)
}

func runServers(
	ctx context.Context,
	logger Logger,
	httpServer *internalhttp.Server,
	grpcServer *internalgrpc.Server,
	cfg Config,
) error {
	httpAddr := net.JoinHostPort(
		cfg.HTTP.Host,
		strconv.Itoa(cfg.HTTP.Port),
	)

	grpcAddr := net.JoinHostPort(
		cfg.GRPC.Host,
		strconv.Itoa(cfg.GRPC.Port),
	)

	httpListener, err := net.Listen("tcp", httpAddr)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}
	defer httpListener.Close()

	grpcListener, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return fmt.Errorf("listen gRPC: %w", err)
	}
	defer grpcListener.Close()

	type serverResult struct {
		name string
		err  error
	}

	results := make(chan serverResult, 2)

	go func() {
		results <- serverResult{
			name: "HTTP",
			err:  httpServer.Serve(httpListener),
		}
	}()

	go func() {
		results <- serverResult{
			name: "gRPC",
			err:  grpcServer.Serve(grpcListener),
		}
	}()

	logger.Info("calendar started",
		"http", cfg.HTTP.Host+":"+strconv.Itoa(cfg.HTTP.Port),
		"grpc", cfg.GRPC.Host+":"+strconv.Itoa(cfg.GRPC.Port),
	)

	var runErr error

	select {
	case result := <-results:
		if result.err != nil {
			runErr = fmt.Errorf("%s server: %w", result.name, result.err)
		} else {
			runErr = fmt.Errorf("%s server stopped unexpectedly", result.name)
		}

	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		3*time.Second,
	)
	defer cancel()

	httpDone := make(chan error, 1)
	grpcDone := make(chan error, 1)

	go func() {
		httpDone <- httpServer.Stop(shutdownCtx)
	}()

	go func() {
		grpcDone <- grpcServer.Stop(shutdownCtx)
	}()

	httpErr := <-httpDone
	grpcErr := <-grpcDone

	if httpErr != nil || grpcErr != nil {
		return errors.Join(runErr, httpErr, grpcErr)
	}

	return runErr
}
