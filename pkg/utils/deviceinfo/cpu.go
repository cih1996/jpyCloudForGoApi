//go:build linux
// +build linux

package deviceinfo

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func CpuInfo() ([]InfoStat, error) {
	filename := GetEnv("HOST_PROC", "/proc", "cpuinfo")
	lines, _ := ReadLines(filename)

	var ret []InfoStat
	var processorName string

	c := InfoStat{CPU: -1, Cores: 1}
	for _, line := range lines {
		fields := strings.Split(line, ":")
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSpace(fields[0])
		value := strings.TrimSpace(fields[1])

		switch key {
		case "Processor":
			processorName = value
		case "processor":
			if c.CPU >= 0 {
				err := finishCPUInfo(&c)
				if err != nil {
					return ret, err
				}
				ret = append(ret, c)
			}
			c = InfoStat{Cores: 1, ModelName: processorName}
			t, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return ret, err
			}
			c.CPU = int32(t)
		case "vendorId", "vendor_id":
			c.VendorID = value
		case "CPU implementer":
			if v, err := strconv.ParseUint(value, 0, 8); err == nil {
				switch v {
				case 0x41:
					c.VendorID = "ARM"
				case 0x42:
					c.VendorID = "Broadcom"
				case 0x43:
					c.VendorID = "Cavium"
				case 0x44:
					c.VendorID = "DEC"
				case 0x46:
					c.VendorID = "Fujitsu"
				case 0x48:
					c.VendorID = "HiSilicon"
				case 0x49:
					c.VendorID = "Infineon"
				case 0x4d:
					c.VendorID = "Motorola/Freescale"
				case 0x4e:
					c.VendorID = "NVIDIA"
				case 0x50:
					c.VendorID = "APM"
				case 0x51:
					c.VendorID = "Qualcomm"
				case 0x56:
					c.VendorID = "Marvell"
				case 0x61:
					c.VendorID = "Apple"
				case 0x69:
					c.VendorID = "Intel"
				case 0xc0:
					c.VendorID = "Ampere"
				}
			}
		case "cpu family":
			c.Family = value
		case "model", "CPU part":
			c.Model = value
		case "model name", "cpu":
			c.ModelName = value
			if strings.Contains(value, "POWER8") ||
				strings.Contains(value, "POWER7") {
				c.Model = strings.Split(value, " ")[0]
				c.Family = "POWER"
				c.VendorID = "IBM"
			}
		case "stepping", "revision", "CPU revision":
			val := value

			if key == "revision" {
				val = strings.Split(value, ".")[0]
			}

			t, err := strconv.ParseInt(val, 10, 64)
			if err != nil {
				return ret, err
			}
			c.Stepping = int32(t)
		case "cpu MHz", "clock":
			// treat this as the fallback value, thus we ignore error
			if t, err := strconv.ParseFloat(strings.Replace(value, "MHz", "", 1), 64); err == nil {
				c.Mhz = t
			}
		case "cache size":
			t, err := strconv.ParseInt(strings.Replace(value, " KB", "", 1), 10, 64)
			if err != nil {
				return ret, err
			}
			c.CacheSize = int32(t)
		case "physical id":
			c.PhysicalID = value
		case "core id":
			c.CoreID = value
		case "flags", "Features":
			c.Flags = strings.FieldsFunc(value, func(r rune) bool {
				return r == ',' || r == ' '
			})
		case "microcode":
			c.Microcode = value
		}
	}
	if c.CPU >= 0 {
		err := finishCPUInfo(&c)
		if err != nil {
			return ret, err
		}
		ret = append(ret, c)
	}
	return ret, nil
}

func finishCPUInfo(c *InfoStat) error {
	var lines []string
	var err error
	var value float64

	if len(c.CoreID) == 0 {
		lines, err = ReadLines(sysCPUPath(c.CPU, "topology/core_id"))
		if err == nil {
			c.CoreID = lines[0]
		}
	}

	// override the value of c.Mhz with cpufreq/cpuinfo_max_freq regardless
	// of the value from /proc/cpuinfo because we want to report the maximum
	// clock-speed of the CPU for c.Mhz, matching the behaviour of Windows
	lines, err = ReadLines(sysCPUPath(c.CPU, "cpufreq/cpuinfo_max_freq"))
	// if we encounter errors below such as there are no cpuinfo_max_freq file,
	// we just ignore. so let Mhz is 0.
	if err != nil || len(lines) == 0 {
		return nil
	}
	value, err = strconv.ParseFloat(lines[0], 64)
	if err != nil {
		return nil
	}
	c.Mhz = value / 1000.0 // value is in kHz
	if c.Mhz > 9999 {
		c.Mhz = c.Mhz / 1000.0 // value in Hz
	}
	return nil
}
func sysCPUPath(cpu int32, relPath string) string {
	return GetEnv("HOST_SYS", "/sys", fmt.Sprintf("devices/system/cpu/cpu%d", cpu), relPath)
}

// ReadLines reads contents from a file and splits them by new lines.
// A convenience wrapper to ReadLinesOffsetN(filename, 0, -1).
func ReadLines(filename string) ([]string, error) {
	return ReadLinesOffsetN(filename, 0, -1)
}

// ReadLinesOffsetN reads contents from file and splits them by new line.
// The offset tells at which line number to start.
// The count determines the number of lines to read (starting from offset):
//
//	n >= 0: at most n lines
//	n < 0: whole file
func ReadLinesOffsetN(filename string, offset uint, n int) ([]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return []string{""}, err
	}
	defer f.Close()

	var ret []string

	r := bufio.NewReader(f)
	for i := 0; i < n+int(offset) || n < 0; i++ {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		if i < int(offset) {
			continue
		}
		ret = append(ret, strings.Trim(line, "\n"))
	}

	return ret, nil
}
