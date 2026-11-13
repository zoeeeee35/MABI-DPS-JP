package main

import "time"

func soloTimerDuration(minutes int) time.Duration {
	if minutes < 1 || minutes > 3 {
		minutes = 2
	}
	return time.Duration(minutes) * time.Minute
}
func soloTimerHit(x, y int32) int {
	if y < 340 || y >= 373 {
		return 0
	}
	for i := 1; i <= 3; i++ {
		left := int32(116 + (i-1)*46)
		if x >= left && x < left+42 {
			return i
		}
	}
	if x >= 254 && x < 342 {
		return 4
	}
	return 0
}

const soloPreparation = 5 * time.Second

func soloCountdown(start, now time.Time) int {
	remaining := start.Sub(now)
	if remaining <= 0 {
		return 0
	}
	return int((remaining + time.Second - 1) / time.Second)
}
