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

var debug *bool

const (
	blockSize        int64  = 200  // size of each block copied to/from remote
	reportIntervalMS uint64 = 1000 // report interval in milliseconds
	testLength       uint   = 10   // length of throughput tests (sec)
	pingTestLength   int    = 30   // number of pings allowed in a ping test

)

// TestType is used to indicate the type of test being performed
type TestType int

const (
	Outbound TestType = iota
	Inbound
	Echo
)

// sparkyServer handles requests for throughput and latency tests
type sparkyServer struct {
	blockTicker chan bool
	done        chan struct{}
	remoteAddr  string
}

func NewSparkyServer(remoteAddr string) *sparkyServer {
	ss := &sparkyServer{remoteAddr: remoteAddr}
	return ss
}

func startListener(listenAddr string) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		panic(err)
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("error accepting connection:", err)
			continue
		}
		go handler(conn)
	}

}

func handler(conn net.Conn) {
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
	if *debug {
		log.Println("COMMAND RECEIVED:", string(cmd))
	}

	switch {
	case bytes.Compare(cmd, cmdSND) == 0:
		tt = Outbound
		log.Printf("[%v] initiated download test", conn.RemoteAddr())
	case bytes.Compare(cmd, cmdRCV) == 0:
		tt = Inbound
		log.Printf("[%v] initiated upload test", conn.RemoteAddr())
	case bytes.Compare(cmd, cmdECO) == 0:
		tt = Echo
		log.Printf("[%v] initiated echo test", conn.RemoteAddr())
	default:
		// If the client didn't send a SND or RCV command, close the connection
		conn.Close()
		return
	}

	if tt == Echo {
		// Start an echo/ping test
		r := bufio.NewReader(conn)
		for c := 0; c <= pingTestLength-1; c++ {
			chr, err := r.ReadByte()
			if err != nil {
				log.Println("Error reading byte:", err)
				break
			}
			if *debug {
				log.Println("Copying byte:", chr)
			}
			_, err = conn.Write([]byte{chr})
			if err != nil {
				log.Println("Error writing byte:", err)
				break
			}
		}
		return
	} else {
		// Start an upload/download test
		ss := NewSparkyServer(conn.RemoteAddr().String())
		ss.done = make(chan struct{})
		ss.blockTicker = make(chan bool, 200)

		// Launch our throughput reporter in a goroutine
		go ss.ReportThroughput(tt)

		// Start our metered copier and block until it finishes
		ss.MeteredCopy(conn, tt)

		// When our metered copy unblocks, the speed test is done, so we close
		// this channel to signal the throughput reporter to halt
		close(ss.done)
	}
}

// MeteredCopy copies to or from a net.Conn, keeping count of the data it passes
func (ss *sparkyServer) MeteredCopy(conn net.Conn, dir TestType) {
	var err error
	var timer *time.Timer

	// Set a timer that we'll use to stop the test.  If we're running an inbound test,
	// we extend the timer by two seconds to allow the client to finish its sending.
	if dir == Inbound {
		timer = time.NewTimer(time.Second * time.Duration(testLength+2))
	} else if dir == Outbound {
		timer = time.NewTimer(time.Second * time.Duration(testLength))
	}

	// Create a new randbo Reader
	rnd := randbo.New()

	for {
		select {
		case <-timer.C:
			if *debug {
				log.Println(testLength, "seconds have elapsed.")
			}
			return
		default:
			// Copy our random data from randbo to our ResponseWriter, 100KB at a time
			switch dir {
			case Outbound:
				_, err = io.CopyN(conn, rnd, 1024*blockSize)
			case Inbound:
				_, err = io.CopyN(ioutil.Discard, conn, 1024*blockSize)
			}

			// io.EOF is normal when a client drops off after the test
			if err != nil {
				if err != io.EOF {
					log.Println("Error copying:", err)
				}
				return
			}

			// // With each 100K copied, we send a message on our blockTicker channel
			ss.blockTicker <- true
		}
	}
	return
}

// ReportThroughput reports on throughput of data passed by MeterWrite
func (ss *sparkyServer) ReportThroughput(tt TestType) {
	var blockCount, prevBlockCount uint64

	tick := time.NewTicker(time.Duration(reportIntervalMS) * time.Millisecond)

	start := time.Now()

blockcounter:
	for {
		select {
		case <-ss.blockTicker:
			// Increment our block counter when we get a ticker
			blockCount++
		case <-ss.done:
			tick.Stop()
			break blockcounter
		case <-tick.C:
			// Every second, we calculate how many blocks were received
			// and derive an average throughput rate.
			if *debug {
				log.Printf("[%v] %v Kbit/sec", ss.remoteAddr, (blockCount-prevBlockCount)*uint64(blockSize*8)*(1000/reportIntervalMS))
			}
			prevBlockCount = blockCount
		}
	}

	mbCopied := float64(blockCount * uint64(blockSize) / 1000)
	duration := time.Now().Sub(start).Seconds()
	if tt == Outbound {
		log.Printf("[%v] Sent %v MB in %.2f seconds (%.2f Mbit/s)", ss.remoteAddr, mbCopied, duration, (mbCopied/duration)*8)
	} else if tt == Inbound {
		log.Printf("[%v] Recd %v MB in %.2f seconds (%.2f) Mbit/s", ss.remoteAddr, mbCopied, duration, (mbCopied/duration)*8)
	}
}

func main() {
	listenAddr := flag.String("listen-addr", ":7121", "IP:Port to listen on for speed tests (default: all IPs, port 7121)")
	debug = flag.Bool("debug", false, "Print debugging information to stdout")
	flag.Parse()

	startListener(*listenAddr)
}
