package main

// Latch an activation/border gesture until the mouse is released, including
// the gap before Windows enters its move/size message loop.
type cursorInteractionGuard struct{ dragging bool }

func (g *cursorInteractionGuard) pause(leftDown, foregroundChanged, outside, moving bool) bool {
	if !leftDown {
		g.dragging = false
	}
	if leftDown && (foregroundChanged || outside || moving) {
		g.dragging = true
	}
	return moving || g.dragging
}
