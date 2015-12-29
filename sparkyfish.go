package main

import (
	"bufio"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/dustin/randbo"
)

const (
	blockSize        int64  = 100
	reportIntervalMS uint64 = 500 // report interval in milliseconds
)

// FlowDirection is used to indicate whether we meter inbound or outbound traffic
type FlowDirection int

var (
	testLength *uint
	echoListen *string
)

const (
	Outbound FlowDirection = iota
	Inbound
)

// MeteredServer is a server that deliveres random data to the client and measures
// the throughput
type MeteredServer struct {
	blockTicker chan bool
	done        chan struct{}
	remoteAddr  string
}

func main() {
	listenAddr := flag.String("listen-addr", "0.0.0.0:7121", "IP address to listen on for speed tests (default: 0.0.0.0:7121)")
	testLength = flag.Uint("test-length", 15, "Length of time to run speed test")
	echoListen = flag.String("echo-listen-addr", "0.0.0.0:7122", "IP address to listen on for echo tests (default: 0.0.0.0:7122)")
	flag.Parse()

	go echoListener()

	http.HandleFunc("/", outboundHandler)
	err := http.ListenAndServe(*listenAddr, nil)
	if err != nil {
		panic(err)
	}
}

func echoListener() {
	l, err := net.Listen("tcp", *echoListen)
	if err != nil {
		panic(err)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go echoHandler(conn)
	}

	err = l.Close()
	if err != nil {
		log.Fatal(err)
	}

}

func echoHandler(conn net.Conn) {
	r := bufio.NewReader(conn)
	for c := 0; c <= 9; c++ {
		chr, err := r.ReadByte()
		if err != nil {
			log.Println("Error reading byte:", err)
			break
		}
		log.Println("Copying byte:", chr)
		_, err = conn.Write([]byte{chr})
		if err != nil {
			log.Println("Error writing byte:", err)
			break
		}
	}
	conn.Close()
}

func outboundHandler(w http.ResponseWriter, r *http.Request) {
	m := &MeteredServer{remoteAddr: r.RemoteAddr}
	m.done = make(chan struct{})
	m.blockTicker = make(chan bool)

	// Launch our throughput reporter in a goroutine
	go m.ReportThroughput()

	// Create a new randbo Reader
	rbr := randbo.New()

	// Start our metered copier and block until it finishes
	m.MeteredCopy(rbr, w, r, Outbound)

	// When our metered copy unblocks, the speed test is done, so we close
	// this channel to signal the throughput reporter to halt
	close(m.done)
}

func inboundHandler(w http.ResponseWriter, r *http.Request) {
	m := &MeteredServer{remoteAddr: r.RemoteAddr}
	m.done = make(chan struct{})
	m.blockTicker = make(chan bool)

	// Launch our throughput reporter in a goroutine
	go m.ReportThroughput()

	// Create a new randbo Reader
	rbr := randbo.New()

	// Start our metered copier and block until it finishes
	m.MeteredCopy(rbr, w, r, Inbound)

	// When our metered copy unblocks, the speed test is done, so we close
	// this channel to signal the throughput reporter to halt
	close(m.done)
}

// MeteredCopy copies from a Reader to a Writer, keeping count of the data it passes
func (m *MeteredServer) MeteredCopy(r io.Reader, w http.ResponseWriter, req *http.Request, dir FlowDirection) {
	var err error

	// We're going to be sending random, binary data out.
	w.Header().Set("Content-Type", "application/octet-stream")

	// We'll be running the test for 15 seconds
	timer := time.NewTimer(time.Second * time.Duration(*testLength))

	for {
		select {
		case <-timer.C:
			log.Println(*testLength, "seconds have elapsed.")
			return
		default:
			// Copy our random data from randbo to our ResponseWriter, 100KB at a time
			switch dir {
			case Outbound:
				_, err = io.CopyN(w, r, 1024*blockSize)
			case Inbound:
				_, err = io.CopyN(ioutil.Discard, req.Body, 1024*blockSize)
			}
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
