// SPDX-License-Identifier: AGPL-3.0-only
package docker

type MemoryStats struct {
	Usage, Limit, InactiveFile uint64
	PIDs                       *uint64
}
type MemoryUsage struct {
	Raw, WorkingSet, Limit, Percent *float64
	PIDs                            *uint64
}

func NormalizeMemory(s MemoryStats, hostTotal uint64) MemoryUsage {
	raw := float64(s.Usage)
	working := float64(s.Usage)
	if s.InactiveFile <= s.Usage {
		working = float64(s.Usage - s.InactiveFile)
	}
	denom := s.Limit
	if denom == 0 || denom > hostTotal && hostTotal > 0 {
		denom = hostTotal
	}
	var pct *float64
	if denom > 0 {
		v := working * 100 / float64(denom)
		pct = &v
	}
	return MemoryUsage{&raw, &working, number(s.Limit), pct, s.PIDs}
}
func number(v uint64) *float64 {
	if v == 0 {
		return nil
	}
	n := float64(v)
	return &n
}
