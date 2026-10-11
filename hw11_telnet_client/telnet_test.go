package main

import (
	"bytes"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var errTestRead = errors.New("test read failure")

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errTestRead
}

func (failingReader) Close() error {
	return nil
}

func TestTelnetClient(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		l, err := net.Listen("tcp", "127.0.0.1:")
		require.NoError(t, err)
		defer func() { require.NoError(t, l.Close()) }()

		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()

			in := &bytes.Buffer{}
			out := &bytes.Buffer{}

			timeout, err := time.ParseDuration("10s")
			require.NoError(t, err)

			client := NewTelnetClient(l.Addr().String(), timeout, io.NopCloser(in), out)
			require.NoError(t, client.Connect())
			defer func() { require.NoError(t, client.Close()) }()

			in.WriteString("hello\n")
			err = client.Send()
			require.NoError(t, err)

			err = client.Receive()
			require.NoError(t, err)
			require.Equal(t, "world\n", out.String())
		}()

		go func() {
			defer wg.Done()

			conn, err := l.Accept()
			require.NoError(t, err)
			require.NotNil(t, conn)
			defer func() { require.NoError(t, conn.Close()) }()

			request := make([]byte, 1024)
			n, err := conn.Read(request)
			require.NoError(t, err)
			require.Equal(t, "hello\n", string(request)[:n])

			n, err = conn.Write([]byte("world\n"))
			require.NoError(t, err)
			require.NotEqual(t, 0, n)
		}()

		wg.Wait()
	})
}

func TestTelnetClient_ConnectionRefused(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	address := listener.Addr().String()
	require.NoError(t, listener.Close())

	client := NewTelnetClient(
		address,
		time.Second,
		io.NopCloser(bytes.NewBuffer(nil)),
		&bytes.Buffer{},
	)

	err = client.Connect()
	require.Error(t, err)
}

func TestTelnetClient_LargePayload(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	payload := bytes.Repeat([]byte("hello world\n"), 10_000)

	received := make(chan []byte, 1)
	serverErr := make(chan error, 1)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()

		data := make([]byte, len(payload))
		_, err = io.ReadFull(conn, data)
		if err != nil {
			serverErr <- err
			return
		}

		received <- data
	}()

	client := NewTelnetClient(
		listener.Addr().String(),
		time.Second,
		io.NopCloser(bytes.NewReader(payload)),
		io.Discard,
	)

	require.NoError(t, client.Connect())
	defer client.Close()

	require.NoError(t, client.Send())

	select {
	case data := <-received:
		require.Equal(t, payload, data)
	case err := <-serverErr:
		t.Fatalf("server error: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for payload")
	}
}

func TestTelnetClient_SendReadError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	serverDone := make(chan struct{})

	go func() {
		defer close(serverDone)

		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		_, _ = io.Copy(io.Discard, conn)
	}()

	client := NewTelnetClient(
		listener.Addr().String(),
		time.Second,
		failingReader{},
		io.Discard,
	)

	require.NoError(t, client.Connect())
	defer client.Close()

	err = client.Send()
	require.ErrorIs(t, err, errTestRead)
}

func TestTelnetClient_ServerDisconnect(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		_, _ = conn.Write([]byte("goodbye\n"))
		_ = conn.Close()
	}()

	var output bytes.Buffer

	client := NewTelnetClient(
		listener.Addr().String(),
		time.Second,
		io.NopCloser(bytes.NewReader(nil)),
		&output,
	)

	require.NoError(t, client.Connect())
	defer client.Close()

	require.NoError(t, client.Receive())
	require.Equal(t, "goodbye\n", output.String())
}
