package server

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// pipeConn creates a net.Pipe and returns a conn wrapping the server end
// plus the raw client end for writing/reading test data.
func pipeConn() (*conn, net.Conn) {
	client, server := net.Pipe()
	c := &conn{
		rwc:    server,
		reader: bufio.NewReader(server),
		logger: testLogger(),
	}
	return c, client
}

func TestHandshake_Valid(t *testing.T) {
	c, client := pipeConn()
	defer c.rwc.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.handshake("test.example.com", "Seattle, WA")
	}()

	// Client sends HELO
	if _, err := client.Write([]byte("HELO0\r\n")); err != nil {
		t.Fatal(err)
	}

	// Read server response
	reader := bufio.NewReader(client)
	line1, _ := reader.ReadString('\n')
	line2, _ := reader.ReadString('\n')
	line3, _ := reader.ReadString('\n')

	if strings.TrimSpace(line1) != "HELO" {
		t.Errorf("expected HELO, got %q", line1)
	}
	if strings.TrimSpace(line2) != "test.example.com" {
		t.Errorf("expected cname, got %q", line2)
	}
	if strings.TrimSpace(line3) != "Seattle, WA" {
		t.Errorf("expected location, got %q", line3)
	}

	if err := <-errCh; err != nil {
		t.Errorf("handshake returned error: %v", err)
	}
}

func TestHandshake_EmptyCnameLocation(t *testing.T) {
	c, client := pipeConn()
	defer c.rwc.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.handshake("", "")
	}()

	client.Write([]byte("HELO0\r\n"))

	reader := bufio.NewReader(client)
	reader.ReadString('\n') // HELO
	line2, _ := reader.ReadString('\n')
	line3, _ := reader.ReadString('\n')

	if strings.TrimSpace(line2) != "none" {
		t.Errorf("expected 'none' for empty cname, got %q", line2)
	}
	if strings.TrimSpace(line3) != "none" {
		t.Errorf("expected 'none' for empty location, got %q", line3)
	}

	if err := <-errCh; err != nil {
		t.Errorf("handshake returned error: %v", err)
	}
}

func TestHandshake_InvalidLength(t *testing.T) {
	c, client := pipeConn()
	defer c.rwc.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.handshake("host", "loc")
	}()

	client.Write([]byte("HEL\r\n"))

	reader := bufio.NewReader(client)
	resp, _ := reader.ReadString('\n')

	if !strings.Contains(resp, "ERR:Invalid HELO received") {
		t.Errorf("expected ERR response, got %q", resp)
	}

	if err := <-errCh; err == nil {
		t.Error("expected error for invalid HELO")
	}
}

func TestHandshake_InvalidPrefix(t *testing.T) {
	c, client := pipeConn()
	defer c.rwc.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.handshake("host", "loc")
	}()

	client.Write([]byte("XELO0\r\n"))

	reader := bufio.NewReader(client)
	resp, _ := reader.ReadString('\n')

	if !strings.Contains(resp, "ERR:Invalid HELO received") {
		t.Errorf("expected ERR response, got %q", resp)
	}

	if err := <-errCh; err == nil {
		t.Error("expected error for invalid prefix")
	}
}

func TestHandshake_UnsupportedVersion(t *testing.T) {
	c, client := pipeConn()
	defer c.rwc.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.handshake("host", "loc")
	}()

	client.Write([]byte("HELO9\r\n"))

	reader := bufio.NewReader(client)
	resp, _ := reader.ReadString('\n')

	if !strings.Contains(resp, "ERR:Protocol version not supported") {
		t.Errorf("expected version ERR, got %q", resp)
	}

	if err := <-errCh; err == nil {
		t.Error("expected error for unsupported version")
	}
}

func TestHandshake_ClientHangup(t *testing.T) {
	c, client := pipeConn()
	defer c.rwc.Close()

	errCh := make(chan error, 1)
	go func() {
		errCh <- c.handshake("host", "loc")
	}()

	// Close client immediately — simulates hangup
	client.Close()

	if err := <-errCh; err == nil {
		t.Error("expected error on client hangup")
	}
}

func TestReadCommand_Valid(t *testing.T) {
	for _, cmd := range []string{"ECO", "SND", "RCV"} {
		t.Run(cmd, func(t *testing.T) {
			c, client := pipeConn()
			defer c.rwc.Close()
			defer client.Close()

			go client.Write([]byte(cmd + "\r\n"))

			got, err := c.readCommand()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != cmd {
				t.Errorf("expected %q, got %q", cmd, got)
			}
		})
	}
}

func TestReadCommand_InvalidLength(t *testing.T) {
	c, client := pipeConn()
	defer c.rwc.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		_, err := c.readCommand()
		errCh <- err
	}()

	client.Write([]byte("SEND\r\n"))

	reader := bufio.NewReader(client)
	resp, _ := reader.ReadString('\n')

	if !strings.Contains(resp, "ERR:Invalid command received") {
		t.Errorf("expected ERR response, got %q", resp)
	}

	if err := <-errCh; err == nil {
		t.Error("expected error for invalid command length")
	}
}

func TestReadCommand_UnknownCommand(t *testing.T) {
	c, client := pipeConn()
	defer c.rwc.Close()
	defer client.Close()

	errCh := make(chan error, 1)
	go func() {
		_, err := c.readCommand()
		errCh <- err
	}()

	client.Write([]byte("FOO\r\n"))

	reader := bufio.NewReader(client)
	resp, _ := reader.ReadString('\n')

	if !strings.Contains(resp, "ERR:Invalid command received") {
		t.Errorf("expected ERR response, got %q", resp)
	}

	if err := <-errCh; err == nil {
		t.Error("expected error for unknown command")
	}
}

func TestIsConnClosed(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"broken pipe", errors.New("write: broken pipe"), true},
		{"connection reset", errors.New("read: connection reset by peer"), true},
		{"closed", errors.New("use of closed network connection"), true},
		{"other", errors.New("some other error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isConnClosed(tt.err); got != tt.want {
				t.Errorf("isConnClosed(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
