package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/dustin/randbo"
	"github.com/gizak/termui"
)

const (
	protocolVersion      uint16 = 0x00 // Protocol Version
	blockSize            int64  = 200  // size (KB) of each block of data copied to/from remote
	reportIntervalMS     uint64 = 500  // report interval in milliseconds
	throughputTestLength uint   = 10   // length of time to conduct each throughput test
	maxPingTestLength    uint   = 10   // maximum time for ping test to complete
	numPings             int    = 30   // number of pings to attempt
)

// command is used to indicate the type of test being performed
type command int

const (
	outbound command = iota // upload test
	inbound                 // download test
	echo                    // echo (ping) test
)

type sparkyClient struct {
	conn               net.Conn
	reader             *bufio.Reader
	randomData         []byte
	randReader         *bytes.Reader
	serverCname        string
	serverLocation     string
	serverHostname     string
	pingTime           chan time.Duration
	blockTicker        chan bool
	pingProgressTicker chan bool
	testDone           chan bool
	allTestsDone       chan struct{}
	progressBarReset   chan bool
	throughputReport   chan float64
	statsGeneratorDone chan struct{}
	changeToUpload     chan struct{}
	pingProcessorReady chan struct{}
	wr                 *widgetRenderer
	rendererMu         *sync.Mutex
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: ", os.Args[0], " <sparkyfish server hostname/IP>[:port]")
	}

	dest := os.Args[1]
	i := last(dest, ':')
	if i < 0 {
		dest = fmt.Sprint(dest, ":7121")
	}

	// Initialize our screen
	err := termui.Init()
	if err != nil {
		panic(err)
	}

	if termui.TermWidth() < 60 || termui.TermHeight() < 28 {
		fmt.Println("sparkyfish needs a terminal window at least 60x28 to run.")
		os.Exit(1)
	}

	defer termui.Close()

	// 'q' quits the program
	termui.Handle("/sys/kbd/q", func(termui.Event) {
		termui.StopLoop()
	})
	// 'Q' also works
	termui.Handle("/sys/kbd/Q", func(termui.Event) {
		termui.StopLoop()
	})

	sc := newsparkyClient()
	sc.serverHostname = dest

	sc.prepareChannels()

	sc.wr = newwidgetRenderer()

	// Begin our tests
	go sc.runTestSequence()

	termui.Loop()
}

// NewsparkyClient creates a new sparkyClient object
func newsparkyClient() *sparkyClient {
	m := sparkyClient{}

	// Make a 10MB byte slice to hold our random data blob
	m.randomData = make([]byte, 1024*1024*10)

	// Use a randbo Reader to fill our big slice with random data
	_, err := randbo.New().Read(m.randomData)
	if err != nil {
		log.Fatalln("error generating random data:", err)
	}

	// Create a bytes.Reader over this byte slice
	m.randReader = bytes.NewReader(m.randomData)

	return &m
}

func (sc *sparkyClient) prepareChannels() {

	// Prepare some channels that we'll use for measuring
	// throughput and latency
	sc.blockTicker = make(chan bool, 200)
	sc.throughputReport = make(chan float64)
	sc.pingTime = make(chan time.Duration, 10)
	sc.pingProgressTicker = make(chan bool, numPings)

	// Prepare some channels that we'll use to signal
	// various state changes in the testing process
	sc.pingProcessorReady = make(chan struct{})
	sc.changeToUpload = make(chan struct{})
	sc.statsGeneratorDone = make(chan struct{})
	sc.testDone = make(chan bool)
	sc.progressBarReset = make(chan bool)
	sc.allTestsDone = make(chan struct{})

}

func (sc *sparkyClient) runTestSequence() {
	// First, we need to build the widgets on our screen.

	// Build our title box
	titleBox := termui.NewPar("──────[ sparkyfish ]────────────────────────────────────────")
	titleBox.Height = 1
	titleBox.Width = 60
	titleBox.Y = 0
	titleBox.Border = false
	titleBox.TextFgColor = termui.ColorWhite | termui.AttrBold

	// Build the server name/location banner line
	bannerBox := termui.NewPar("")
	bannerBox.Height = 1
	bannerBox.Width = 60
	bannerBox.Y = 1
	bannerBox.Border = false
	bannerBox.TextFgColor = termui.ColorRed | termui.AttrBold

	// Build a download graph widget
	dlGraph := termui.NewLineChart()
	dlGraph.BorderLabel = " Download Speed (Mbit/s)"
	dlGraph.Data = []float64{0}
	dlGraph.Width = 30
	dlGraph.Height = 12
	dlGraph.PaddingTop = 1
	dlGraph.X = 0
	dlGraph.Y = 6
	// Windows Command Prompt doesn't support our Unicode characters with the default font
	if runtime.GOOS == "windows" {
		dlGraph.Mode = "dot"
		dlGraph.DotStyle = '+'
	}
	dlGraph.AxesColor = termui.ColorWhite
	dlGraph.LineColor = termui.ColorGreen | termui.AttrBold

	// Build an upload graph widget
	ulGraph := termui.NewLineChart()
	ulGraph.BorderLabel = " Upload Speed (Mbit/s)"
	ulGraph.Data = []float64{0}
	ulGraph.Width = 30
	ulGraph.Height = 12
	ulGraph.PaddingTop = 1
	ulGraph.X = 30
	ulGraph.Y = 6
	// Windows Command Prompt doesn't support our Unicode characters with the default font
	if runtime.GOOS == "windows" {
		ulGraph.Mode = "dot"
		ulGraph.DotStyle = '+'
	}
	ulGraph.AxesColor = termui.ColorWhite
	ulGraph.LineColor = termui.ColorGreen | termui.AttrBold

	latencyGraph := termui.NewSparkline()
	latencyGraph.LineColor = termui.ColorCyan
	latencyGraph.Height = 3

	latencyGroup := termui.NewSparklines(latencyGraph)
	latencyGroup.Y = 3
	latencyGroup.Height = 3
	latencyGroup.Width = 30
	latencyGroup.Border = false
	latencyGroup.Lines[0].Data = []int{0}

	latencyTitle := termui.NewPar("Latency")
	latencyTitle.Height = 1
	latencyTitle.Width = 30
	latencyTitle.Border = false
	latencyTitle.TextFgColor = termui.ColorGreen
	latencyTitle.Y = 2

	latencyStats := termui.NewPar("")
	latencyStats.Height = 4
	latencyStats.Width = 30
	latencyStats.X = 32
	latencyStats.Y = 2
	latencyStats.Border = false
	latencyStats.TextFgColor = termui.ColorWhite | termui.AttrBold
	latencyStats.Text = "Last: 30ms\nMin: 2ms\nMax: 34ms"

	// Build a stats summary widget
	statsSummary := termui.NewPar("")
	statsSummary.Height = 7
	statsSummary.Width = 60
	statsSummary.Y = 18
	statsSummary.BorderLabel = " Throughput Summary "
	statsSummary.Text = fmt.Sprintf("DOWNLOAD \nCurrent: -- Mbit/s\tMax: --\tAvg: --\n\nUPLOAD\nCurrent: -- Mbit/s\tMax: --\tAvg: --")
	statsSummary.TextFgColor = termui.ColorWhite | termui.AttrBold

	// Build out progress gauge widget
	progress := termui.NewGauge()
	progress.Percent = 40
	progress.Width = 60
	progress.Height = 3
	progress.Y = 25
	progress.X = 0
	progress.Border = true
	progress.BorderLabel = " Test Progress "
	progress.Percent = 0
	progress.BarColor = termui.ColorRed
	progress.BorderFg = termui.ColorWhite
	progress.PercentColorHighlighted = termui.ColorWhite | termui.AttrBold
	progress.PercentColor = termui.ColorWhite | termui.AttrBold

	// Build our helpbox widget
	helpBox := termui.NewPar(" COMMANDS: [q]uit")
	helpBox.Height = 1
	helpBox.Width = 60
	helpBox.Y = 28
	helpBox.Border = false
	helpBox.TextBgColor = termui.ColorBlue
	helpBox.TextFgColor = termui.ColorYellow | termui.AttrBold
	helpBox.Bg = termui.ColorBlue

	// Add the widgets to the rendering jobs and render the screen
	sc.wr.Add("titlebox", titleBox)
	sc.wr.Add("bannerbox", bannerBox)
	sc.wr.Add("dlgraph", dlGraph)
	sc.wr.Add("ulgraph", ulGraph)
	sc.wr.Add("latency", latencyGroup)
	sc.wr.Add("latencytitle", latencyTitle)
	sc.wr.Add("latencystats", latencyStats)
	sc.wr.Add("statsSummary", statsSummary)
	sc.wr.Add("progress", progress)
	sc.wr.Add("helpbox", helpBox)
	sc.wr.Render()

	// Launch a progress bar updater
	go sc.updateProgressBar()

	// Start our ping test and block until it's complete
	sc.pingTest()

	// Start our stats generator, which receives realtime measurements from the throughput
	// reporter and generates metrics from them
	go sc.generateStats()

	// Run our download tests and block until that's done
	sc.runThroughputTest(inbound)

	// Signal to our MeasureThroughput that we're about to begin the upload test
	close(sc.changeToUpload)

	// Run an outbound (upload) throughput test and block until it's complete
	sc.runThroughputTest(outbound)

	// Signal to our generators that the upload test is complete
	close(sc.statsGeneratorDone)

	// Notify the progress bar updater to change the bar color to green
	close(sc.allTestsDone)

	return
}

// updateProgressBar updates the progress bar as tests run
func (sc *sparkyClient) updateProgressBar() {
	var updateIntervalMS uint = 500
	var progress uint

	sc.wr.jobs["progress"].(*termui.Gauge).BarColor = termui.ColorRed

	//progressPerUpdate := throughputTestLength / (updateIntervalMS / 1000)
	var progressPerUpdate uint = 100 / 20

	// Set a ticker for advancing the progress bar
	tick := time.NewTicker(time.Duration(updateIntervalMS) * time.Millisecond)

	for {
		select {
		case <-tick.C:
			// Update via our update interval ticker, but never beyond 100%
			progress = progress + progressPerUpdate
			if progress > 100 {
				progress = 100
			}
			sc.wr.jobs["progress"].(*termui.Gauge).Percent = int(progress)
			sc.wr.Render()

		case <-sc.pingProgressTicker:
			// Update as each ping comes back, but never beyond 100%
			progress = progress + uint(100/numPings)
			if progress > 100 {
				progress = 100
			}
			sc.wr.jobs["progress"].(*termui.Gauge).Percent = int(progress)
			sc.wr.Render()

			// No need to render, since it's already happening with each ping
		case <-sc.testDone:
			// As each test completes, we set the progress bar to 100% completion.
			// It will be reset to 0% at the start of the next test.
			sc.wr.jobs["progress"].(*termui.Gauge).Percent = 100
			sc.wr.Render()
		case <-sc.progressBarReset:
			// Reset our progress tracker
			progress = 0
			// Reset the progress bar
			sc.wr.jobs["progress"].(*termui.Gauge).Percent = 0
			sc.wr.Render()
		case <-sc.allTestsDone:
			// Make sure that our progress bar always ends at 100%.  :)
			sc.wr.jobs["progress"].(*termui.Gauge).Percent = 100
			sc.wr.jobs["progress"].(*termui.Gauge).BarColor = termui.ColorGreen
			sc.wr.Render()
			return
		}
	}

}

func fatalError(err error) {
	termui.Clear()
	termui.Close()
	log.Fatal(err)
}

// Index of rightmost occurrence of b in s.
// Borrowed from golang.org/pkg/net/net.go
func last(s string, b byte) int {
	i := len(s)
	for i--; i >= 0; i-- {
		if s[i] == b {
			break
		}
	}
	return i
}
