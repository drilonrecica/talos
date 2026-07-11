// SPDX-License-Identifier: AGPL-3.0-only
package host

import (
	"fmt"
	"strconv"
	"strings"
)

type Memory struct{ Total, Available, Used, SwapTotal, SwapFree, Cached, Buffers uint64 }

func ParseMeminfo(input string) (Memory, error) {
	vals := map[string]uint64{}
	for _, line := range strings.Split(input, "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		n, e := strconv.ParseUint(f[1], 10, 64)
		if e != nil {
			return Memory{}, fmt.Errorf("%s: %w", f[0], e)
		}
		vals[strings.TrimSuffix(f[0], ":")] = n * 1024
	}
	m := Memory{Total: vals["MemTotal"], Available: vals["MemAvailable"], SwapTotal: vals["SwapTotal"], SwapFree: vals["SwapFree"], Cached: vals["Cached"], Buffers: vals["Buffers"]}
	if m.Total == 0 {
		return m, fmt.Errorf("missing MemTotal")
	}
	if m.Available > m.Total {
		m.Available = m.Total
	}
	m.Used = m.Total - m.Available
	return m, nil
}
func ParseLoadavg(input string) (float64, error) {
	f := strings.Fields(input)
	if len(f) == 0 {
		return 0, fmt.Errorf("missing load")
	}
	return strconv.ParseFloat(f[0], 64)
}
func ParseUptime(input string) (float64, error) {
	f := strings.Fields(input)
	if len(f) == 0 {
		return 0, fmt.Errorf("missing uptime")
	}
	return strconv.ParseFloat(f[0], 64)
}
func BootIdentity(machineID string, uptimeSeconds float64) string {
	return fmt.Sprintf("%s:%d", strings.TrimSpace(machineID), int64(uptimeSeconds))
}
