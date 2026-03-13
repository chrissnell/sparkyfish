package sparkyfish

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"time"

	"github.com/chrissnell/sparkyfish/pkg/backend"
)

const (
	reportInterval       = 500 * time.Millisecond
	throughputTestLength = 10 * time.Second
	downloadGrace        = 2 * time.Second // extra time for server startup
	numPings             = 30
	randBufSize          = 10 * 1024 * 1024 // 10 MB
)

// blockSizeForElapsed returns a ramping block size so that charts update
// quickly at the start of a test and settle into larger, more efficient
// blocks once throughput stabilizes.
func blockSizeForElapsed(elapsed time.Duration) int64 {
	switch {
	case elapsed < 1*time.Second:
		return 8 * 1024 // 8 KB
	case elapsed < 2*time.Second:
		return 64 * 1024 // 64 KB
	case elapsed < 3*time.Second:
		return 256 * 1024 // 256 KB
	default:
		return 1024 * 1024 // 1 MB
	}
}

// Client implements backend.Backend for the sparkyfish protocol.
type Client struct {
	addr       string
	serverInfo backend.ServerInfo
	randBuf    []byte
}

func New() *Client {
	return &Client{}
}

func (c *Client) Connect(ctx context.Context, addr string) (backend.ServerInfo, error) {
	c.addr = addr

	// Pre-generate random data for upload tests
	c.randBuf = make([]byte, randBufSize)
	if _, err := rand.Read(c.randBuf); err != nil {
		return backend.ServerInfo{}, fmt.Errorf("generate random data: %w", err)
	}

	// Test the connection with a handshake
	s, info, err := dial(addr)
	if err != nil {
		return backend.ServerInfo{}, err
	}
	s.Close()

	c.serverInfo = info
	return info, nil
}

func (c *Client) Ping(ctx context.Context, results chan<- backend.PingSample) error {
	defer close(results)

	s, _, err := dial(c.addr)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.writeCommand("ECO"); err != nil {
		return fmt.Errorf("send ECO: %w", err)
	}

	buf := make([]byte, 1)
	for i := 0; i < numPings; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		start := time.Now()
		if _, err := s.conn.Write([]byte{46}); err != nil {
			return fmt.Errorf("ping write: %w", err)
		}
		if _, err := s.conn.Read(buf); err != nil {
			return fmt.Errorf("ping read: %w", err)
		}
		latency := time.Since(start)

		select {
		case results <- backend.PingSample{Seq: i, Latency: latency}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

func (c *Client) Download(ctx context.Context, results chan<- backend.ThroughputSample) error {
	defer close(results)

	s, _, err := dial(c.addr)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.writeCommand("SND"); err != nil {
		return fmt.Errorf("send SND: %w", err)
	}

	return c.measureThroughput(ctx, s, results, func(size int64) (int64, error) {
		return io.CopyN(io.Discard, s.conn, size)
	}, throughputTestLength+downloadGrace)
}

func (c *Client) Upload(ctx context.Context, results chan<- backend.ThroughputSample) error {
	defer close(results)

	s, _, err := dial(c.addr)
	if err != nil {
		return err
	}
	defer s.Close()

	if err := s.writeCommand("RCV"); err != nil {
		return fmt.Errorf("send RCV: %w", err)
	}

	reader := bytes.NewReader(c.randBuf)
	return c.measureThroughput(ctx, s, results, func(size int64) (int64, error) {
		if int64(reader.Len()) <= size {
			reader.Seek(0, io.SeekStart)
		}
		return io.CopyN(s.conn, reader, size)
	}, throughputTestLength)
}

// measureThroughput runs a throughput test with ramping block sizes.
// copyBlock is called repeatedly to transfer one block of the given size.
func (c *Client) measureThroughput(
	ctx context.Context,
	s *session,
	results chan<- backend.ThroughputSample,
	copyBlock func(size int64) (int64, error),
	timeout time.Duration,
) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	ticker := time.NewTicker(reportInterval)
	defer ticker.Stop()

	start := time.Now()
	var totalBytes, prevBytes int64

	// Byte-counting goroutine
	bytesCh := make(chan int64, 256)
	copyDone := make(chan error, 1)
	go func() {
		for {
			select {
			case <-timer.C:
				copyDone <- nil
				return
			default:
			}
			size := blockSizeForElapsed(time.Since(start))
			n, err := copyBlock(size)
			if err != nil {
				if err == io.EOF || isConnectionClosed(err) {
					copyDone <- nil
					return
				}
				copyDone <- err
				return
			}
			bytesCh <- n
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-copyDone:
			return err
		case n := <-bytesCh:
			totalBytes += n
		case <-ticker.C:
			delta := totalBytes - prevBytes
			mbps := float64(delta) * 8 / reportInterval.Seconds() / 1_000_000
			prevBytes = totalBytes

			select {
			case results <- backend.ThroughputSample{Mbps: mbps, Time: time.Now()}:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

func isConnectionClosed(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "broken pipe") ||
		contains(msg, "connection reset") ||
		contains(msg, "closed")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
