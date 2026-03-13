package server

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
)

const protocolVersion = 0

// conn represents a single client connection and its buffered reader.
type conn struct {
	rwc    net.Conn
	reader *bufio.Reader
	logger *slog.Logger
}

func newConn(rwc net.Conn, logger *slog.Logger) *conn {
	return &conn{
		rwc:    rwc,
		reader: bufio.NewReader(rwc),
		logger: logger,
	}
}

// handshake reads the client HELO, validates it, and sends the server response.
func (c *conn) handshake(cname, location string) error {
	helo, err := c.reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read HELO: %w", err)
	}
	helo = strings.TrimSpace(helo)

	if len(helo) != 5 || helo[:4] != "HELO" {
		fmt.Fprintf(c.rwc, "ERR:Invalid HELO received\n")
		return fmt.Errorf("invalid HELO: %q", helo)
	}

	version, err := strconv.ParseUint(helo[4:], 10, 16)
	if err != nil {
		fmt.Fprintf(c.rwc, "ERR:Invalid HELO received\n")
		return fmt.Errorf("parse version: %w", err)
	}

	if uint16(version) > protocolVersion {
		fmt.Fprintf(c.rwc, "ERR:Protocol version not supported\n")
		return fmt.Errorf("unsupported version: %d", version)
	}

	cn := cname
	if cn == "" {
		cn = "none"
	}
	loc := location
	if loc == "" {
		loc = "none"
	}

	_, err = fmt.Fprintf(c.rwc, "HELO\n%s\n%s\n", cn, loc)
	return err
}

// readCommand reads and validates the next protocol command (ECO, SND, or RCV).
func (c *conn) readCommand() (string, error) {
	cmd, err := c.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read command: %w", err)
	}
	cmd = strings.TrimSpace(cmd)

	if len(cmd) != 3 {
		fmt.Fprintf(c.rwc, "ERR:Invalid command received\n")
		return "", fmt.Errorf("invalid command: %q", cmd)
	}

	switch cmd {
	case "ECO", "SND", "RCV":
		return cmd, nil
	default:
		fmt.Fprintf(c.rwc, "ERR:Invalid command received\n")
		return "", fmt.Errorf("unknown command: %q", cmd)
	}
}

func isConnClosed(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "closed")
}
