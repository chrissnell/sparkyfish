package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/dustin/randbo"
	"github.com/gizak/termui"
)

// Kick off a throughput measurement test
func (mc *meteredClient) runThroughputTest(dir testType) {
	// Notify the progress bar updater to reset the bar
	mc.progressBarReset <- true

	// Used to signal test completion to the throughput measurer
	measurerDone := make(chan struct{})

	// Launch a throughput measurer and then kick off the metered copy,
	// blocking until it completes.
	go mc.MeasureThroughput(measurerDone)
	mc.MeteredCopy(dir, measurerDone)

	// Notify the progress bar updater that the test is done
	mc.testDone <- true
}

// Kicks off a metered copy (throughput test) by sending a command to the server
// and then performing the appropriate I/O copy, sending "ticks" by channel as
// each block of data passes through.
func (mc *meteredClient) MeteredCopy(dir testType, measurerDone chan<- struct{}) {
	var rnd io.Reader
	var tl time.Duration

	// Connect to the remote sparkyfish server
	conn, err := net.Dial("tcp", os.Args[1])
	if err != nil {
		termui.Close()
		log.Fatalln(err)
	}

	defer conn.Close()

	// Send the appropriate command to the sparkyfish server to initiate our
	// throughput test
	switch dir {
	case inbound:
		// For inbound tests, we bump our timer by 2 seconds to account for
		// the remote server's test startup time
		tl = time.Second * time.Duration(throughputTestLength+2)

		// Send the SND command to the remote server, requesting a download test
		// (remote sends).
		_, err = conn.Write([]byte("SND"))
		if err != nil {
			termui.Close()
			log.Fatalln(err)
		}
	case outbound:
		tl = time.Second * time.Duration(throughputTestLength)

		// Send the RCV command to the remote server, requesting an upload test
		// (remote receives).
		_, err = conn.Write([]byte("RCV"))
		if err != nil {
			termui.Close()
			log.Fatalln(err)
		}
		// Create a new randbo Reader, used to generate our random data that we'll upload
		rnd = randbo.New()
	}

	// Set a timer for running the tests
	timer := time.NewTimer(tl)

	switch dir {
	case inbound:
		// Receive, tally, and discard incoming data as fast as we can until the sender stops sending or the timer expires
		for {
			select {
			case <-timer.C:
				// Timer has elapsed and test is finished
				close(measurerDone)
				return
			default:
				// Copy data from our net.Conn to the rubbish bin in (blockSize) KB chunks
				_, err = io.CopyN(ioutil.Discard, conn, 1024*blockSize)
				if err != nil {
					// Handle the EOF when the test timer has expired at the remote end.
					if err == io.EOF {
						close(measurerDone)
						return
					}
					log.Println("Error copying:", err)
					return
				}
				// With each chunk copied, we send a message on our blockTicker channel
				mc.blockTicker <- true

			}
		}
	case outbound:
		// Send and tally outgoing data as fast as we can until the receiver stops receiving or the timer expires
		for {
			select {
			case <-timer.C:
				// Timer has elapsed and test is finished
				close(measurerDone)
				return
			default:
				// Copy data from our RNG to the net.Conn in (blockSize) KB chunks
				_, err = io.CopyN(conn, rnd, 1024*blockSize)
				if err != nil {
					if err == io.EOF {
						close(measurerDone)
						return
					}
					log.Println("Error copying:", err)
					return
				}
				// With each chunk copied, we send a message on our blockTicker channel
				mc.blockTicker <- true
			}
		}
	}
}

// MeasureThroughput receives ticks sent by MeteredCopy() and derives a throughput rate, which is then sent
// to the throughput reporter.
func (mc *meteredClient) MeasureThroughput(measurerDone <-chan struct{}) {
	var dir = inbound
	var blockCount, prevBlockCount uint64
	var throughput float64
	var throughputHist []float64

	tick := time.NewTicker(time.Duration(reportIntervalMS) * time.Millisecond)
	for {
		select {
		case <-mc.blockTicker:
			// Increment our block counter when we get a ticker
			blockCount++
		case <-measurerDone:
			tick.Stop()
			return
		case <-mc.changeToUpload:
			// The download test has completed, so we switch to tallying upload chunks
			dir = outbound
		case <-tick.C:
			throughput = (float64(blockCount - prevBlockCount)) * float64(blockSize*8) / float64(reportIntervalMS)

			// We discard the first element of the throughputHist slice once we have 70
			// elements stored.  This gives the user a chart that appears to scroll to
			// the left as new measurements come in and old ones are discarded.
			if len(throughputHist) >= 70 {
				throughputHist = throughputHist[1:]
			}

			// Add our latest measurement to the slice of historical measurements
			throughputHist = append(throughputHist, throughput)

			// Update the appropriate graph with the latest measurements
			switch dir {
			case inbound:
				mc.wr.jobs["dlgraph"].(*termui.LineChart).Data = throughputHist
			case outbound:
				mc.wr.jobs["ulgraph"].(*termui.LineChart).Data = throughputHist
			}

			// Send the latest measurement on to the stats generator
			mc.throughputReport <- throughput

			// Update the current block counter
			prevBlockCount = blockCount
		}
	}
}

// generateStats receives download and upload speed reports and computes metrics
// which are displayed in the stats widget.
func (mc *meteredClient) generateStats() {
	var measurement float64
	var currentDL, maxDL, avgDL float64
	var currentUL, maxUL, avgUL float64
	var dlReadingCount, dlReadingSum float64
	var ulReadingCount, ulReadingSum float64
	var dir = inbound

	for {
		select {
		case measurement = <-mc.throughputReport:
			switch dir {
			case inbound:
				currentDL = measurement
				dlReadingCount++
				dlReadingSum = dlReadingSum + currentDL
				avgDL = dlReadingSum / dlReadingCount
				if currentDL > maxDL {
					maxDL = currentDL
				}
				// Update our stats widget with the latest readings
				mc.wr.jobs["statsSummary"].(*termui.Par).Text = fmt.Sprintf("DOWNLOAD \nCurrent: %v Mbps\tMax: %v\tAvg: %v\n\nUPLOAD\nCurrent: %v Mbps\tMax: %v\tAvg: %v",
					strconv.FormatFloat(currentDL, 'f', 1, 64), strconv.FormatFloat(maxDL, 'f', 1, 64), strconv.FormatFloat(avgDL, 'f', 1, 64),
					strconv.FormatFloat(currentUL, 'f', 1, 64), strconv.FormatFloat(maxUL, 'f', 1, 64), strconv.FormatFloat(avgUL, 'f', 1, 64))
				mc.wr.Render()
			case outbound:
				currentUL = measurement
				ulReadingCount++
				ulReadingSum = ulReadingSum + currentUL
				avgUL = ulReadingSum / ulReadingCount
				if currentUL > maxUL {
					maxUL = currentUL
				}
				// Update our stats widget with the latest readings
				mc.wr.jobs["statsSummary"].(*termui.Par).Text = fmt.Sprintf("DOWNLOAD \nCurrent: %v Mbps\tMax: %v\tAvg: %v\n\nUPLOAD\nCurrent: %v Mbps\tMax: %v\tAvg: %v",
					strconv.FormatFloat(currentDL, 'f', 1, 64), strconv.FormatFloat(maxDL, 'f', 1, 64), strconv.FormatFloat(avgDL, 'f', 1, 64),
					strconv.FormatFloat(currentUL, 'f', 1, 64), strconv.FormatFloat(maxUL, 'f', 1, 64), strconv.FormatFloat(avgUL, 'f', 1, 64))
				mc.wr.Render()

			}
		case <-mc.changeToUpload:
			dir = outbound
		case <-mc.statsGeneratorDone:
			return
		}
	}
}
