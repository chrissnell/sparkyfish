package main

import (
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"sort"
	"time"

	"github.com/gizak/termui"
)

type pingHistory []int64

func (mc *meteredClient) pingTest() {
	// Reset our progress bar to 0% if it's not there already
	mc.progressBarReset <- true

	// start our ping processor
	go mc.pingProcessor()

	// Wait for our processor to become ready
	<-mc.pingProcessorReady

	buf := make([]byte, 1)
	conn, err := net.Dial("tcp", os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	// Send the ECO command to the remote server, requesting an echo test
	// (remote receives and echoes back).
	_, err = conn.Write([]byte("ECO"))
	if err != nil {
		termui.Close()
		log.Fatalln(err)
	}

	for c := 0; c <= numPings-1; c++ {
		startTime := time.Now()
		conn.Write([]byte{46})

		_, err = conn.Read(buf)
		if err != nil {
			log.Fatal(err)
		}
		endTime := time.Now()

		mc.pingTime <- endTime.Sub(startTime)
	}

	// Kill off the progress bar updater and block until it's gone
	mc.testDone <- true

	return
}

// pingProcessor recieves the ping times from pingTest and updates the UI
func (mc *meteredClient) pingProcessor() {
	var pingCount int
	var ptMax, ptMin int
	var latencyHist pingHistory

	// We never want to run the ping test beyond maxPingTestLength seconds
	timeout := time.NewTimer(time.Duration(maxPingTestLength) * time.Second)

	// Signal pingTest() that we're ready
	close(mc.pingProcessorReady)

	for {
		select {
		case <-timeout.C:
			// If we've been pinging for maxPingTestLength, call it quits
			return
		case pt := <-mc.pingTime:
			pingCount++

			// Calculate our ping time in microseconds
			ptMicro := pt.Nanoseconds() / 1000

			// Add this ping to our ping history
			latencyHist = append(latencyHist, ptMicro)

			ptMin, ptMax = latencyHist.minMax()

			// Advance the progress bar a bit
			mc.pingProgressTicker <- true

			// Update the ping stats widget
			mc.wr.jobs["latency"].(*termui.Sparklines).Lines[0].Data = latencyHist.toMilli()
			mc.wr.jobs["latencystats"].(*termui.Par).Text = fmt.Sprintf("Cur/Min/Max\n%.2f/%.2f/%.2f ms\nAvg/σ\n%.2f/%.2f ms",
				float64(ptMicro/1000), float64(ptMin/1000), float64(ptMax/1000), latencyHist.mean()/1000, latencyHist.stdDev()/1000)
			mc.wr.Render()
		}
	}
}

// toMilli Converts our ping history to milliseconds for display purposes
func (h *pingHistory) toMilli() []int {
	var pingMilli []int

	for _, v := range *h {
		pingMilli = append(pingMilli, int(v/1000))
	}

	return pingMilli
}

// mean generates a statistical mean of our historical ping times
func (h *pingHistory) mean() float64 {
	var sum uint64
	for _, t := range *h {
		sum = sum + uint64(t)
	}

	return float64(sum / uint64(len(*h)))
}

// variance calculates the variance of our historical ping times
func (h *pingHistory) variance() float64 {
	var sqDevSum float64

	mean := h.mean()

	for _, t := range *h {
		sqDevSum = sqDevSum + math.Pow((float64(t)-mean), 2)
	}
	return sqDevSum / float64(len(*h))
}

// stdDev calculates the standard deviation of our historical ping times
func (h *pingHistory) stdDev() float64 {
	return math.Sqrt(h.variance())
}

func (h *pingHistory) minMax() (int, int) {
	var hist []int
	for _, v := range *h {
		hist = append(hist, int(v))
	}
	sort.Ints(hist)
	return hist[0], hist[len(hist)-1]
}
