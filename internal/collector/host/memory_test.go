package host

import "testing"

func TestMemory(t *testing.T) {
	m, e := ParseMeminfo("MemTotal: 100 kB\nMemAvailable: 40 kB\nCached: 2 kB\n")
	if e != nil || m.Used != 60*1024 {
		t.Fatalf("%+v %v", m, e)
	}
	if _, e := ParseLoadavg("bad"); e == nil {
		t.Fatal("bad load")
	}
	if _, e := ParseUptime(""); e == nil {
		t.Fatal("bad uptime")
	}
}
