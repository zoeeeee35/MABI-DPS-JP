package main

import "testing"

func TestCursorResizeGestureRemainsUnrestricted(t *testing.T) {
	g := cursorInteractionGuard{}
	for i, step := range []struct{ left, changed, outside, moving, want bool }{
		{true, true, true, false, true},   // Border activates game before move loop.
		{true, false, false, false, true}, // Pointer enters client during same drag.
		{true, false, false, true, true},
		{false, false, false, true, true},   // Keyboard/system move still in progress.
		{false, false, false, false, false}, // Safe to confine again.
		{true, false, false, false, false},  // Ordinary in-game attack must stay confined.
		{false, false, false, false, false},
		{true, false, true, false, true}, // Resize without foreground change.
		{false, false, false, false, false},
	} {
		if got := g.pause(step.left, step.changed, step.outside, step.moving); got != step.want {
			t.Fatalf("step %d got=%v", i, got)
		}
	}
}
