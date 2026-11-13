//go:build native && windows

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type applicationDiagnostics struct {
	mu                     sync.Mutex
	file, crash            *os.File
	logger                 *log.Logger
	last                   time.Time
	lastCapture, lastTimed bool
	dir                    string
}

var appDiagnostics *applicationDiagnostics

func startApplicationDiagnostics(dir string) (*applicationDiagnostics, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	stamp := fmt.Sprintf("%s-%d", time.Now().Format("20060102-150405.000000000"), os.Getpid())
	f, err := os.OpenFile(filepath.Join(dir, "app-"+stamp+".log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	c, err := os.OpenFile(filepath.Join(dir, "crash-"+stamp+".log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		f.Close()
		return nil, err
	}
	d := &applicationDiagnostics{file: f, crash: c, dir: dir, logger: log.New(f, "", log.Ldate|log.Ltime|log.Lmicroseconds)}
	if err = debug.SetCrashOutput(c, debug.CrashOptions{}); err != nil {
		f.Close()
		c.Close()
		return nil, err
	}
	d.logger.Printf("START version=%s pid=%d", appVersion(), os.Getpid())
	_ = f.Sync()
	pruneApplicationLogs(dir)
	return d, nil
}
func (d *applicationDiagnostics) tick(now time.Time, capture, timed bool) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if now.Sub(d.last) < 5*time.Minute && capture == d.lastCapture && timed == d.lastTimed {
		return
	}
	d.last = now
	d.lastCapture, d.lastTimed = capture, timed
	if s, e := d.file.Stat(); e == nil && s.Size() < (64<<10)-128 {
		d.logger.Printf("HEARTBEAT capture=%t timed=%t", capture, timed)
		_ = d.file.Sync()
	}
}
func (d *applicationDiagnostics) finish() {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.logger.Print("NORMAL_EXIT")
	_ = d.file.Sync()
	_ = debug.SetCrashOutput(nil, debug.CrashOptions{})
	crashPath := d.crash.Name()
	_ = d.crash.Close()
	if info, err := os.Stat(crashPath); err == nil && info.Size() == 0 {
		_ = os.Remove(crashPath)
	}
	_ = d.file.Close()
}
func pruneApplicationLogs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, kind := range []struct {
		prefix string
		keep   int
	}{{"app-", 5}, {"crash-", 10}} {
		var names []string
		for _, entry := range entries {
			name := entry.Name()
			if !entry.Type().IsRegular() || !strings.HasPrefix(name, kind.prefix) || !strings.HasSuffix(name, ".log") {
				continue
			}
			stem := strings.TrimSuffix(strings.TrimPrefix(name, kind.prefix), ".log")
			split := strings.LastIndexByte(stem, '-')
			if split < 0 {
				continue
			}
			if _, err := time.Parse("20060102-150405.000000000", stem[:split]); err != nil {
				continue
			}
			if pid, err := strconv.Atoi(stem[split+1:]); err != nil || pid <= 0 {
				continue
			}
			if kind.prefix == "crash-" {
				info, err := entry.Info()
				if err != nil {
					continue
				}
				if info.Size() == 0 {
					// Windows denies removal while a running process owns the crash file.
					_ = os.Remove(filepath.Join(dir, name))
					continue
				}
			}
			names = append(names, name)
		}
		sort.Strings(names)
		for i := 0; i < len(names)-kind.keep; i++ {
			_ = os.Remove(filepath.Join(dir, names[i]))
		}
	}
}

// Only a normal return is marked NORMAL_EXIT. Keep runtime crash output attached
// when a panic unwinds the main goroutine, then let the runtime terminate normally.
func (d *applicationDiagnostics) finishAfterRun() {
	if p := recover(); p != nil {
		if d != nil {
			d.mu.Lock()
			d.logger.Print("PANIC_EXIT")
			_ = d.file.Sync()
			d.mu.Unlock()
		}
		panic(p)
	}
	d.finish()
}
