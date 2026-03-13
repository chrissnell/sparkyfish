package server

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net"
	"os"
)

const randBufSize = 10 * 1024 * 1024 // 10 MB shared random data

// Config holds server configuration parsed from CLI flags.
type Config struct {
	ListenAddr string
	Cname      string
	Location   string
	Debug      bool
}

// Server accepts TCP connections and runs sparkyfish speed tests.
type Server struct {
	cfg     Config
	randBuf []byte
	logger  *slog.Logger
}

// New creates a Server with pre-generated random data for throughput tests.
func New(cfg Config) (*Server, error) {
	level := slog.LevelInfo
	if cfg.Debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	randBuf := make([]byte, randBufSize)
	if _, err := rand.Read(randBuf); err != nil {
		return nil, fmt.Errorf("generate random data: %w", err)
	}

	return &Server{cfg: cfg, randBuf: randBuf, logger: logger}, nil
}

// ListenAndServe starts the TCP listener and blocks until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", s.cfg.ListenAddr, err)
	}
	defer ln.Close()

	// Close listener on context cancellation to unblock Accept
	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	s.logger.Info("listening", "addr", s.cfg.ListenAddr)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil // clean shutdown
			}
			s.logger.Error("accept", "err", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(netConn net.Conn) {
	defer netConn.Close()

	c := newConn(netConn, s.logger)

	if err := c.handshake(s.cfg.Cname, s.cfg.Location); err != nil {
		s.logger.Debug("handshake failed", "addr", netConn.RemoteAddr(), "err", err)
		return
	}

	for {
		cmd, err := c.readCommand()
		if err != nil {
			s.logger.Debug("read command", "addr", netConn.RemoteAddr(), "err", err)
			return
		}

		s.logger.Info("test", "addr", netConn.RemoteAddr(), "cmd", cmd)

		switch cmd {
		case "ECO":
			s.handleEcho(c)
		case "SND":
			s.handleSend(c)
		case "RCV":
			s.handleReceive(c)
			return // RCV is terminal; client closes after upload
		}
	}
}
