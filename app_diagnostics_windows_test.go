//go:build native && windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestApplicationDiagnosticsNormal(t *testing.T) {
	dir := t.TempDir()
	d, e := startApplicationDiagnostics(dir)
	if e != nil {
		t.Fatal(e)
	}
	d.tick(time.Now(), true, false)
	d.finish()
	if files, _ := filepath.Glob(filepath.Join(dir, "crash-*.log")); len(files) != 0 {
		t.Fatal("empty crash file retained", files)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "app-*.log"))
	if len(files) != 1 {
		t.Fatal(files)
	}
	b, _ := os.ReadFile(files[0])
	s := string(b)
	for _, want := range []string{"START version=", "HEARTBEAT capture=true timed=false", "NORMAL_EXIT"} {
		if !strings.Contains(s, want) {
			t.Fatal(s)
		}
	}
}
func TestApplicationDiagnosticsCrashSubprocess(t *testing.T) {
	if dir := os.Getenv("MABI_DIAGNOSTIC_TEST_DIR"); dir != "" {
		d, e := startApplicationDiagnostics(dir)
		if e != nil {
			panic(e)
		}
		if os.Getenv("MABI_DIAGNOSTIC_TEST_MAIN") == "1" {
			defer d.finishAfterRun()
			panic("MABI_DIAGNOSTIC_TEST")
		}
		go func() { panic("MABI_DIAGNOSTIC_TEST") }()
		select {}
	}
	dir := t.TempDir()
	c := exec.Command(os.Args[0], "-test.run=^TestApplicationDiagnosticsCrashSubprocess$")
	c.Env = append(os.Environ(), "MABI_DIAGNOSTIC_TEST_DIR="+dir)
	if c.Run() == nil {
		t.Fatal("crash child unexpectedly succeeded")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "crash-*.log"))
	if len(files) != 1 {
		t.Fatal(files)
	}
	b, _ := os.ReadFile(files[0])
	if !strings.Contains(string(b), "MABI_DIAGNOSTIC_TEST") || !strings.Contains(string(b), "goroutine") {
		t.Fatal("crash not captured", string(b))
	}
	files, _ = filepath.Glob(filepath.Join(dir, "app-*.log"))
	b, _ = os.ReadFile(files[0])
	if strings.Contains(string(b), "NORMAL_EXIT") {
		t.Fatal("crash marked normal")
	}
}

func TestApplicationDiagnosticsMainPanic(t *testing.T) {
	dir := t.TempDir()
	c := exec.Command(os.Args[0], "-test.run=^TestApplicationDiagnosticsCrashSubprocess$")
	c.Env = append(os.Environ(), "MABI_DIAGNOSTIC_TEST_DIR="+dir, "MABI_DIAGNOSTIC_TEST_MAIN=1")
	if c.Run() == nil {
		t.Fatal("panic child succeeded")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "crash-*.log"))
	if len(files) != 1 {
		t.Fatal(files)
	}
	b, _ := os.ReadFile(files[0])
	if !strings.Contains(string(b), "MABI_DIAGNOSTIC_TEST") {
		t.Fatal("main panic missing")
	}
	files, _ = filepath.Glob(filepath.Join(dir, "app-*.log"))
	b, _ = os.ReadFile(files[0])
	if strings.Contains(string(b), "NORMAL_EXIT") || !strings.Contains(string(b), "PANIC_EXIT") {
		t.Fatal("main panic misclassified", string(b))
	}
}

func TestApplicationLogRetention(t *testing.T) {
	dir := t.TempDir()
	write := func(name, value string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for day := 1; day <= 12; day++ {
		stamp := time.Date(2026, 11, day, 0, 0, 0, 0, time.UTC).Format("20060102-150405.000000000")
		write("app-"+stamp+"-1.log", "heartbeat")
		write("crash-"+stamp+"-1.log", "stack")
	}
	write("crash-20261113-000000.000000000-1.log", "")
	write("app-manual.log", "keep")
	pruneApplicationLogs(dir)
	for pattern, want := range map[string]int{"app-2026*.log": 5, "crash-*.log": 10, "app-manual.log": 1} {
		files, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil || len(files) != want {
			t.Fatalf("%s: got %v, want %d entries", pattern, files, want)
		}
	}
}
