package main

import (
	"testing"
	"time"
)

func TestReceiveMeterActualDeltaStopAndReset(t *testing.T) {
	now := time.Now()
	m := ReceiveMeter{}
	m.Update(now, 100, true)
	m.Update(now.Add(time.Second), 2148, true)
	if m.Rate != 2048 || len(m.Samples) != 1 {
		t.Fatal(m)
	}
	m.Update(now.Add(1100*time.Millisecond), 2200, true)
	if m.Total != 2148 {
		t.Fatal("sampling too soon")
	}
	m.Update(now.Add(2*time.Second), 2148, true)
	if m.Rate != 0 {
		t.Fatal("idle traffic fabricated")
	}
	m.Update(now.Add(3*time.Second), 0, true)
	if m.Rate != 0 || len(m.Samples) != 0 {
		t.Fatal("counter reset")
	}
	m.Update(now.Add(4*time.Second), 1000, false)
	if m.Rate != 0 || m.Running {
		t.Fatal("stopped state")
	}
}
