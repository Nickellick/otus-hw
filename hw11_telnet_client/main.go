package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"time"
)

const (
	ExitOk = iota
	ExitWrongArguments
	ExitConnectionError
	ExitSendError
	ExitReceiveError
)

func main() {
	code, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(code)
}

func run(args []string) (int, error) {
	flags := flag.NewFlagSet("go-telnet", flag.ContinueOnError)

	flags.SetOutput(io.Discard)

	timeout := flags.Duration(
		"timeout",
		10*time.Second,
		"connection timeout",
	)

	if err := flags.Parse(args); err != nil {
		return ExitWrongArguments, err
	}

	if flags.NArg() != 2 {
		return ExitWrongArguments, errors.New("usage: go-telnet [--timeout=10s] host port")
	}

	address := net.JoinHostPort(flags.Arg(0), flags.Arg(1))

	client := NewTelnetClient(
		address,
		*timeout,
		os.Stdin,
		os.Stdout,
	)

	if err := client.Connect(); err != nil {
		return ExitConnectionError, err
	}

	defer func() {
		err := client.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()

	fmt.Fprintf(os.Stderr, "...Connected to %s\n", address)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		os.Kill,
	)
	defer stop()

	sendDone := make(chan error, 1)
	receiveDone := make(chan error, 1)

	go func() {
		sendDone <- client.Send()
	}()

	go func() {
		receiveDone <- client.Receive()
	}()

	for {
		select {
		case err := <-sendDone:
			if err != nil {
				return ExitSendError, err
			}
			fmt.Fprintln(os.Stderr, "...EOF")
			return ExitOk, nil

		case err := <-receiveDone:
			if err != nil {
				return ExitReceiveError, err
			}
			fmt.Fprintln(os.Stderr, "...Connection was closed by peer")

			receiveDone = nil

		case <-ctx.Done():
			fmt.Fprintln(os.Stderr, "...Interrupted")
			return ExitOk, nil
		}
	}
}
