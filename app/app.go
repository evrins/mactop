package app

import (
	"fmt"
	"github.com/context-labs/mactop/v2/parser"
	"github.com/context-labs/mactop/v2/soc"
	"github.com/context-labs/mactop/v2/ui"
	"os"
	"os/signal"
	"syscall"
)

func Start(updateInterval int, colorName string) {
	if os.Geteuid() != 0 {
		fmt.Println("Welcome to mactop! Please try again and run mactop with sudo privileges!")
		fmt.Println("Usage: sudo mactop")
		return
	}

	cpuMetricsChan := make(chan parser.CPUMetrics)
	gpuMetricsChan := make(chan parser.GPUMetrics)
	netDiskMetricsChan := make(chan parser.NetDiskMetrics)
	processMetricsChan := make(chan []parser.ProcessMetrics)

	done := make(chan struct{})
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	appleSiliconModel := soc.GetSOCInfo()
	go parser.CollectMetrics(done, cpuMetricsChan, gpuMetricsChan, netDiskMetricsChan, processMetricsChan, appleSiliconModel.Name, updateInterval)

	term := ui.NewUI(colorName,
		updateInterval,
		done,
		quit,
		cpuMetricsChan,
		gpuMetricsChan,
		netDiskMetricsChan,
		processMetricsChan,
	)

	term.Render()
}
