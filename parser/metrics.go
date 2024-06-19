package parser

import (
	"bufio"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	dataRegex   = regexp.MustCompile(`(?m)^\s*(\S.*?)\s+(\d+)\s+(\d+\.\d+)\s+\d+\.\d+\s+`)
	outRegex    = regexp.MustCompile(`out:\s*([\d.]+)\s*packets/s,\s*([\d.]+)\s*bytes/s`)
	inRegex     = regexp.MustCompile(`in:\s*([\d.]+)\s*packets/s,\s*([\d.]+)\s*bytes/s`)
	readRegex   = regexp.MustCompile(`read:\s*([\d.]+)\s*ops/s\s*([\d.]+)\s*KBytes/s`)
	writeRegex  = regexp.MustCompile(`write:\s*([\d.]+)\s*ops/s\s*([\d.]+)\s*KBytes/s`)
	residencyRe = regexp.MustCompile(`(\w+-Cluster)\s+HW active residency:\s+(\d+\.\d+)%`)
	frequencyRe = regexp.MustCompile(`(\w+-Cluster)\s+HW active frequency:\s+(\d+)\s+MHz`)
	re          = regexp.MustCompile(`GPU\s*(HW)?\s*active\s*(residency|frequency):\s+(\d+\.\d+)%?`)
	freqRe      = regexp.MustCompile(`(\d+)\s*MHz:\s*(\d+)%`)
)

type CPUMetrics struct {
	EClusterActive, EClusterFreqMHz, PClusterActive, PClusterFreqMHz                                                                                                                                                 int
	ECores, PCores                                                                                                                                                                                                   []int
	ANEW, CPUW, GPUW, PackageW                                                                                                                                                                                       float64
	E0ClusterActive, E0ClusterFreqMHz, E1ClusterActive, E1ClusterFreqMHz, P0ClusterActive, P0ClusterFreqMHz, P1ClusterActive, P1ClusterFreqMHz, P2ClusterActive, P2ClusterFreqMHz, P3ClusterActive, P3ClusterFreqMHz int
}
type NetDiskMetrics struct {
	OutPacketsPerSec, OutBytesPerSec, InPacketsPerSec, InBytesPerSec, ReadOpsPerSec, WriteOpsPerSec, ReadKBytesPerSec, WriteKBytesPerSec float64
}

type GPUMetrics struct {
	FreqMHz int
	Active  float64
}

type ProcessMetrics struct {
	ID       int
	Name     string
	CPUUsage float64
}

type MemoryMetrics struct {
	Total, Used, Available, SwapTotal, SwapUsed uint64
}

func CollectMetrics(done chan struct{}, cpuMetricsChan chan CPUMetrics, gpuMetricsChan chan GPUMetrics, netDiskMetricsChan chan NetDiskMetrics, processMetricsChan chan []ProcessMetrics, modelName string, updateInterval int) {
	var cpuMetrics CPUMetrics
	var gpuMetrics GPUMetrics
	var netDiskMetrics NetDiskMetrics
	var processMetrics []ProcessMetrics
	cmd := exec.Command("powermetrics", "--samplers", "cpu_power,gpu_power,thermal,network,disk", "--show-process-gpu", "--show-process-energy", "--show-initial-usage", "--show-process-netstats", "-i", strconv.Itoa(updateInterval))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		logrus.Fatalf("failed to get stdout pipe: %v", err)
	}
	if err := cmd.Start(); err != nil {
		logrus.Fatalf("failed to start command: %v", err)
	}
	scanner := bufio.NewScanner(stdout)
	go func() {
		for {
			select {
			case <-done: // Check if we need to exit
				cmd.Process.Kill() // Ensure subprocess is terminated
				os.Exit(0)
				return
			default:
				if scanner.Scan() {
					line := scanner.Text()
					cpuMetrics = parseCPUMetrics(line, cpuMetrics, modelName)
					gpuMetrics = parseGPUMetrics(line, gpuMetrics)
					netDiskMetrics = parseActivityMetrics(line, netDiskMetrics)
					processMetrics = parseProcessMetrics(line, processMetrics)

					cpuMetricsChan <- cpuMetrics
					gpuMetricsChan <- gpuMetrics
					netDiskMetricsChan <- netDiskMetrics
					processMetricsChan <- processMetrics

				} else {
					if err := scanner.Err(); err != nil {
						logrus.Printf("error during scan: %v", err)
					}
					return // Exit loop if Scan() returns false
				}
			}
		}
	}()
	if err := cmd.Wait(); err != nil {
		logrus.Fatalf("command failed: %v", err)
	}
}

func parseProcessMetrics(powermetricsOutput string, processMetrics []ProcessMetrics) []ProcessMetrics {
	lines := strings.Split(powermetricsOutput, "\n")
	seen := make(map[int]bool) // Map to track seen process IDs
	for _, line := range lines {
		matches := dataRegex.FindStringSubmatch(line)
		if len(matches) > 3 {
			processName := matches[1]
			if processName == "mactop" || processName == "main" || processName == "powermetrics" {
				continue // Skip this process
			}
			id, _ := strconv.Atoi(matches[2])
			if !seen[id] {
				seen[id] = true
				cpuMsPerS, _ := strconv.ParseFloat(matches[3], 64)
				processMetrics = append(processMetrics, ProcessMetrics{
					Name:     matches[1],
					ID:       id,
					CPUUsage: cpuMsPerS,
				})
			}
		}
	}

	sort.Slice(processMetrics, func(i, j int) bool {
		return processMetrics[i].CPUUsage > processMetrics[j].CPUUsage
	})
	return processMetrics
}

func parseActivityMetrics(powermetricsOutput string, netdiskMetrics NetDiskMetrics) NetDiskMetrics {

	outMatches := outRegex.FindStringSubmatch(powermetricsOutput)
	inMatches := inRegex.FindStringSubmatch(powermetricsOutput)
	if len(outMatches) == 3 {
		netdiskMetrics.OutPacketsPerSec, _ = strconv.ParseFloat(outMatches[1], 64)
		netdiskMetrics.OutBytesPerSec, _ = strconv.ParseFloat(outMatches[2], 64)
	}
	if len(inMatches) == 3 {
		netdiskMetrics.InPacketsPerSec, _ = strconv.ParseFloat(inMatches[1], 64)
		netdiskMetrics.InBytesPerSec, _ = strconv.ParseFloat(inMatches[2], 64)
	}

	readMatches := readRegex.FindStringSubmatch(powermetricsOutput)
	writeMatches := writeRegex.FindStringSubmatch(powermetricsOutput)
	if len(readMatches) == 3 {
		netdiskMetrics.ReadOpsPerSec, _ = strconv.ParseFloat(readMatches[1], 64)
		netdiskMetrics.ReadKBytesPerSec, _ = strconv.ParseFloat(readMatches[2], 64)
	}
	if len(writeMatches) == 3 {
		netdiskMetrics.WriteOpsPerSec, _ = strconv.ParseFloat(writeMatches[1], 64)
		netdiskMetrics.WriteKBytesPerSec, _ = strconv.ParseFloat(writeMatches[2], 64)
	}
	return netdiskMetrics
}

func parseCPUMetrics(powermetricsOutput string, cpuMetrics CPUMetrics, modelName string) CPUMetrics {
	lines := strings.Split(powermetricsOutput, "\n")
	eCores := []int{}
	pCores := []int{}
	var eClusterActiveSum, pClusterActiveSum, eClusterFreqSum, pClusterFreqSum float64
	var eClusterCount, pClusterCount, eClusterActiveTotal, pClusterActiveTotal, eClusterFreqTotal, pClusterFreqTotal int

	if modelName == "Apple M3 Max" || modelName == "Apple M2 Max" { // For the M3/M2 Max, we need to manually parse the CPU Usage from the powermetrics output (as current bug in Apple's powermetrics)
		for _, line := range lines {

			maxCores := 15 // 16 Cores for M3 Max (4+12)
			if modelName == "Apple M2 Max" {
				maxCores = 11 // 12 Cores M2 Max (4+8)
			}
			for i := 0; i <= maxCores; i++ {
				re := regexp.MustCompile(`CPU ` + strconv.Itoa(i) + ` active residency:\s+(\d+\.\d+)%`)
				matches := re.FindStringSubmatch(powermetricsOutput)
				if len(matches) > 1 {
					activeResidency, _ := strconv.ParseFloat(matches[1], 64)
					if i <= 3 {
						eClusterActiveSum += activeResidency
						eClusterCount++
					} else {
						pClusterActiveSum += activeResidency
						pClusterCount++
					}
				}
			}
			for i := 0; i <= maxCores; i++ {
				fre := regexp.MustCompile(`^CPU\s+` + strconv.Itoa(i) + `\s+frequency:\s+(\d+)\s+MHz$`)
				matches := fre.FindStringSubmatch(powermetricsOutput)
				if len(matches) > 1 {
					activeFreq, _ := strconv.ParseFloat(matches[1], 64)
					if i <= 3 {
						eClusterFreqSum += activeFreq
						eClusterCount++
					} else {
						pClusterFreqSum += activeFreq
						pClusterCount++
					}
				}
			}

			if eClusterCount > 0 && eClusterActiveSum > 0.0 && eClusterActiveSum < 100.0 && eClusterActiveSum != 0 {
				cpuMetrics.EClusterActive = int(eClusterActiveSum / float64(eClusterCount))
			}
			if pClusterCount > 0 && pClusterActiveSum > 0.0 && pClusterActiveSum < 100.0 && pClusterActiveSum != 0 {
				cpuMetrics.PClusterActive = int(pClusterActiveSum / float64(pClusterCount))
			}
			if eClusterCount > 0 && eClusterFreqSum > 0.0 && eClusterFreqSum != 0 {
				cpuMetrics.EClusterFreqMHz = int(eClusterFreqSum / float64(eClusterCount))
			}
			if pClusterCount > 0 && pClusterFreqSum > 0.0 && pClusterFreqSum != 0 {
				cpuMetrics.PClusterFreqMHz = int(pClusterFreqSum / float64(pClusterCount))
			}

			if strings.Contains(line, "CPU ") && strings.Contains(line, "frequency") {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					core, _ := strconv.Atoi(strings.TrimPrefix(fields[1], "CPU"))
					if strings.Contains(line, "E-Cluster") {
						eCores = append(eCores, core)
					} else if strings.Contains(line, "P-Cluster") {
						pCores = append(pCores, core)
					}
				}
			} else if strings.Contains(line, "ANE Power") {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					cpuMetrics.ANEW, _ = strconv.ParseFloat(strings.TrimSuffix(fields[2], "mW"), 64)
					cpuMetrics.ANEW /= 1000 // Convert mW to W
				}
			} else if strings.Contains(line, "CPU Power") {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					cpuMetrics.CPUW, _ = strconv.ParseFloat(strings.TrimSuffix(fields[2], "mW"), 64)
					cpuMetrics.CPUW /= 1000 // Convert mW to W
				}
			} else if strings.Contains(line, "GPU Power") {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					cpuMetrics.GPUW, _ = strconv.ParseFloat(strings.TrimSuffix(fields[2], "mW"), 64)
					cpuMetrics.GPUW /= 1000 // Convert mW to W
				}
			} else if strings.Contains(line, "Combined Power (CPU + GPU + ANE)") {
				fields := strings.Fields(line)
				if len(fields) >= 8 {
					cpuMetrics.PackageW, _ = strconv.ParseFloat(strings.TrimSuffix(fields[7], "mW"), 64)
					cpuMetrics.PackageW /= 1000 // Convert mW to W
				}
			}
		}
		cpuMetrics.ECores = eCores
		cpuMetrics.PCores = pCores
	} else {
		for _, line := range lines {
			residencyMatches := residencyRe.FindStringSubmatch(line)
			frequencyMatches := frequencyRe.FindStringSubmatch(line)

			if residencyMatches != nil {
				cluster := residencyMatches[1]
				percent, _ := strconv.ParseFloat(residencyMatches[2], 64)
				switch cluster {
				case "E0-Cluster":
					cpuMetrics.E0ClusterActive = int(percent)
				case "E1-Cluster":
					cpuMetrics.E1ClusterActive = int(percent)
				case "P0-Cluster":
					cpuMetrics.P0ClusterActive = int(percent)
				case "P1-Cluster":
					cpuMetrics.P1ClusterActive = int(percent)
				case "P2-Cluster":
					cpuMetrics.P2ClusterActive = int(percent)
				case "P3-Cluster":
					cpuMetrics.P3ClusterActive = int(percent)
				}
				if strings.HasPrefix(cluster, "E") {
					eClusterActiveTotal += int(percent)
					eClusterCount++
				} else if strings.HasPrefix(cluster, "P") {
					pClusterActiveTotal += int(percent)
					pClusterCount++
					cpuMetrics.PClusterActive = pClusterActiveTotal / pClusterCount
				}
			}

			if frequencyMatches != nil {
				cluster := frequencyMatches[1]
				freqMHz, _ := strconv.Atoi(frequencyMatches[2])
				switch cluster {
				case "E0-Cluster":
					cpuMetrics.E0ClusterFreqMHz = freqMHz
				case "E1-Cluster":
					cpuMetrics.E1ClusterFreqMHz = freqMHz
				case "P0-Cluster":
					cpuMetrics.P0ClusterFreqMHz = freqMHz
				case "P1-Cluster":
					cpuMetrics.P1ClusterFreqMHz = freqMHz
				case "P2-Cluster":
					cpuMetrics.P2ClusterFreqMHz = freqMHz
				case "P3-Cluster":
					cpuMetrics.P3ClusterFreqMHz = freqMHz
				}
				if strings.HasPrefix(cluster, "E") {
					eClusterFreqTotal += int(freqMHz)
					cpuMetrics.EClusterFreqMHz = eClusterFreqTotal
				} else if strings.HasPrefix(cluster, "P") {
					pClusterFreqTotal += int(freqMHz)
					cpuMetrics.PClusterFreqMHz = pClusterFreqTotal
				}
			}

			if strings.Contains(line, "CPU ") && strings.Contains(line, "frequency") {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					core, _ := strconv.Atoi(strings.TrimPrefix(fields[1], "CPU"))
					if strings.Contains(line, "E-Cluster") {
						eCores = append(eCores, core)
					} else if strings.Contains(line, "P-Cluster") {
						pCores = append(pCores, core)
					}
				}
			} else if strings.Contains(line, "ANE Power") {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					cpuMetrics.ANEW, _ = strconv.ParseFloat(strings.TrimSuffix(fields[2], "mW"), 64)
					cpuMetrics.ANEW /= 1000 // Convert mW to W
				}
			} else if strings.Contains(line, "CPU Power") {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					cpuMetrics.CPUW, _ = strconv.ParseFloat(strings.TrimSuffix(fields[2], "mW"), 64)
					cpuMetrics.CPUW /= 1000 // Convert mW to W
				}
			} else if strings.Contains(line, "GPU Power") {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					cpuMetrics.GPUW, _ = strconv.ParseFloat(strings.TrimSuffix(fields[2], "mW"), 64)
					cpuMetrics.GPUW /= 1000 // Convert mW to W
				}
			} else if strings.Contains(line, "Combined Power (CPU + GPU + ANE)") {
				fields := strings.Fields(line)
				if len(fields) >= 8 {
					cpuMetrics.PackageW, _ = strconv.ParseFloat(strings.TrimSuffix(fields[7], "mW"), 64)
					cpuMetrics.PackageW /= 1000 // Convert mW to W
				}
			}
		}

		cpuMetrics.ECores = eCores
		cpuMetrics.PCores = pCores
		if cpuMetrics.E1ClusterActive != 0 {
			// M1 Ultra
			cpuMetrics.EClusterActive = (cpuMetrics.E0ClusterActive + cpuMetrics.E1ClusterActive) / 2
			cpuMetrics.EClusterFreqMHz = max(cpuMetrics.E0ClusterFreqMHz, cpuMetrics.E1ClusterFreqMHz)
		}
		if cpuMetrics.P3ClusterActive != 0 {
			// M1 Ultra
			cpuMetrics.PClusterActive = (cpuMetrics.P0ClusterActive + cpuMetrics.P1ClusterActive + cpuMetrics.P2ClusterActive + cpuMetrics.P3ClusterActive) / 4
			cpuMetrics.PClusterFreqMHz = max(cpuMetrics.P0ClusterFreqMHz, cpuMetrics.P1ClusterFreqMHz, cpuMetrics.P2ClusterFreqMHz, cpuMetrics.P3ClusterFreqMHz)
		} else if cpuMetrics.P1ClusterActive != 0 {
			// M1/M2/M3 Max/Pro
			cpuMetrics.PClusterActive = (cpuMetrics.P0ClusterActive + cpuMetrics.P1ClusterActive) / 2
			cpuMetrics.PClusterFreqMHz = max(cpuMetrics.P0ClusterFreqMHz, cpuMetrics.P1ClusterFreqMHz)
		} else {
			// M1
			cpuMetrics.PClusterActive = cpuMetrics.PClusterActive + cpuMetrics.P0ClusterActive
		}
		if eClusterCount > 0 { // Calculate average active residency and frequency for E and P clusters
			cpuMetrics.EClusterActive = eClusterActiveTotal / eClusterCount
		}
	}
	return cpuMetrics
}

func parseGPUMetrics(powermetricsOutput string, gpuMetrics GPUMetrics) GPUMetrics {

	lines := strings.Split(powermetricsOutput, "\n")

	for _, line := range lines {
		if strings.Contains(line, "GPU active") || strings.Contains(line, "GPU HW active") {
			matches := re.FindStringSubmatch(line)
			if len(matches) > 3 {
				if strings.Contains(matches[2], "residency") {
					gpuMetrics.Active, _ = strconv.ParseFloat(matches[3], 64)
				} else if strings.Contains(matches[2], "frequency") {
					gpuMetrics.FreqMHz, _ = strconv.Atoi(strings.TrimSuffix(matches[3], "MHz"))
				}
			}

			freqMatches := freqRe.FindAllStringSubmatch(line, -1)
			for _, match := range freqMatches {
				if len(match) == 3 {
					freq, _ := strconv.Atoi(match[1])
					residency, _ := strconv.ParseFloat(match[2], 64)
					if residency > 0 {
						gpuMetrics.FreqMHz = freq
						break
					}
				}
			}
		}
	}

	return gpuMetrics
}

func GetMemoryMetrics() MemoryMetrics {
	v, _ := mem.VirtualMemory()
	s, _ := mem.SwapMemory()

	totalMemory := v.Total
	usedMemory := v.Used
	availableMemory := v.Available
	swapTotal := s.Total
	swapUsed := s.Used

	return MemoryMetrics{
		Total:     totalMemory,
		Used:      usedMemory,
		Available: availableMemory,
		SwapTotal: swapTotal,
		SwapUsed:  swapUsed,
	}
}
