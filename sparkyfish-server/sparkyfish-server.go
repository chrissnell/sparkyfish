package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/randbo"
)

var (
	cname    *string
	location *string
	debug    *bool
)

const (
	protocolVersion  uint16 = 0x00 // The latest version of the sparkyfish protocol supported
	blockSize        int64  = 1024 // size of each block copied to/from remote
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

type sparkyServer struct {
	randomData []byte
}

// newsparkyServer creates a sparkyServer object and pre-fills some random data
func newsparkyServer() sparkyServer {
	ss := sparkyServer{}

	// Make a 10MB byte slice
	ss.randomData = make([]byte, 1024*1024*10)

	// Fill our 10MB byte slice with random data
	_, err := randbo.New().Read(ss.randomData)
	if err != nil {
		log.Fatalln("error generating random data:", err)
	}

	return ss
}

// sparkyClient handles requests for throughput and latency tests
type sparkyClient struct {
	client      net.Conn
	testType    TestType
	reader      *bufio.Reader
	randReader  *bytes.Reader
	blockTicker chan bool
	done        chan bool
}

func newsparkyClient(client net.Conn) sparkyClient {
	sc := sparkyClient{client: client}
	return sc
}

func startListener(listenAddr string, ss *sparkyServer) {
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
		go handler(conn, ss)
	}

}

func handler(conn net.Conn, ss *sparkyServer) {
	var version uint64

	sc := newsparkyClient(conn)

	sc.done = make(chan bool)
	sc.blockTicker = make(chan bool, 200)

	// Create a bytes.Reader over our pre-filled 10MB byte slice
	sc.randReader = bytes.NewReader(ss.randomData)

	defer sc.client.Close()

	sc.reader = bufio.NewReader(sc.client)

	// Every connection begins with a HELO<version> command,
	// where <version> is one byte that will be converted to a uint16
	helo, err := sc.reader.ReadString('\n')
	if err != nil {
		// If a client hangs up, just hang up silently
		return
	}
	helo = strings.TrimSpace(helo)

	if *debug {
		log.Println("COMMAND RECEIVED:", helo)
	}

	// The HELO command must be exactly 7 bytes long, including version and CRLF
	if len(helo) != 5 {
		sc.client.Write([]byte("ERR:Invalid HELO received\n"))
		return
	}

	if helo[:4] != "HELO" {
		sc.client.Write([]byte("ERR:Invalid HELO received\n"))
		return
	}

	// Parse the version number
	version, err = strconv.ParseUint(helo[4:], 10, 16)
	if err != nil {
		sc.client.Write([]byte("ERR:Invalid HELO received\n"))
		log.Println("error parsing version", err)
		return
	}

	if *debug {
		log.Printf("HELO received.  Version: %#x", version)
	}

	// Close the connection if the client requests a protocol version
	// greater than what we support
	if uint16(version) > protocolVersion {
		sc.client.Write([]byte("ERR:Protocol version not supported\n"))
		log.Println("Invalid protocol version requested", version)
		return
	}

	banner := bytes.NewBufferString("HELO\n")
	if *cname != "" {
		banner.WriteString(fmt.Sprintln(*cname))
	} else {
		banner.WriteString("none\n")
	}
	if *location != "" {
		banner.WriteString(fmt.Sprintln(*location))
	} else {
		banner.WriteString("none\n")
	}

	_, err = banner.WriteTo(sc.client)
	if err != nil {
		log.Println("error writing HELO response to client:", err)
		return
	}

	cmd, err := sc.reader.ReadString('\n')
	if err != nil {
		return
	}
	cmd = strings.TrimSpace(cmd)

	if *debug {
		log.Println("COMMAND RECEIVED:", string(cmd))
	}

	if len(cmd) != 3 {
		sc.client.Write([]byte("ERR:Invalid command received\n"))
		return
	}

	switch cmd {
	case "SND":
		sc.testType = outbound
		log.Printf("[%v] initiated download test", sc.client.RemoteAddr())
	case "RCV":
		sc.testType = inbound
		log.Printf("[%v] initiated upload test", sc.client.RemoteAddr())
	case "ECO":
		sc.testType = echo
		log.Printf("[%v] initiated echo test", sc.client.RemoteAddr())
	default:
		sc.client.Write([]byte("ERR:Invalid command received"))
		return
	}

	if sc.testType == echo {
		// Start an echo/ping test and block until it finishes
		sc.echoTest()
	} else {
		// Start an upload/download test

		// Launch our throughput reporter in a goroutine
		go sc.ReportThroughput()

		// Start our metered copier and block until it finishes
		sc.MeteredCopy()

		// When our metered copy unblocks, the speed test is done, so we close
		// this channel to signal the throughput reporter to halt
		sc.done <- true
	}
}

func (sc *sparkyClient) echoTest() {
	for c := 0; c <= pingTestLength-1; c++ {
		chr, err := sc.reader.ReadByte()
		if err != nil {
			log.Println("Error reading byte:", err)
			break
		}
		if *debug {
			log.Println("Copying byte:", chr)
		}
		_, err = sc.client.Write([]byte{chr})
		if err != nil {
			log.Println("Error writing byte:", err)
			break
		}
	}
	return
}

// MeteredCopy copies to or from a net.Conn, keeping count of the data it passes
func (sc *sparkyClient) MeteredCopy() {
	var err error
	var timer *time.Timer

	// Set a timer that we'll use to stop the test.  If we're running an inbound test,
	// we extend the timer by two seconds to allow the client to finish its sending.
	if sc.testType == inbound {
		timer = time.NewTimer(time.Second * time.Duration(testLength+2))
	} else if sc.testType == outbound {
		timer = time.NewTimer(time.Second * time.Duration(testLength))
	}

	for {
		select {
		case <-timer.C:
			if *debug {
				log.Println(testLength, "seconds have elapsed.")
			}
			return
		default:
			// Copy our random data from randbo to our ResponseWriter, 100KB at a time
			switch sc.testType {
			case outbound:
				// Try to copy the entire 10MB bytes.Reader to the client.
				_, err = io.CopyN(sc.client, sc.randReader, 1024*1024*10)
			case inbound:
				_, err = io.CopyN(ioutil.Discard, sc.client, 1024*blockSize)
			}

			// io.EOF is normal when a client drops off after the test
			if err != nil {
				if err != io.EOF {
					log.Println("Error copying:", err)
				}
				return
			}

			// Seek back to the beginning of the bytes.Reader
			sc.randReader.Seek(0, 0)

			// // With each 100K copied, we send a message on our blockTicker channel
			sc.blockTicker <- true
		}
	}
}

// ReportThroughput reports on throughput of data passed by MeterWrite
func (sc *sparkyClient) ReportThroughput() {
	var blockCount, prevBlockCount uint64

	tick := time.NewTicker(time.Duration(reportIntervalMS) * time.Millisecond)

	start := time.Now()

blockcounter:
	for {
		select {
		case <-sc.blockTicker:
			// Increment our block counter when we get a ticker
			blockCount++
		case <-sc.done:
			tick.Stop()
			break blockcounter
		case <-tick.C:
			// Every second, we calculate how many blocks were received
			// and derive an average throughput rate.
			if *debug {
				log.Printf("[%v] %v Kbit/sec", sc.client.RemoteAddr(), (blockCount-prevBlockCount)*uint64(blockSize*8)*(1000/reportIntervalMS))
			}
			prevBlockCount = blockCount
		}
	}

	duration := time.Now().Sub(start).Seconds()
	if sc.testType == outbound {
		mbCopied := float64(blockCount * 10)
		log.Printf("[%v] Sent %v MB in %.2f seconds (%.2f Mbit/s)", sc.client.RemoteAddr(), mbCopied, duration, (mbCopied/duration)*8)
	} else if sc.testType == inbound {
		mbCopied := float64(blockCount * uint64(blockSize) / 1024)
		log.Printf("[%v] Recd %v MB in %.2f seconds (%.2f) Mbit/s", sc.client.RemoteAddr(), mbCopied, duration, (mbCopied/duration)*8)
	}
}

func main() {
	listenAddr := flag.String("listen-addr", ":7121", "IP:Port to listen on for speed tests (default: all IPs, port 7121)")
	debug = flag.Bool("debug", false, "Print debugging information to stdout")

	// Fetch our hostname.  Reported to the client after a successful HELO
	cname = flag.String("cname", "", "Canonical hostname or IP address to optionally report to client. If you specify one, it must be DNS-resolvable.")
	location = flag.String("location", "", "Location of server (e.g. \"Dallas, TX\") [optional]")
	flag.Parse()

	ss := newsparkyServer()

	startListener(*listenAddr, &ss)
}
