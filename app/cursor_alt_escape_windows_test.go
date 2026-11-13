//go:build native && windows

package main

import "testing"

func TestAltEscapeOverridesChangedGameClip(t *testing.T) {
	oldSet, oldGet := cursorSetClip, cursorCurrentClip
	oldOwned, oldRect, oldEscaped := cursorLockOwned, cursorLockRect, cursorAltEscaped
	defer func() {
		cursorSetClip, cursorCurrentClip = oldSet, oldGet
		cursorLockOwned, cursorLockRect, cursorAltEscaped = oldOwned, oldRect, oldEscaped
	}()
	for _, owned := range []bool{false, true} {
		calls := 0
		cursorLockOwned = owned
		cursorCurrentClip = func() (rect, bool) { return rect{1, 1, 500, 500}, true }
		cursorSetClip = func(r *rect) bool {
			if r != nil {
				t.Fatal("Alt must remove clipping")
			}
			calls++
			return true
		}
		suspendCursorForAlt(true)
		suspendCursorForAlt(true)
		if calls != 2 || cursorLockOwned || !cursorAltEscaped {
			t.Fatalf("escape failed: %d", calls)
		}
	}
	cursorLockOwned = false
	cursorSetClip = func(r *rect) bool { t.Fatal("must not override another foreground app"); return false }
	suspendCursorForAlt(false)
}
