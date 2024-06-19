package soc

import (
	"bufio"
	"bytes"
	"github.com/sirupsen/logrus"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type SocInfo struct {
	Name         string
	CoreCount    string
	CpuMaxPower  string
	GpuMaxPower  string
	CpuMaxBw     string
	GpuMaxBw     string
	ECoreCount   int
	PCoreCount   int
	GpuCoreCount string
}

var socInfo *SocInfo

func GetSOCInfo() *SocInfo {
	// cache the result won't change after launch
	if socInfo != nil {
		return socInfo
	}

	sync.OnceFunc(func() {
		m := getSysCtlProperties("machdep.cpu", "hw.perflevel0.logicalcpu", "hw.perflevel1.logicalcpu")

		name := m["machdep.cpu.brand_string"]
		coreCount := m["machdep.cpu.core_count"]
		eCoreCount, err := strconv.Atoi(m["hw.perflevel1.logicalcpu"])
		if err != nil {
			logrus.Fatalf("failed to parse hw.perflevel1.logicalcpu, err: %v", err)
		}
		pCoreCount, err := strconv.Atoi(m["hw.perflevel0.logicalcpu"])
		if err != nil {
			logrus.Errorf("failed to parse hw.perflevel0.logicalcpu, err: %v", err)
		}

		socInfo = &SocInfo{
			Name:         name,
			CoreCount:    coreCount,
			CpuMaxPower:  "",
			GpuMaxPower:  "",
			CpuMaxBw:     "",
			GpuMaxBw:     "",
			ECoreCount:   eCoreCount,
			PCoreCount:   pCoreCount,
			GpuCoreCount: getGPUCores(),
		}
	})()

	return socInfo
}

func getSysCtlProperties(properties ...string) map[string]string {
	var rs = make(map[string]string)
	out, err := exec.Command("sysctl", properties...).Output()
	if err != nil {
		logrus.Fatalf("fail to execute getSysCtlProperties() sysctl command: %v", err)
	}

	buf := bytes.NewReader(out)
	scanner := bufio.NewScanner(buf)

	var text string
	for scanner.Scan() {
		text = scanner.Text()
		parts := strings.Split(text, ":")
		if len(parts) != 2 {
			logrus.Fatalf("fail to split properties: %s", text)
		}
		rs[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}

	return rs
}

func getGPUCores() string {
	cmd, err := exec.Command("system_profiler", "-detailLevel", "basic", "SPDisplaysDataType").Output()
	if err != nil {
		logrus.Fatalf("failed to execute system_profiler command: %v", err)
	}
	output := string(cmd)
	logrus.Printf("Output: %s\n", output)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Total Number of Cores") {
			parts := strings.Split(line, ": ")
			if len(parts) > 1 {
				cores := strings.TrimSpace(parts[1])
				return cores
			}
			break
		}
	}
	return "?"
}
