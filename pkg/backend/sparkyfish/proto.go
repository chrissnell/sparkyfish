package sparkyfish

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/chrissnell/sparkyfish/pkg/backend"
	"github.com/chrissnell/sparkyfish/pkg/resolver"
)

const protocolVersion = 0

// session represents a single TCP connection to the sparkyfish server.
type session struct {
	conn   net.Conn
	reader *bufio.Reader
}

// dial opens a TCP connection and performs the HELO handshake.
// Returns a session and the server info from the handshake.
func dial(addr string) (*session, backend.ServerInfo, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, backend.ServerInfo{}, fmt.Errorf("parse address %s: %w", addr, err)
	}

	ctx := context.Background()
	ips, err := resolver.LookupHost(ctx, host)
	if err != nil {
		return nil, backend.ServerInfo{}, fmt.Errorf("resolve %s: %w", host, err)
	}

	var conn net.Conn
	var lastErr error
	for _, ip := range ips {
		conn, lastErr = net.Dial("tcp", net.JoinHostPort(ip, port))
		if lastErr == nil {
			break
		}
	}
	if lastErr != nil {
		return nil, backend.ServerInfo{}, fmt.Errorf("dial %s: %w", addr, lastErr)
	}

	s := &session{
		conn:   conn,
		reader: bufio.NewReader(conn),
	}

	info, err := s.helo(addr)
	if err != nil {
		conn.Close()
		return nil, backend.ServerInfo{}, err
	}

	return s, info, nil
}

// helo performs the HELO handshake and returns server metadata.
func (s *session) helo(addr string) (backend.ServerInfo, error) {
	if err := s.writeCommand(fmt.Sprintf("HELO%d", protocolVersion)); err != nil {
		return backend.ServerInfo{}, fmt.Errorf("send HELO: %w", err)
	}

	response, err := s.reader.ReadString('\n')
	if err != nil {
		return backend.ServerInfo{}, fmt.Errorf("read HELO response: %w", err)
	}
	if strings.TrimSpace(response) != "HELO" {
		return backend.ServerInfo{}, fmt.Errorf("invalid HELO response: %q", response)
	}

	cname, err := s.reader.ReadString('\n')
	if err != nil {
		return backend.ServerInfo{}, fmt.Errorf("read cname: %w", err)
	}
	cname = sanitize(strings.TrimSpace(cname))
	if cname == "none" {
		host, _, _ := net.SplitHostPort(addr)
		cname = host
	}

	location, err := s.reader.ReadString('\n')
	if err != nil {
		return backend.ServerInfo{}, fmt.Errorf("read location: %w", err)
	}
	location = sanitize(strings.TrimSpace(location))
	if location == "none" {
		location = ""
	}

	return backend.ServerInfo{Hostname: cname, Location: location}, nil
}

// writeCommand sends a command string followed by \r\n.
func (s *session) writeCommand(cmd string) error {
	_, err := fmt.Fprintf(s.conn, "%s\r\n", cmd)
	return err
}

func (s *session) Close() error {
	return s.conn.Close()
}

// sanitize strips non-printable ASCII characters.
func sanitize(str string) string {
	b := make([]byte, 0, len(str))
	for i := 0; i < len(str); i++ {
		c := str[i]
		if c >= 32 && c < 127 {
			b = append(b, c)
		}
	}
	return string(b)
}
