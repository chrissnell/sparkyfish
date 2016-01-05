package main

import (
	"bufio"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/dustin/randbo"
)

var debug *bool

const (
	protocolVersion  uint16 = 0x00 // The latest version of the sparkyfish protocol supported
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
	client      net.Conn
	blockTicker chan bool
	done        chan bool
	remoteAddr  string
}

func newSparkyServer(remoteAddr string) sparkyServer {
	ss := sparkyServer{remoteAddr: remoteAddr}
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
	var tt TestType
	var version uint64

	defer conn.Close()

	reader := bufio.NewReader(conn)

	helo, err := reader.ReadString('\n')
	if err != nil {
		log.Println("error reading from remote:", conn)
	}
	if *debug {
		log.Println("COMMAND RECEIVED:", helo[:len(helo)-2])
	}

	if len(helo) == 7 {
		if helo[:4] == "HELO" {
			version, err = strconv.ParseUint(helo[4:5], 10, 16)
			if err != nil {
				log.Println("error parsing version", err)
				return
			}
			log.Printf("HELO received.  Version: %#x", version)
			if uint16(version) > protocolVersion {
				conn.Write([]byte("ERR:Protocol version not supported\n"))
				log.Println("Invalid protocol version requested", version)
				return
			}

		} else {
			conn.Write([]byte("ERR:Invalid HELO received\n"))
			return
		}
	} else {
		conn.Write([]byte("ERR:Invalid HELO received\n"))
		return
	}

	cmd, err := reader.ReadString('\n')
	if err != nil {
		log.Println("error reading from remote:", conn)
		return
	}
	if *debug {
		log.Println("COMMAND RECEIVED:", string(cmd))
	}

	if len(cmd) < 4 {
		conn.Write([]byte("ERR:Invalid command received\n"))
		return
	}

	switch string(cmd[:3]) {
	case "SND":
		tt = Outbound
		log.Printf("[%v] initiated download test", conn.RemoteAddr())
	case "RCV":
		tt = Inbound
		log.Printf("[%v] initiated upload test", conn.RemoteAddr())
	case "ECO":
		tt = Echo
		log.Printf("[%v] initiated echo test", conn.RemoteAddr())
	default:
		conn.Write([]byte("ERR:Invalid command received"))
		return
	}

	if tt == Echo {
		// Start an echo/ping test
		for c := 0; c <= pingTestLength-1; c++ {
			chr, err := reader.ReadByte()
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
		ss := newSparkyServer(conn.RemoteAddr().String())
		ss.done = make(chan bool)
		ss.blockTicker = make(chan bool, 200)

		// Launch our throughput reporter in a goroutine
		go ss.ReportThroughput(tt)

		// Start our metered copier and block until it finishes
		ss.MeteredCopy(conn, tt)

		// When our metered copy unblocks, the speed test is done, so we close
		// this channel to signal the throughput reporter to halt
		ss.done <- true
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
