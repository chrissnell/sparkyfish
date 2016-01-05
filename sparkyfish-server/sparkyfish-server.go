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
	outbound TestType = iota
	inbound
	echo
)

// sparkyServer handles requests for throughput and latency tests
type sparkyServer struct {
	client      net.Conn
	testType    TestType
	reader      *bufio.Reader
	blockTicker chan bool
	done        chan bool
}

func newSparkyServer(client net.Conn) sparkyServer {
	ss := sparkyServer{client: client}
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
	var version uint64

	ss := newSparkyServer(conn)

	ss.done = make(chan bool)
	ss.blockTicker = make(chan bool, 200)

	defer ss.client.Close()

	ss.reader = bufio.NewReader(ss.client)

	// Every connection begins with a HELO<version> command,
	// where <version> is one byte that will be converted to a uint16
	helo, err := ss.reader.ReadString('\n')
	if err != nil {
		log.Println("error reading from remote:", err)
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
				ss.client.Write([]byte("ERR:Protocol version not supported\n"))
				log.Println("Invalid protocol version requested", version)
				return
			}

		} else {
			ss.client.Write([]byte("ERR:Invalid HELO received\n"))
			return
		}
	} else {
		ss.client.Write([]byte("ERR:Invalid HELO received\n"))
		return
	}

	cmd, err := ss.reader.ReadString('\n')
	if err != nil {
		log.Println("error reading from remote:", err)
		return
	}
	if *debug {
		log.Println("COMMAND RECEIVED:", string(cmd))
	}

	if len(cmd) < 4 {
		ss.client.Write([]byte("ERR:Invalid command received\n"))
		return
	}

	switch string(cmd[:3]) {
	case "SND":
		ss.testType = outbound
		log.Printf("[%v] initiated download test", ss.client.RemoteAddr())
	case "RCV":
		ss.testType = inbound
		log.Printf("[%v] initiated upload test", ss.client.RemoteAddr())
	case "ECO":
		ss.testType = echo
		log.Printf("[%v] initiated echo test", ss.client.RemoteAddr())
	default:
		ss.client.Write([]byte("ERR:Invalid command received"))
		return
	}

	if ss.testType == echo {
		// Start an echo/ping test and block until it finishes
		ss.echoTest()
	} else {
		// Start an upload/download test

		// Launch our throughput reporter in a goroutine
		go ss.ReportThroughput()

		// Start our metered copier and block until it finishes
		ss.MeteredCopy()

		// When our metered copy unblocks, the speed test is done, so we close
		// this channel to signal the throughput reporter to halt
		ss.done <- true
	}
}

func (ss *sparkyServer) echoTest() {
	for c := 0; c <= pingTestLength-1; c++ {
		chr, err := ss.reader.ReadByte()
		if err != nil {
			log.Println("Error reading byte:", err)
			break
		}
		if *debug {
			log.Println("Copying byte:", chr)
		}
		_, err = ss.client.Write([]byte{chr})
		if err != nil {
			log.Println("Error writing byte:", err)
			break
		}
	}
	return
}

// MeteredCopy copies to or from a net.Conn, keeping count of the data it passes
func (ss *sparkyServer) MeteredCopy() {
	var err error
	var timer *time.Timer

	// Set a timer that we'll use to stop the test.  If we're running an inbound test,
	// we extend the timer by two seconds to allow the client to finish its sending.
	if ss.testType == inbound {
		timer = time.NewTimer(time.Second * time.Duration(testLength+2))
	} else if ss.testType == outbound {
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
			switch ss.testType {
			case outbound:
				_, err = io.CopyN(ss.client, rnd, 1024*blockSize)
			case inbound:
				_, err = io.CopyN(ioutil.Discard, ss.client, 1024*blockSize)
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
func (ss *sparkyServer) ReportThroughput() {
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
				log.Printf("[%v] %v Kbit/sec", ss.client.RemoteAddr(), (blockCount-prevBlockCount)*uint64(blockSize*8)*(1000/reportIntervalMS))
			}
			prevBlockCount = blockCount
		}
	}

	mbCopied := float64(blockCount * uint64(blockSize) / 1000)
	duration := time.Now().Sub(start).Seconds()
	if ss.testType == outbound {
		log.Printf("[%v] Sent %v MB in %.2f seconds (%.2f Mbit/s)", ss.client.RemoteAddr(), mbCopied, duration, (mbCopied/duration)*8)
	} else if ss.testType == inbound {
		log.Printf("[%v] Recd %v MB in %.2f seconds (%.2f) Mbit/s", ss.client.RemoteAddr(), mbCopied, duration, (mbCopied/duration)*8)
	}
}

func main() {
	listenAddr := flag.String("listen-addr", ":7121", "IP:Port to listen on for speed tests (default: all IPs, port 7121)")
	debug = flag.Bool("debug", false, "Print debugging information to stdout")
	flag.Parse()

	startListener(*listenAddr)
}
