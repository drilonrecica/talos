// SPDX-License-Identifier: AGPL-3.0-only
package diagnostics

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadProcessParsesStatStatusAndCgroup(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "42")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	stat := "42 (worker thread) S 1 1 1 0 0 0 0 0 0 0 20 10 0 0 0 0 1 0 500 0 25"
	files := map[string]string{
		"stat": stat, "cmdline": "worker\x00--serve\x00", "status": "Name:\tworker\nUid:\t1001\t1001\t1001\t1001\n",
		"cgroup": "0::/docker/" + strings.Repeat("a", 64) + "\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := readProcess(root, 42)
	if err != nil {
		t.Fatal(err)
	}
	if got.Command != "worker --serve" || got.UID != 1001 || got.Ticks != 30 || got.StartTicks != 500 || got.ContainerID != strings.Repeat("a", 64) {
		t.Fatalf("process=%+v", got)
	}
}

func TestProcessHelpersHandleMalformedAndUsers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "passwd")
	if err := os.WriteFile(path, []byte("root:x:0:0::/root:/bin/sh\ninvalid\nalice:x:1001:1001::/home/alice:/bin/sh\n"), 0600); err != nil {
		t.Fatal(err)
	}
	users := readPasswd(path)
	if users[0] != "root" || users[1001] != "alice" {
		t.Fatalf("users=%v", users)
	}
	if containerFromCgroup("0::/user.slice") != "" {
		t.Fatal("associated host process with a container")
	}
}
