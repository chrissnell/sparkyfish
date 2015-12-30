package main

import (
	"bufio"
	"bytes"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"time"

	"github.com/dustin/randbo"
)

const (
	blockSize        int64  = 100
	reportIntervalMS uint64 = 500 // report interval in milliseconds
)

// TestType is used to indicate the type of test being performed
type TestType int

var (
	testLength *uint
)

const (
	Outbound TestType = iota
	Inbound
	Echo
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
	flag.Parse()

	startListener(*listenAddr)
}

func startListener(listenAddr string) {
	testListener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		panic(err)
	}

	for {
		conn, err := testListener.Accept()
		if err != nil {
			log.Println("error accepting connection:", err)
			continue
		}
		go testHandler(conn)
	}

}

func testHandler(conn net.Conn) {
	defer conn.Close()
	var tt TestType

	// Valid client commands, issued upon successful connection
	cmdSND := []byte{'S', 'N', 'D'} // server->client speed test (download)
	cmdRCV := []byte{'R', 'C', 'V'} // client->server speed test (upload)
	cmdECO := []byte{'E', 'C', 'O'} // ping/echo test

	// Read a 3-byte command from the client
	cmd := make([]byte, 3, 3)
	_, err := conn.Read(cmd)
	if err != nil {
		log.Println("error reading from remote:", conn)
	}
	log.Println("COMMAND RECEIVED:", string(cmd))

	switch {
	case bytes.Compare(cmd, cmdSND) == 0:
		tt = Outbound
	case bytes.Compare(cmd, cmdRCV) == 0:
		tt = Inbound
	case bytes.Compare(cmd, cmdECO) == 0:
		tt = Echo
	default:
		// If the client didn't send a SND or RCV command, close the connection
		conn.Close()
		return
	}

	if tt == Echo {
		// Start an echo/ping test
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
		return
	} else {
		// Start an upload/download test
		m := &MeteredServer{remoteAddr: conn.RemoteAddr().String()}
		m.done = make(chan struct{})
		m.blockTicker = make(chan bool)

		// Launch our throughput reporter in a goroutine
		go m.ReportThroughput()

		// Start our metered copier and block until it finishes
		m.MeteredCopy(conn, tt)

		// When our metered copy unblocks, the speed test is done, so we close
		// this channel to signal the throughput reporter to halt
		close(m.done)
	}
}

// MeteredCopy copies to or from a net.Conn, keeping count of the data it passes
func (m *MeteredServer) MeteredCopy(conn net.Conn, dir TestType) {
	var err error

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
				// Create a new randbo Reader
				rnd := randbo.New()
				_, err = io.CopyN(conn, rnd, 1024*blockSize)
			case Inbound:
				_, err = io.CopyN(ioutil.Discard, conn, 1024*blockSize)
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
