// SPDX-License-Identifier: AGPL-3.0-only
package host

import (
	"fmt"
	"strconv"
	"strings"
)

type CPUCounters struct{ User, Nice, System, Idle, IOWait, IRQ, SoftIRQ, Steal uint64 }
type CPUUsage struct{ Busy, User, System, IOWait, Steal *float64 }

func ParseProcStat(input string) (map[string]CPUCounters, error) {
	out := map[string]CPUCounters{}
	for _, line := range strings.Split(input, "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || !strings.HasPrefix(f[0], "cpu") {
			continue
		}
		v := make([]uint64, len(f)-1)
		for i := range v {
			n, e := strconv.ParseUint(f[i+1], 10, 64)
			if e != nil {
				return nil, fmt.Errorf("%s: %w", f[0], e)
			}
			v[i] = n
		}
		c := CPUCounters{User: v[0], Nice: v[1], System: v[2], Idle: v[3]}
		if len(v) > 4 {
			c.IOWait = v[4]
		}
		if len(v) > 7 {
			c.Steal = v[7]
		}
		out[f[0]] = c
	}
	if _, ok := out["cpu"]; !ok {
		return nil, fmt.Errorf("missing cpu counters")
	}
	return out, nil
}
func CPUDelta(previous, current CPUCounters) CPUUsage {
	total := func(c CPUCounters) uint64 {
		return c.User + c.Nice + c.System + c.Idle + c.IOWait + c.IRQ + c.SoftIRQ + c.Steal
	}
	p, c := total(previous), total(current)
	if c <= p {
		return CPUUsage{}
	}
	d := float64(c - p)
	pct := func(now, old uint64) *float64 {
		if now < old {
			return nil
		}
		v := float64(now-old) * 100 / d
		return &v
	}
	busyNow := c - current.Idle - current.IOWait
	busyOld := p - previous.Idle - previous.IOWait
	return CPUUsage{pct(busyNow, busyOld), pct(current.User+current.Nice, previous.User+previous.Nice), pct(current.System, previous.System), pct(current.IOWait, previous.IOWait), pct(current.Steal, previous.Steal)}
}
