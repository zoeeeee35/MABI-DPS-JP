//go:build native && windows

package main

import "testing"

func TestCursorReleaseRespectsClipOwnership(t *testing.T) {
	oldSet, oldGet := cursorSetClip, cursorCurrentClip
	oldOwned, oldRect := cursorLockOwned, cursorLockRect
	defer func() {
		cursorSetClip, cursorCurrentClip = oldSet, oldGet
		cursorLockOwned, cursorLockRect = oldOwned, oldRect
	}()
	ours := rect{10, 20, 300, 400}
	for _, tc := range []struct {
		name      string
		owned, ok bool
		current   rect
		want      int
	}{
		{"ours", true, true, ours, 1},
		{"another application", true, true, rect{0, 0, 100, 100}, 0},
		{"not owned", false, true, ours, 0},
		{"query failure", true, false, ours, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			cursorLockOwned, cursorLockRect = tc.owned, ours
			cursorCurrentClip = func() (rect, bool) { return tc.current, tc.ok }
			cursorSetClip = func(r *rect) bool {
				if r != nil {
					t.Fatal("release must unclip")
				}
				calls++
				return true
			}
			releaseCursorLock()
			if calls != tc.want || cursorLockOwned {
				t.Fatalf("calls=%d owned=%v", calls, cursorLockOwned)
			}
			releaseCursorLock()
			if calls != tc.want {
				t.Fatal("duplicate release")
			}
		})
	}
}
