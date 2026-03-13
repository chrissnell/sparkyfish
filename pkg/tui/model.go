package tui

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/canvas/runes"
	"github.com/NimbleMarkets/ntcharts/linechart/streamlinechart"
	"github.com/NimbleMarkets/ntcharts/sparkline"
	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/chrissnell/sparkyfish/pkg/backend"
	"github.com/chrissnell/sparkyfish/pkg/measure"
)

const (
	minWidth  = 60
	minHeight = 24

	// Expected sample counts per phase, used to scale charts to full width.
	expectedPingSamples = 30 // numPings
	expectedDlSamples  = 24 // (10s + 2s grace) / 500ms
	expectedUlSamples  = 20 // 10s / 500ms
)

type phase int

const (
	phaseConnecting phase = iota
	phasePing
	phaseDownload
	phaseUpload
	phaseDone
	phaseError
)

// Internal messages carrying channel references for the pull loop.
type (
	serverInfoMsg  backend.ServerInfo
	testErrorMsg   struct{ err error }
	pingDoneMsg    struct{}
	dlDoneMsg      struct{}
	ulDoneMsg      struct{}
	pingSampleMsg  struct {
		sample backend.PingSample
		ch     <-chan backend.PingSample
	}
	dlSampleMsg struct {
		sample backend.ThroughputSample
		ch     <-chan backend.ThroughputSample
	}
	ulSampleMsg struct {
		sample backend.ThroughputSample
		ch     <-chan backend.ThroughputSample
	}
)

type Model struct {
	backend backend.Backend
	addr    string
	ctx     context.Context
	cancel  context.CancelFunc

	phase  phase
	width  int
	height int

	serverInfo backend.ServerInfo

	// Latency data
	pings     []time.Duration
	pingMin   time.Duration
	pingMax   time.Duration
	pingMean  time.Duration
	pingStdev time.Duration

	// Throughput data
	dlSamples []float64
	ulSamples []float64
	dlCur     float64
	dlMax     float64
	dlAvg     float64
	ulCur     float64
	ulMax     float64
	ulAvg     float64

	// Chart sub-models
	dlChart      streamlinechart.Model
	ulChart      streamlinechart.Model
	latencyChart sparkline.Model

	// Columns pushed so far per chart, for proportional fill
	dlColsPushed  int
	ulColsPushed  int
	latColsPushed int

	err error
}

func New(b backend.Backend, addr string) Model {
	ctx, cancel := context.WithCancel(context.Background())

	chartStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // green
	latStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("14"))   // cyan

	dlChart := streamlinechart.New(26, 10,
		streamlinechart.WithStyles(runes.ArcLineStyle, chartStyle),
		streamlinechart.WithYRange(0, 100),
	)
	ulChart := streamlinechart.New(26, 10,
		streamlinechart.WithStyles(runes.ArcLineStyle, chartStyle),
		streamlinechart.WithYRange(0, 100),
	)
	latencyChart := sparkline.New(28, 3,
		sparkline.WithStyle(latStyle),
	)

	return Model{
		backend:      b,
		addr:         addr,
		ctx:          ctx,
		cancel:       cancel,
		phase:        phaseConnecting,
		dlChart:      dlChart,
		ulChart:      ulChart,
		latencyChart: latencyChart,
	}
}

func (m Model) Init() tea.Cmd {
	return m.connectCmd()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeCharts()
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q", "ctrl+c":
			m.cancel()
			return m, tea.Quit
		case "r", "R":
			if m.phase == phaseDone || m.phase == phaseError {
				return m.resetTest()
			}
		}

	case serverInfoMsg:
		m.serverInfo = backend.ServerInfo(msg)
		m.phase = phasePing
		return m, m.startPingCmd()

	case pingSampleMsg:
		m.addPingSample(msg.sample)
		return m, waitForPing(msg.ch)

	case pingDoneMsg:
		m.phase = phaseDownload
		return m, m.startDownloadCmd()

	case dlSampleMsg:
		m.addDlSample(msg.sample)
		return m, waitForThroughput(msg.ch, false)

	case dlDoneMsg:
		m.phase = phaseUpload
		return m, m.startUploadCmd()

	case ulSampleMsg:
		m.addUlSample(msg.sample)
		return m, waitForThroughput(msg.ch, true)

	case ulDoneMsg:
		m.phase = phaseDone
		return m, nil

	case testErrorMsg:
		m.err = msg.err
		m.phase = phaseError
		return m, nil
	}

	return m, nil
}

func (m Model) View() string {
	if m.width < minWidth || m.height < minHeight {
		msg := fmt.Sprintf("Terminal too small (%dx%d). Need at least %dx%d.",
			m.width, m.height, minWidth, minHeight)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
	}

	if m.phase == phaseError {
		return fmt.Sprintf("Error: %v\n\nPress q to quit.", m.err)
	}

	var sections []string

	// Title
	sections = append(sections, m.renderTitle())

	// Banner
	sections = append(sections, m.renderBanner())
	sections = append(sections, dividerStyle.Render(strings.Repeat("─", m.width)))

	// Latency row
	sections = append(sections, m.renderLatencyRow())
	sections = append(sections, dividerStyle.Render(strings.Repeat("─", m.width)))

	// Charts row
	sections = append(sections, m.renderChartsRow())

	// Throughput summary
	sections = append(sections, m.renderSummary())

	// Progress bar
	sections = append(sections, m.renderProgress())

	// Help bar
	sections = append(sections, m.renderHelp())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// --- Rendering helpers ---

func (m Model) renderTitle() string {
	title := "[ sparkyfish ]"
	padding := m.width - len(title) - 6
	if padding < 0 {
		padding = 0
	}
	line := strings.Repeat("─", 6) + title + strings.Repeat("─", padding)
	return titleStyle.Render(line)
}

func (m Model) renderBanner() string {
	if m.serverInfo.Hostname == "" {
		return bannerStyle.Render("Connecting...")
	}
	banner := m.serverInfo.Hostname
	if m.serverInfo.Location != "" {
		banner += " :: " + m.serverInfo.Location
	}
	if len(banner) > m.width {
		banner = banner[:m.width]
	}
	return bannerStyle.Render(banner)
}

func (m Model) renderLatencyRow() string {
	chartW := m.width / 2
	if chartW < 10 {
		chartW = 10
	}

	m.latencyChart.Draw()
	sparkView := latencyLabelStyle.Render("Latency") + "\n" + m.latencyChart.View()
	left := lipgloss.NewStyle().Width(chartW).Render(sparkView)

	var stats string
	if len(m.pings) > 0 {
		stats = fmt.Sprintf("Cur/Min/Max\n%.2f/%.2f/%.2f ms\nAvg/σ\n%.2f/%.2f ms",
			ms(m.pings[len(m.pings)-1]),
			ms(m.pingMin), ms(m.pingMax),
			ms(m.pingMean), ms(m.pingStdev))
	} else {
		stats = "Cur/Min/Max\n--/--/-- ms\nAvg/σ\n--/-- ms"
	}
	right := latencyStatsStyle.Render(stats)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) renderChartsRow() string {
	chartW := m.width/2 - 1
	if chartW < 10 {
		chartW = 10
	}
	chartH := m.chartHeight()

	_ = chartH // charts already sized via resizeCharts

	m.dlChart.Draw()
	m.ulChart.Draw()

	dlBox := chartBorderStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		chartLabelStyle.Render(" Download Speed (Mbit/s)"),
		m.dlChart.View(),
	))

	ulBox := chartBorderStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		chartLabelStyle.Render(" Upload Speed (Mbit/s)"),
		m.ulChart.View(),
	))

	return lipgloss.JoinHorizontal(lipgloss.Top, dlBox, " ", ulBox)
}

func (m Model) renderSummary() string {
	dl := fmt.Sprintf("Current: %.1f Mbit/s\tMax: %.1f\tAvg: %.1f", m.dlCur, m.dlMax, m.dlAvg)
	ul := fmt.Sprintf("Current: %.1f Mbit/s\tMax: %.1f\tAvg: %.1f", m.ulCur, m.ulMax, m.ulAvg)

	content := lipgloss.JoinVertical(lipgloss.Left,
		summaryHeaderStyle.Render("DOWNLOAD"),
		summaryValueStyle.Render(dl),
		"",
		summaryHeaderStyle.Render("UPLOAD"),
		summaryValueStyle.Render(ul),
	)

	border := lipgloss.RoundedBorder()
	box := lipgloss.NewStyle().
		Border(border).
		BorderForeground(lipgloss.Color("7")).
		Width(m.width - 2).
		Render(
			progressLabelStyle.Render(" Throughput Summary") + "\n" + content,
		)
	return box
}

func (m Model) renderProgress() string {
	pct := m.progressPct()
	barWidth := m.width - 4
	if barWidth < 10 {
		barWidth = 10
	}

	filled := int(float64(barWidth) * pct)
	if filled > barWidth {
		filled = barWidth
	}

	label := fmt.Sprintf(" %d%% ", int(pct*100))

	// Place label in center of bar
	labelPos := (barWidth - len(label)) / 2
	if labelPos < 0 {
		labelPos = 0
	}

	style := progressBarActive
	if m.phase == phaseDone {
		style = progressBarDone
	}

	var bar strings.Builder
	for i := 0; i < barWidth; i++ {
		ch := " "
		if i >= labelPos && i < labelPos+len(label) {
			ch = string(label[i-labelPos])
		}
		if i < filled {
			bar.WriteString(style.Render(ch))
		} else {
			bar.WriteString(progressBarEmpty.Render(ch))
		}
	}

	border := lipgloss.RoundedBorder()
	box := lipgloss.NewStyle().
		Border(border).
		BorderForeground(lipgloss.Color("7")).
		Width(m.width - 2).
		Render(progressLabelStyle.Render(" Test Progress") + "\n" + bar.String())
	return box
}

func (m Model) renderHelp() string {
	help := " COMMANDS: [q]uit"
	if m.phase == phaseDone || m.phase == phaseError {
		help += "  [r]etest"
	}
	padding := m.width - len(help)
	if padding < 0 {
		padding = 0
	}
	return helpStyle.Render(help + strings.Repeat(" ", padding))
}

// --- Data processing ---

func (m *Model) addPingSample(s backend.PingSample) {
	m.pings = append(m.pings, s.Latency)
	m.pingMin, m.pingMax, m.pingMean, m.pingStdev = measure.DurationStats(m.pings)

	v := ms(s.Latency)
	chartW := m.latencyChart.Width()
	targetCols := proportionalColumns(len(m.pings), expectedPingSamples, chartW)
	for m.latColsPushed < targetCols {
		m.latencyChart.Push(v)
		m.latColsPushed++
	}
}

func (m *Model) addDlSample(s backend.ThroughputSample) {
	m.dlSamples = append(m.dlSamples, s.Mbps)
	m.dlCur = s.Mbps
	if s.Mbps > m.dlMax {
		m.dlMax = s.Mbps
	}
	m.dlAvg = measure.Mean(m.dlSamples)

	chartW := m.dlChart.GraphWidth()
	targetCols := proportionalColumns(len(m.dlSamples), expectedDlSamples, chartW)
	for m.dlColsPushed < targetCols {
		m.dlChart.Push(s.Mbps)
		m.dlColsPushed++
	}
}

func (m *Model) addUlSample(s backend.ThroughputSample) {
	m.ulSamples = append(m.ulSamples, s.Mbps)
	m.ulCur = s.Mbps
	if s.Mbps > m.ulMax {
		m.ulMax = s.Mbps
	}
	m.ulAvg = measure.Mean(m.ulSamples)

	chartW := m.ulChart.GraphWidth()
	targetCols := proportionalColumns(len(m.ulSamples), expectedUlSamples, chartW)
	for m.ulColsPushed < targetCols {
		m.ulChart.Push(s.Mbps)
		m.ulColsPushed++
	}
}

// proportionalColumns returns how many chart columns should be filled
// after receiving sampleCount of expectedTotal samples, given chartWidth
// total columns. This ensures the chart is exactly full at the last sample.
func proportionalColumns(sampleCount, expectedTotal, chartWidth int) int {
	if expectedTotal <= 0 || chartWidth <= 0 {
		return sampleCount
	}
	return int(math.Round(float64(sampleCount) / float64(expectedTotal) * float64(chartWidth)))
}

func (m *Model) resizeCharts() {
	chartW := m.width/2 - 2
	if chartW < 10 {
		chartW = 10
	}
	chartH := m.chartHeight()
	latW := m.width/2 - 2
	if latW < 10 {
		latW = 10
	}

	m.dlChart.Resize(chartW, chartH)
	m.ulChart.Resize(chartW, chartH)
	m.latencyChart.Resize(latW, 3)

	// Re-push historical data with new proportions after resize
	m.repushChartData()
}

func (m *Model) repushChartData() {
	m.dlChart.ClearAllData()
	m.dlColsPushed = 0
	graphW := m.dlChart.GraphWidth()
	for i, v := range m.dlSamples {
		target := proportionalColumns(i+1, expectedDlSamples, graphW)
		for m.dlColsPushed < target {
			m.dlChart.Push(v)
			m.dlColsPushed++
		}
	}

	m.ulChart.ClearAllData()
	m.ulColsPushed = 0
	graphW = m.ulChart.GraphWidth()
	for i, v := range m.ulSamples {
		target := proportionalColumns(i+1, expectedUlSamples, graphW)
		for m.ulColsPushed < target {
			m.ulChart.Push(v)
			m.ulColsPushed++
		}
	}

	m.latencyChart.Clear()
	m.latColsPushed = 0
	latW := m.latencyChart.Width()
	for i, d := range m.pings {
		target := proportionalColumns(i+1, expectedPingSamples, latW)
		for m.latColsPushed < target {
			m.latencyChart.Push(ms(d))
			m.latColsPushed++
		}
	}
}

func (m Model) chartHeight() int {
	// Allocate available height to charts
	// Fixed rows: title(1) + banner(1) + spacing(2) + latency(5) + summary(~8) + progress(~4) + help(1) = ~22
	avail := m.height - 22
	if avail < 6 {
		avail = 6
	}
	if avail > 16 {
		avail = 16
	}
	return avail
}

func (m Model) progressPct() float64 {
	const numPings = 30
	const expectedSamples = 20 // 10s / 500ms

	switch m.phase {
	case phaseConnecting:
		return 0
	case phasePing:
		return float64(len(m.pings)) / float64(numPings) * (1.0 / 3.0)
	case phaseDownload:
		return (1.0 / 3.0) + float64(len(m.dlSamples))/float64(expectedSamples)*(1.0/3.0)
	case phaseUpload:
		return (2.0 / 3.0) + float64(len(m.ulSamples))/float64(expectedSamples)*(1.0/3.0)
	case phaseDone:
		return 1.0
	default:
		return 0
	}
}

func (m Model) resetTest() (tea.Model, tea.Cmd) {
	m.cancel()
	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx
	m.cancel = cancel

	m.phase = phaseConnecting
	m.err = nil
	m.serverInfo = backend.ServerInfo{}

	m.pings = nil
	m.pingMin = 0
	m.pingMax = 0
	m.pingMean = 0
	m.pingStdev = 0

	m.dlSamples = nil
	m.ulSamples = nil
	m.dlCur = 0
	m.dlMax = 0
	m.dlAvg = 0
	m.ulCur = 0
	m.ulMax = 0
	m.ulAvg = 0

	m.dlColsPushed = 0
	m.ulColsPushed = 0
	m.latColsPushed = 0

	m.dlChart.ClearAllData()
	m.ulChart.ClearAllData()
	m.latencyChart.Clear()

	return m, m.connectCmd()
}

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

// --- Async commands ---

func (m Model) connectCmd() tea.Cmd {
	return func() tea.Msg {
		info, err := m.backend.Connect(m.ctx, m.addr)
		if err != nil {
			return testErrorMsg{err: err}
		}
		return serverInfoMsg(info)
	}
}

func (m Model) startPingCmd() tea.Cmd {
	return func() tea.Msg {
		ch := make(chan backend.PingSample, 10)
		go func() {
			m.backend.Ping(m.ctx, ch)
		}()
		sample, ok := <-ch
		if !ok {
			return pingDoneMsg{}
		}
		return pingSampleMsg{sample: sample, ch: ch}
	}
}

func waitForPing(ch <-chan backend.PingSample) tea.Cmd {
	return func() tea.Msg {
		sample, ok := <-ch
		if !ok {
			return pingDoneMsg{}
		}
		return pingSampleMsg{sample: sample, ch: ch}
	}
}

func (m Model) startDownloadCmd() tea.Cmd {
	return func() tea.Msg {
		ch := make(chan backend.ThroughputSample, 10)
		go func() {
			m.backend.Download(m.ctx, ch)
		}()
		sample, ok := <-ch
		if !ok {
			return dlDoneMsg{}
		}
		return dlSampleMsg{sample: sample, ch: ch}
	}
}

func (m Model) startUploadCmd() tea.Cmd {
	return func() tea.Msg {
		ch := make(chan backend.ThroughputSample, 10)
		go func() {
			m.backend.Upload(m.ctx, ch)
		}()
		sample, ok := <-ch
		if !ok {
			return ulDoneMsg{}
		}
		return ulSampleMsg{sample: sample, ch: ch}
	}
}

func waitForThroughput(ch <-chan backend.ThroughputSample, isUpload bool) tea.Cmd {
	return func() tea.Msg {
		sample, ok := <-ch
		if !ok {
			if isUpload {
				return ulDoneMsg{}
			}
			return dlDoneMsg{}
		}
		if isUpload {
			return ulSampleMsg{sample: sample, ch: ch}
		}
		return dlSampleMsg{sample: sample, ch: ch}
	}
}
