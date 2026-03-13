package server

import (
	"bytes"
	"fmt"
	"io"
	"time"
)

const (
	sendTestLength = 10 * time.Second
	recvTestLength = 12 * time.Second // 10s + 2s grace for client to finish
	sendBufSize    = 10 * 1024 * 1024 // copy entire 10MB buffer per iteration
	recvBlockSize  = 1024 * 1024      // read in ~1MB chunks
)

func (s *Server) handleSend(c *conn) {
	reader := bytes.NewReader(s.randBuf)
	start := time.Now()
	timer := time.NewTimer(sendTestLength)
	defer timer.Stop()

	var totalBytes int64
	for {
		select {
		case <-timer.C:
			s.logThroughput(c, "sent", totalBytes, time.Since(start))
			return
		default:
		}
		n, err := io.CopyN(c.rwc, reader, sendBufSize)
		totalBytes += n
		if err != nil {
			if err != io.EOF && !isConnClosed(err) {
				c.logger.Error("send copy", "err", err)
			}
			s.logThroughput(c, "sent", totalBytes, time.Since(start))
			return
		}
		reader.Seek(0, io.SeekStart)
	}
}

func (s *Server) handleReceive(c *conn) {
	start := time.Now()
	timer := time.NewTimer(recvTestLength)
	defer timer.Stop()

	var totalBytes int64
	for {
		select {
		case <-timer.C:
			s.logThroughput(c, "received", totalBytes, time.Since(start))
			return
		default:
		}
		n, err := io.CopyN(io.Discard, c.rwc, recvBlockSize)
		totalBytes += n
		if err != nil {
			if err != io.EOF && !isConnClosed(err) {
				c.logger.Error("recv copy", "err", err)
			}
			s.logThroughput(c, "received", totalBytes, time.Since(start))
			return
		}
	}
}

func (s *Server) logThroughput(c *conn, direction string, totalBytes int64, dur time.Duration) {
	mb := float64(totalBytes) / (1024 * 1024)
	secs := dur.Seconds()
	s.logger.Info(direction,
		"addr", c.rwc.RemoteAddr(),
		"mb", fmt.Sprintf("%.1f", mb),
		"duration", fmt.Sprintf("%.2fs", secs),
		"mbps", fmt.Sprintf("%.2f", mb/secs*8))
}
