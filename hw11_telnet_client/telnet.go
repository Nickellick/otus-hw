package main

import (
	"fmt"
	"io"
	"net"
	"time"
)

type TelnetClient interface {
	Connect() error
	io.Closer
	Send() error
	Receive() error
}

func NewTelnetClient(address string, timeout time.Duration, in io.ReadCloser, out io.Writer) TelnetClient {
	return &telnetClient{
		address: address,
		timeout: timeout,
		in:      in,
		out:     out,
	}
}

type telnetClient struct {
	address string
	timeout time.Duration
	in      io.ReadCloser
	out     io.Writer
	conn    net.Conn
}

func (c *telnetClient) Connect() error {
	conn, err := net.DialTimeout("tcp", c.address, c.timeout)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", c.address, err)
	}

	c.conn = conn
	return nil
}

func (c *telnetClient) Send() error {
	_, err := io.Copy(c.conn, c.in)
	if err != nil {
		return fmt.Errorf("send error: %w", err)
	}
	return nil
}

func (c *telnetClient) Receive() error {
	_, err := io.Copy(c.out, c.conn)
	if err != nil {
		return fmt.Errorf("receive error: %w", err)
	}
	return nil
}

func (c *telnetClient) Close() error {
	if c.conn == nil {
		return nil
	}

	return c.conn.Close()
}
