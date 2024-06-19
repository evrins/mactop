package ui

import (
	"fmt"
	"github.com/context-labs/mactop/v2/event_throttler"
	"github.com/context-labs/mactop/v2/parser"
	"github.com/context-labs/mactop/v2/soc"
	"github.com/gizak/termui/v3"
	"github.com/gizak/termui/v3/widgets"
	"github.com/sirupsen/logrus"
	"math"
	"os"
	"sort"
	"time"
)

type GridLayout int

const (
	DefaultGridLayout GridLayout = iota
	AlternativeGridLayout
)

type UI struct {
	socInfo           *soc.SocInfo
	colorName         string
	currentGridLayout GridLayout
	lastUpdateTime    time.Time
	updateInterval    int

	done chan struct{}
	quit <-chan os.Signal

	cpuMetricsChan     chan parser.CPUMetrics
	gpuMetricsChan     chan parser.GPUMetrics
	netDiskMetricsChan chan parser.NetDiskMetrics
	processMetricsChan chan []parser.ProcessMetrics

	grid                                            *termui.Grid
	cpu1Gauge, cpu2Gauge, gpuGauge, aneGauge        *widgets.Gauge
	TotalPowerChart                                 *widgets.BarChart
	memoryGauge                                     *widgets.Gauge
	modelText, PowerChart, NetworkInfo, ProcessInfo *widgets.Paragraph

	powerValues []float64
}

func NewUI(colorName string,
	updateInterval int,
	socInfo *soc.SocInfo,
	done chan struct{},
	quit <-chan os.Signal,
	cpuMetricsChan chan parser.CPUMetrics,
	gpuMetricsChan chan parser.GPUMetrics,
	netDiskMetricsChan chan parser.NetDiskMetrics,
	processMetricsChan chan []parser.ProcessMetrics,
) *UI {
	var ui = &UI{}
	ui.colorName = colorName
	ui.updateInterval = updateInterval

	ui.socInfo = socInfo

	ui.done = done
	ui.quit = quit

	ui.cpuMetricsChan = cpuMetricsChan
	ui.gpuMetricsChan = gpuMetricsChan
	ui.netDiskMetricsChan = netDiskMetricsChan
	ui.processMetricsChan = processMetricsChan

	return ui
}

func (ui *UI) setupGrid() {
	ui.grid = termui.NewGrid()
	ui.grid.Set(
		termui.NewRow(1.0/2, // This row now takes half the height of the grid
			termui.NewCol(1.0/2, termui.NewRow(1.0/2, ui.cpu1Gauge), termui.NewCol(1.0, termui.NewRow(1.0, ui.cpu2Gauge))),
			termui.NewCol(1.0/2, termui.NewRow(1.0/2, ui.gpuGauge), termui.NewCol(1.0, termui.NewRow(1.0, ui.aneGauge))), // termui.NewCol(1.0/2, termui.NewRow(1.0, ProcessInfo)), // ProcessInfo spans this entire column
		),
		termui.NewRow(1.0/4,
			termui.NewCol(1.0/6, ui.modelText),
			termui.NewCol(1.0/3, ui.NetworkInfo),
			termui.NewCol(1.0/4, ui.PowerChart),
			termui.NewCol(1.0/4, ui.TotalPowerChart),
		),
		termui.NewRow(1.0/4,
			termui.NewCol(1.0, ui.memoryGauge),
		),
	)
}

func (ui *UI) switchGridLayout() {
	if ui.currentGridLayout == DefaultGridLayout {
		newGrid := termui.NewGrid()
		newGrid.Set(
			termui.NewRow(1.0/2, // This row now takes half the height of the grid
				termui.NewCol(1.0/2, termui.NewRow(1.0, ui.cpu1Gauge)), // termui.NewCol(1.0, termui.NewRow(1.0, cpu2Gauge))),
				termui.NewCol(1.0/2, termui.NewRow(1.0, ui.cpu2Gauge)), // ProcessInfo spans this entire column
			),
			termui.NewRow(1.0/4,
				termui.NewCol(1.0/4, ui.gpuGauge),
				termui.NewCol(1.0/4, ui.aneGauge),
				termui.NewCol(1.0/4, ui.PowerChart),
				termui.NewCol(1.0/4, ui.TotalPowerChart),
			),
			termui.NewRow(1.0/4,
				termui.NewCol(3.0/6, ui.memoryGauge),
				termui.NewCol(1.0/6, ui.modelText),
				termui.NewCol(2.0/6, ui.NetworkInfo),
			),
		)
		termWidth, termHeight := termui.TerminalDimensions()
		newGrid.SetRect(0, 0, termWidth, termHeight)
		ui.grid = newGrid
		ui.currentGridLayout = AlternativeGridLayout
	} else {
		newGrid := termui.NewGrid()
		newGrid.Set(
			termui.NewRow(1.0/2,
				termui.NewCol(1.0/2, termui.NewRow(1.0/2, ui.cpu1Gauge), termui.NewCol(1.0, termui.NewRow(1.0, ui.cpu2Gauge))),
				termui.NewCol(1.0/2, termui.NewRow(1.0/2, ui.gpuGauge), termui.NewCol(1.0, termui.NewRow(1.0, ui.aneGauge))),
			),
			termui.NewRow(1.0/4,
				termui.NewCol(1.0/4, ui.modelText),
				termui.NewCol(1.0/4, ui.NetworkInfo),
				termui.NewCol(1.0/4, ui.PowerChart),
				termui.NewCol(1.0/4, ui.TotalPowerChart),
			),
			termui.NewRow(1.0/4,
				termui.NewCol(1.0, ui.memoryGauge),
			),
		)
		termWidth, termHeight := termui.TerminalDimensions()
		newGrid.SetRect(0, 0, termWidth, termHeight)
		ui.grid = newGrid
		ui.currentGridLayout = DefaultGridLayout
	}
}

func (ui *UI) setupWidgets() {
	appleSiliconModel := ui.socInfo
	ui.modelText = widgets.NewParagraph()
	ui.modelText.Title = "Apple Silicon"
	modelName := appleSiliconModel.Name
	if modelName == "" {
		modelName = "Unknown Model"
	}
	eCoreCount := appleSiliconModel.ECoreCount
	pCoreCount := appleSiliconModel.PCoreCount
	gpuCoreCount := appleSiliconModel.GpuCoreCount
	if gpuCoreCount == "" {
		gpuCoreCount = "?"
	}
	ui.modelText.Text = fmt.Sprintf("%s\nTotal Cores: %d\nE-Cores: %d\nP-Cores: %d\nGPU Cores: %s",
		modelName,
		eCoreCount+pCoreCount,
		eCoreCount,
		pCoreCount,
		gpuCoreCount,
	)
	logrus.Printf("Model: %s\nE-Core Count: %d\nP-Core Count: %d\nGPU Core Count: %s",
		modelName,
		eCoreCount,
		pCoreCount,
		gpuCoreCount,
	)

	ui.cpu1Gauge = widgets.NewGauge()
	ui.cpu1Gauge.Title = "E-CPU Usage"
	ui.cpu1Gauge.Percent = 0
	ui.cpu1Gauge.BarColor = termui.ColorGreen

	ui.cpu2Gauge = widgets.NewGauge()
	ui.cpu2Gauge.Title = "P-CPU Usage"
	ui.cpu2Gauge.Percent = 0
	ui.cpu2Gauge.BarColor = termui.ColorYellow

	ui.gpuGauge = widgets.NewGauge()
	ui.gpuGauge.Title = "GPU Usage"
	ui.gpuGauge.Percent = 0
	ui.gpuGauge.BarColor = termui.ColorMagenta

	ui.aneGauge = widgets.NewGauge()
	ui.aneGauge.Title = "ANE"
	ui.aneGauge.Percent = 0
	ui.aneGauge.BarColor = termui.ColorBlue

	ui.PowerChart = widgets.NewParagraph()
	ui.PowerChart.Title = "Power Usage"

	ui.NetworkInfo = widgets.NewParagraph()
	ui.NetworkInfo.Title = "Network & Disk Info"

	ui.ProcessInfo = widgets.NewParagraph()
	ui.ProcessInfo.Title = "Process Info"

	ui.TotalPowerChart = widgets.NewBarChart()
	ui.TotalPowerChart.Title = "~ W Total Power"
	ui.TotalPowerChart.SetRect(50, 0, 75, 10)
	ui.TotalPowerChart.BarWidth = 5 // Adjust the bar width to fill the available space
	ui.TotalPowerChart.BarGap = 1   // Remove the gap between the bars
	ui.TotalPowerChart.PaddingBottom = 0
	ui.TotalPowerChart.PaddingTop = 1
	ui.TotalPowerChart.NumFormatter = func(num float64) string {
		return ""
	}
	ui.memoryGauge = widgets.NewGauge()
	ui.memoryGauge.Title = "Memory Usage"
	ui.memoryGauge.Percent = 0
	ui.memoryGauge.BarColor = termui.ColorCyan
}

func (ui *UI) updateCPUUI(cpuMetrics parser.CPUMetrics) {
	ui.cpu1Gauge.Title = fmt.Sprintf("E-CPU Usage: %d%% @ %d MHz", cpuMetrics.EClusterActive, cpuMetrics.EClusterFreqMHz)
	ui.cpu1Gauge.Percent = cpuMetrics.EClusterActive

	ui.cpu2Gauge.Title = fmt.Sprintf("P-CPU Usage: %d%% @ %d MHz", cpuMetrics.PClusterActive, cpuMetrics.PClusterFreqMHz)
	ui.cpu2Gauge.Percent = cpuMetrics.PClusterActive

	aneUtil := int(cpuMetrics.ANEW * 100 / 8.0)

	ui.aneGauge.Title = fmt.Sprintf("ANE Usage: %d%% @ %.1f W", aneUtil, cpuMetrics.ANEW)
	ui.aneGauge.Percent = aneUtil

	ui.TotalPowerChart.Title = fmt.Sprintf("%.1f W Total Power", cpuMetrics.PackageW)

	ui.PowerChart.Title = fmt.Sprintf("%.1f W CPU - %.1f W GPU", cpuMetrics.CPUW, cpuMetrics.GPUW)
	ui.PowerChart.Text = fmt.Sprintf("CPU Power: %.1f W\nGPU Power: %.1f W\nANE Power: %.1f W\nTotal Power: %.1f W", cpuMetrics.CPUW, cpuMetrics.GPUW, cpuMetrics.ANEW, cpuMetrics.PackageW)

	memoryMetrics := parser.GetMemoryMetrics()

	ui.memoryGauge.Title = fmt.Sprintf("Memory Usage: %.2f GB / %.2f GB (Swap: %.2f/%.2f GB)", float64(memoryMetrics.Used)/1024/1024/1024, float64(memoryMetrics.Total)/1024/1024/1024, float64(memoryMetrics.SwapUsed)/1024/1024/1024, float64(memoryMetrics.SwapTotal)/1024/1024/1024)
	ui.memoryGauge.Percent = int((float64(memoryMetrics.Used) / float64(memoryMetrics.Total)) * 100)
}

func (ui *UI) updateGPUUI(gpuMetrics parser.GPUMetrics) {
	ui.gpuGauge.Title = fmt.Sprintf("GPU Usage: %d%% @ %d MHz", int(gpuMetrics.Active), gpuMetrics.FreqMHz)
	ui.gpuGauge.Percent = int(gpuMetrics.Active)
}

func (ui *UI) updateNetDiskUI(netdiskMetrics parser.NetDiskMetrics) {
	ui.NetworkInfo.Text = fmt.Sprintf("Out: %.1f packets/s, %.1f bytes/s\nIn: %.1f packets/s, %.1f bytes/s\nRead: %.1f ops/s, %.1f KBytes/s\nWrite: %.1f ops/s, %.1f KBytes/s", netdiskMetrics.OutPacketsPerSec, netdiskMetrics.OutBytesPerSec, netdiskMetrics.InPacketsPerSec, netdiskMetrics.InBytesPerSec, netdiskMetrics.ReadOpsPerSec, netdiskMetrics.ReadKBytesPerSec, netdiskMetrics.WriteOpsPerSec, netdiskMetrics.WriteKBytesPerSec)
}

func (ui *UI) updateProcessUI(processMetrics []parser.ProcessMetrics) {
	ui.ProcessInfo.Text = ""
	sort.Slice(processMetrics, func(i, j int) bool {
		return processMetrics[i].CPUUsage > processMetrics[j].CPUUsage
	})
	maxEntries := 15
	if len(processMetrics) > maxEntries {
		processMetrics = processMetrics[:maxEntries]
	}
	for _, pm := range processMetrics {
		ui.ProcessInfo.Text += fmt.Sprintf("%d - %s: %.2f ms/s\n", pm.ID, pm.Name, pm.CPUUsage)
	}
}

func (ui *UI) updateTotalPowerChart(newPowerValue float64) {
	currentTime := time.Now()
	ui.powerValues = append(ui.powerValues, newPowerValue)
	if currentTime.Sub(ui.lastUpdateTime) >= 2*time.Second {
		var sum float64
		for _, value := range ui.powerValues {
			sum += value
		}
		averagePower := sum / float64(len(ui.powerValues))
		averagePower = math.Round(averagePower)
		ui.TotalPowerChart.Data = append([]float64{averagePower}, ui.TotalPowerChart.Data...)
		if len(ui.TotalPowerChart.Data) > 25 {
			ui.TotalPowerChart.Data = ui.TotalPowerChart.Data[:25]
		}
		ui.powerValues = nil
		ui.lastUpdateTime = currentTime
	}
}

func (ui *UI) Render() {
	var err = termui.Init()
	if err != nil {
		logrus.Fatalf("failed to initialize termui: %v", err)
	}

	defer termui.Close()

	if ui.colorName != "" {
		var color termui.Color
		switch ui.colorName {
		case "green":
			color = termui.ColorGreen
		case "red":
			color = termui.ColorRed
		case "blue":
			color = termui.ColorBlue
		case "cyan":
			color = termui.ColorCyan
		case "magenta":
			color = termui.ColorMagenta
		case "yellow":
			color = termui.ColorYellow
		case "white":
			color = termui.ColorWhite
		default:
			logrus.Printf("Unsupported color: %s. Using default color.\n", ui.colorName)
			color = termui.ColorWhite
		}
		termui.Theme.Block.Title.Fg = color
		termui.Theme.Block.Border.Fg = color

		termui.Theme.Paragraph.Text.Fg = color

		termui.Theme.BarChart.Bars = []termui.Color{color}

		termui.Theme.Gauge.Label.Fg = color
		termui.Theme.Gauge.Bar = color
		ui.setupWidgets()
		ui.cpu1Gauge.BarColor = color
		ui.cpu2Gauge.BarColor = color
		ui.aneGauge.BarColor = color
		ui.gpuGauge.BarColor = color
		ui.memoryGauge.BarColor = color
	} else {
		ui.setupWidgets()
	}

	ui.setupGrid()

	termWidth, termHeight := termui.TerminalDimensions()
	ui.grid.SetRect(0, 0, termWidth, termHeight)
	termui.Render(ui.grid)

	needRender := event_throttler.NewEventThrottler(time.Duration(ui.updateInterval/2) * time.Millisecond)

	go func() {
		for {
			select {
			case cpuMetrics := <-ui.cpuMetricsChan:
				ui.updateCPUUI(cpuMetrics)
				ui.updateTotalPowerChart(cpuMetrics.PackageW)
				needRender.Notify()
			case gpuMetrics := <-ui.gpuMetricsChan:
				ui.updateGPUUI(gpuMetrics)
				needRender.Notify()
			case netdiskMetrics := <-ui.netDiskMetricsChan:
				ui.updateNetDiskUI(netdiskMetrics)
				needRender.Notify()
			case processMetrics := <-ui.processMetricsChan:
				ui.updateProcessUI(processMetrics)
				needRender.Notify()
			case <-needRender.C:
				termui.Render(ui.grid)
			case <-ui.quit:
				close(ui.done)
				termui.Close()
				os.Exit(0)
				return
			}
		}
	}()

	uiEvents := termui.PollEvents()

	for {
		select {
		case e := <-uiEvents:
			switch e.ID {
			case "q", "<C-c>": // "q" or Ctrl+C to quit
				close(ui.done)
				termui.Close()
				os.Exit(0)
				return
			case "<Resize>":
				payload := e.Payload.(termui.Resize)
				ui.grid.SetRect(0, 0, payload.Width, payload.Height)
				termui.Render(ui.grid)
			case "r":
				// refresh termui data
				termWidth, termHeight := termui.TerminalDimensions()
				ui.grid.SetRect(0, 0, termWidth, termHeight)
				termui.Clear()
				termui.Render(ui.grid)
			case "l":
				// Set the new grid's dimensions to match the terminal size
				termWidth, termHeight := termui.TerminalDimensions()
				ui.grid.SetRect(0, 0, termWidth, termHeight)
				termui.Clear()
				ui.switchGridLayout()
				termui.Render(ui.grid)
			}
		case <-ui.done:
			termui.Close()
			os.Exit(0)
			return
		}
	}
}
