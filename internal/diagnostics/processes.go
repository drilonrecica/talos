// SPDX-License-Identifier: AGPL-3.0-only
package diagnostics

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultProcesses    = 25
	MaxProcesses        = 100
	maxScannedProcesses = 4096
	maxProcFileBytes    = 64 << 10
)

var ErrProcessScanBusy = errors.New("a process scan is already running")

type Process struct {
	PID           int     `json:"pid"`
	Command       string  `json:"command"`
	CPUPercent    float64 `json:"cpuPct"`
	RSSBytes      int64   `json:"rssBytes"`
	User          string  `json:"user"`
	UID           uint64  `json:"uid"`
	State         string  `json:"state"`
	UptimeSeconds float64 `json:"uptimeSeconds"`
	ContainerID   string  `json:"containerId,omitempty"`
}

type ProcessScanner struct {
	ProcRoot, PasswdPath string
	SampleInterval       time.Duration
	sem                  chan struct{}
}

func NewProcessScanner(procRoot, passwdPath string) *ProcessScanner {
	return &ProcessScanner{ProcRoot: procRoot, PasswdPath: passwdPath, SampleInterval: 200 * time.Millisecond, sem: make(chan struct{}, 1)}
}

type processSample struct {
	PID                         int
	Command, State, ContainerID string
	Ticks                       uint64
	StartTicks                  uint64
	RSSBytes                    int64
	UID                         uint64
}

func (s *ProcessScanner) Scan(ctx context.Context, limit int) ([]Process, error) {
	if limit == 0 {
		limit = DefaultProcesses
	}
	if limit < 1 || limit > MaxProcesses {
		return nil, fmt.Errorf("process limit must be between 1 and %d", MaxProcesses)
	}
	select {
	case s.sem <- struct{}{}:
		defer func() { <-s.sem }()
	default:
		return nil, ErrProcessScanBusy
	}
	firstTotal, first, err := s.sample(ctx)
	if err != nil {
		return nil, err
	}
	timer := time.NewTimer(s.SampleInterval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}
	secondTotal, second, err := s.sample(ctx)
	if err != nil {
		return nil, err
	}
	deltaTotal := secondTotal - firstTotal
	if secondTotal < firstTotal || deltaTotal == 0 {
		deltaTotal = 1
	}
	users := readPasswd(s.PasswdPath)
	uptime := readUptime(s.ProcRoot)
	values := make([]Process, 0, len(second))
	for pid, current := range second {
		previous, ok := first[pid]
		if !ok || current.StartTicks != previous.StartTicks {
			continue
		}
		delta := current.Ticks - previous.Ticks
		if current.Ticks < previous.Ticks {
			delta = 0
		}
		user := users[current.UID]
		if user == "" {
			user = strconv.FormatUint(current.UID, 10)
		}
		age := uptime - float64(current.StartTicks)/100
		if age < 0 {
			age = 0
		}
		values = append(values, Process{PID: pid, Command: current.Command, CPUPercent: float64(delta) * 100 / float64(deltaTotal), RSSBytes: current.RSSBytes, User: user, UID: current.UID, State: current.State, UptimeSeconds: age, ContainerID: current.ContainerID})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].CPUPercent == values[j].CPUPercent {
			return values[i].RSSBytes > values[j].RSSBytes
		}
		return values[i].CPUPercent > values[j].CPUPercent
	})
	if len(values) > limit {
		values = values[:limit]
	}
	return values, nil
}

func (s *ProcessScanner) sample(ctx context.Context) (uint64, map[int]processSample, error) {
	total, err := readCPUTotal(filepath.Join(s.ProcRoot, "stat"))
	if err != nil {
		return 0, nil, err
	}
	entries, err := os.ReadDir(s.ProcRoot)
	if err != nil {
		return 0, nil, err
	}
	values := map[int]processSample{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return 0, nil, err
		}
		if len(values) >= maxScannedProcesses {
			break
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 || !entry.IsDir() {
			continue
		}
		value, err := readProcess(s.ProcRoot, pid)
		if err == nil {
			values[pid] = value
		}
	}
	return total, values, nil
}

func readProcess(root string, pid int) (processSample, error) {
	dir := filepath.Join(root, strconv.Itoa(pid))
	stat, err := readBounded(filepath.Join(dir, "stat"))
	if err != nil {
		return processSample{}, err
	}
	open, close := strings.IndexByte(stat, '('), strings.LastIndex(stat, ") ")
	if open < 0 || close < open {
		return processSample{}, errors.New("malformed process stat")
	}
	fields := strings.Fields(stat[close+2:])
	if len(fields) < 22 {
		return processSample{}, errors.New("short process stat")
	}
	utime, e1 := strconv.ParseUint(fields[11], 10, 64)
	stime, e2 := strconv.ParseUint(fields[12], 10, 64)
	start, e3 := strconv.ParseUint(fields[19], 10, 64)
	rss, e4 := strconv.ParseInt(fields[21], 10, 64)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
		return processSample{}, errors.New("invalid process counters")
	}
	command := strings.TrimSpace(stat[open+1 : close])
	if raw, err := readBounded(filepath.Join(dir, "cmdline")); err == nil {
		if value := strings.TrimSpace(strings.ReplaceAll(raw, "\x00", " ")); value != "" {
			command = value
		}
	}
	uid := uint64(0)
	if status, err := readBounded(filepath.Join(dir, "status")); err == nil {
		for _, line := range strings.Split(status, "\n") {
			if strings.HasPrefix(line, "Uid:") {
				parts := strings.Fields(line)
				if len(parts) > 1 {
					uid, _ = strconv.ParseUint(parts[1], 10, 64)
				}
				break
			}
		}
	}
	container := ""
	if cgroup, err := readBounded(filepath.Join(dir, "cgroup")); err == nil {
		container = containerFromCgroup(cgroup)
	}
	return processSample{PID: pid, Command: command, State: fields[0], Ticks: utime + stime, StartTicks: start, RSSBytes: max(0, rss) * int64(os.Getpagesize()), UID: uid, ContainerID: container}, nil
}

func readCPUTotal(path string) (uint64, error) {
	raw, err := readBounded(path)
	if err != nil {
		return 0, err
	}
	line, _, _ := strings.Cut(raw, "\n")
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "cpu" {
		return 0, errors.New("malformed host cpu stat")
	}
	var total uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, err
		}
		total += value
	}
	return total, nil
}
func readUptime(root string) float64 {
	raw, err := readBounded(filepath.Join(root, "uptime"))
	if err != nil {
		return 0
	}
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return 0
	}
	value, _ := strconv.ParseFloat(fields[0], 64)
	return value
}
func readBounded(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), maxProcFileBytes)
	var b strings.Builder
	for scanner.Scan() {
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(scanner.Text())
	}
	return b.String(), scanner.Err()
}
func readPasswd(path string) map[uint64]string {
	users := map[uint64]string{}
	raw, err := readBounded(path)
	if err != nil {
		return users
	}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Split(line, ":")
		if len(fields) < 3 {
			continue
		}
		uid, err := strconv.ParseUint(fields[2], 10, 64)
		if err == nil {
			users[uid] = fields[0]
		}
	}
	return users
}
func containerFromCgroup(raw string) string {
	for _, field := range strings.FieldsFunc(raw, func(r rune) bool { return r == '/' || r == ':' || r == '\n' || r == '-' }) {
		field = strings.TrimSuffix(field, ".scope")
		if len(field) >= 64 {
			candidate := field[len(field)-64:]
			if allHex(candidate) {
				return candidate
			}
		}
	}
	return ""
}
func allHex(value string) bool {
	for _, r := range value {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}
