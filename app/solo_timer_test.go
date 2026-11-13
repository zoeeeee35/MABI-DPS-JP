package main

import (
	"testing"
	"time"
)

func TestSoloTimerSelectionAndBounds(t *testing.T) {
	for i := 1; i <= 3; i++ {
		if soloTimerDuration(i) != time.Duration(i)*time.Minute {
			t.Fatal(i)
		}
		if soloTimerHit(int32(120+(i-1)*46), 350) != i {
			t.Fatal(i)
		}
	}
	if soloTimerHit(300, 350) != 4 || soloTimerHit(300, 339) != 0 || soloTimerHit(158, 350) != 0 || soloTimerDuration(0) != 2*time.Minute {
		t.Fatal("bounds/default")
	}
}

func TestSoloPreparationCountdown(t *testing.T) {
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	start := now.Add(soloPreparation)
	for _, tc := range []struct {
		elapsed time.Duration
		want    int
	}{{0, 5}, {time.Millisecond, 5}, {time.Second, 4}, {2 * time.Second, 3}, {3 * time.Second, 2}, {4 * time.Second, 1}, {5*time.Second - time.Nanosecond, 1}, {5 * time.Second, 0}, {6 * time.Second, 0}} {
		if got := soloCountdown(start, now.Add(tc.elapsed)); got != tc.want {
			t.Fatalf("%s: %d != %d", tc.elapsed, got, tc.want)
		}
	}
}
