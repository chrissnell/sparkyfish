package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/dustin/randbo"
)

const (
	blockSize        int64  = 100
	reportIntervalMS uint64 = 500 // report interval in milliseconds
)

// MeteredServer is a server that deliveres random data to the client and measures
// the throughput
type MeteredServer struct {
	blockTicker chan bool
	done        chan struct{}
	remoteAddr  string
}

func main() {
	listenPort := flag.String("listenport", "7121", "Port to listen on")
	listenAddr := flag.String("listenaddr", "", "IP address to listen on (default: all)")
	flag.Parse()

	listenHost := fmt.Sprint(*listenAddr, ":", *listenPort)

	http.HandleFunc("/", handler)
	err := http.ListenAndServe(listenHost, nil)
	if err != nil {
		panic(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	m := &MeteredServer{remoteAddr: r.RemoteAddr}
	m.done = make(chan struct{})
	m.blockTicker = make(chan bool)

	// Launch our throughput reporter in a goroutine
	go m.ReportThroughput()

	// Create a new randbo Reader
	rbr := randbo.New()

	// Start our metered copier and block until it finishes
	m.MeteredCopy(rbr, w)

	// When our metered copy unblocks, the speed test is done, so we close
	// this channel to signal the throughput reporter to halt
	close(m.done)
}

// MeteredCopy copies from a Reader to a Writer, keeping count of the data it passes
func (m *MeteredServer) MeteredCopy(r io.Reader, w http.ResponseWriter) {
	// We're going to be sending random, binary data out.
	w.Header().Set("Content-Type", "application/octet-stream")

	// We'll be running the test for 15 seconds
	timer := time.NewTimer(time.Second * 15)

	for {
		select {
		case <-timer.C:
			log.Println("15 seconds has elapsed.")
			return
		default:
			// Copy our random data from randbo to our ResponseWriter, 100KB at a time
			_, err := io.CopyN(w, r, 1024*blockSize)
			if err != nil {
				log.Println("Error copying:", err)
				return
			}

			// // With each 100K copied, we send a message on our blockTicker channel
			m.blockTicker <- true
		}
	}
	return
}

// ReportThroughput reports on throughput of data passed by MeterWrite
func (m *MeteredServer) ReportThroughput() {
	var blockCount, prevBlockCount uint64

	tick := time.NewTicker(time.Duration(reportIntervalMS) * time.Millisecond)
	for {
		select {
		case <-m.blockTicker:
			// Increment our block counter when we get a ticker
			blockCount++
		case <-m.done:
			log.Println("ReportThroughput() Done")
			tick.Stop()
			return
		case <-tick.C:
			// Every second, we calculate how many blocks were received
			// and derive an average throughput rate.
			log.Printf("[%v] %v KB/sec", m.remoteAddr, (blockCount-prevBlockCount)*uint64(blockSize)*(1000/reportIntervalMS))
			prevBlockCount = blockCount
		}
	}
}
