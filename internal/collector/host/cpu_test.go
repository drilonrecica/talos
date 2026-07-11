package host

import (
	"strings"
	"testing"
)

func TestCPU(t *testing.T) {
	v, e := ParseProcStat("cpu 10 0 10 80 0 0 0 0\ncpu0 10 0 10 80\n")
	if e != nil || len(v) != 2 {
		t.Fatal(e)
	}
	d := CPUDelta(v["cpu"], CPUCounters{User: 20, System: 20, Idle: 160})
	if d.Busy == nil || *d.Busy != 20 {
		t.Fatalf("%+v", d)
	}
	if CPUDelta(CPUCounters{Idle: 2}, CPUCounters{Idle: 1}).Busy != nil {
		t.Fatal("reset")
	}
	_, e = ParseProcStat(strings.TrimSpace("cpu x"))
	if e == nil {
		t.Fatal("malformed")
	}
}
