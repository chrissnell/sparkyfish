package backend

import (
	"context"
	"time"
)

// ServerInfo holds metadata returned during a backend handshake.
type ServerInfo struct {
	Hostname string
	Location string
}

// PingSample is a single round-trip latency measurement.
type PingSample struct {
	Seq     int
	Latency time.Duration
}

// ThroughputSample is a periodic throughput measurement.
type ThroughputSample struct {
	Mbps float64
	Time time.Time
}

// Backend abstracts a speed testing protocol.
type Backend interface {
	// Connect performs the initial handshake and returns server metadata.
	// The addr is stored for subsequent per-test connections.
	Connect(ctx context.Context, addr string) (ServerInfo, error)

	// Ping runs a latency test, sending samples on results.
	// The channel is closed when the test completes.
	Ping(ctx context.Context, results chan<- PingSample) error

	// Download runs a download throughput test, sending periodic samples.
	// The channel is closed when the test completes.
	Download(ctx context.Context, results chan<- ThroughputSample) error

	// Upload runs an upload throughput test, sending periodic samples.
	// The channel is closed when the test completes.
	Upload(ctx context.Context, results chan<- ThroughputSample) error
}
