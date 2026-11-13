//go:build native && windows

package main

import "unsafe"

var clipCursorProc = user32.NewProc("ClipCursor")
var getClipCursorProc = user32.NewProc("GetClipCursor")
var cursorSetClip = func(bounds *rect) bool {
	result, _, _ := clipCursorProc.Call(uintptr(unsafe.Pointer(bounds)))
	return result != 0
}
var cursorCurrentClip = func() (rect, bool) {
	var current rect
	result, _, _ := getClipCursorProc.Call(uintptr(unsafe.Pointer(&current)))
	return current, result != 0
}
var cursorLockOwned bool
var cursorLockRect rect
var cursorLockForeground uintptr
var cursorLockPID uint32
var cursorLockIsGame bool
var cursorInteraction cursorInteractionGuard
var cursorAltEscaped bool

// Clip only the active game's client area. No game hooks or process writes.
func tickCursorLock() {
	if !gameOverlay.CursorLock || updateExiting {
		releaseCursorLock()
		return
	}
	window, _, _ := user32.NewProc("GetForegroundWindow").Call()
	pid := gameWindowPID(window)
	foregroundChanged := window != cursorLockForeground || pid != cursorLockPID
	if window != cursorLockForeground || pid != cursorLockPID {
		cursorLockForeground, cursorLockPID = window, pid
		cursorLockIsGame = isGameProcess(pid)
	}
	alt, _, _ := user32.NewProc("GetAsyncKeyState").Call(0x12)
	lalt, _, _ := user32.NewProc("GetAsyncKeyState").Call(0xA4)
	ralt, _, _ := user32.NewProc("GetAsyncKeyState").Call(0xA5)
	if (alt|lalt|ralt)&0x8000 != 0 {
		suspendCursorForAlt(cursorLockIsGame)
		return
	}
	if !cursorLockIsGame {
		releaseCursorLock()
		return
	}
	bounds, ok := gameClientBounds(window)
	if !ok {
		releaseCursorLock()
		return
	}
	// A border click can activate the game before Windows starts its resize loop.
	// Keep that whole mouse gesture unrestricted; also detect keyboard/system moves.
	left, _, _ := user32.NewProc("GetAsyncKeyState").Call(1)
	var pos point
	posOK, _, _ := user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&pos)))
	outside := posOK == 0 || pos.X < bounds.Left || pos.X >= bounds.Right || pos.Y < bounds.Top || pos.Y >= bounds.Bottom
	if cursorAltEscaped && outside {
		releaseCursorLock()
		return
	}
	cursorAltEscaped = false
	moving := cursorWindowMoving(window)
	if cursorInteraction.pause(left&0x8000 != 0, foregroundChanged, outside, moving) {
		releaseCursorLock()
		return
	}
	// Reapply only after the resize/activation gesture has finished.
	current, _ := cursorCurrentClip()
	if cursorLockOwned && current == bounds {
		return
	}
	if cursorSetClip(&bounds) {
		cursorLockOwned, cursorLockRect = true, bounds
	}
}

func releaseCursorLock() {
	if !cursorLockOwned {
		return
	}
	current, ok := cursorCurrentClip()
	// Do not remove a different application's clipping after focus changes.
	if ok && current == cursorLockRect {
		cursorSetClip(nil)
	}
	cursorLockOwned = false
}

// GUITHREADINFO is a public Windows read-only window-state API.
type cursorGUIThreadInfo struct {
	Size, Flags                                        uint32
	Active, Focus, Capture, MenuOwner, MoveSize, Caret uintptr
	CaretRect                                          rect
}

func cursorWindowMoving(window uintptr) bool {
	thread, _, _ := user32.NewProc("GetWindowThreadProcessId").Call(window, 0)
	info := cursorGUIThreadInfo{Size: uint32(unsafe.Sizeof(cursorGUIThreadInfo{}))}
	ok, _, _ := user32.NewProc("GetGUIThreadInfo").Call(thread, uintptr(unsafe.Pointer(&info)))
	return ok == 0 || info.Flags&2 != 0 || info.MoveSize != 0
}

// Explicit user escape overrides the game's current clipping while it is
// foreground, even if it differs from the rectangle we last applied.
func suspendCursorForAlt(gameForeground bool) {
	if gameForeground {
		if cursorSetClip(nil) {
			cursorLockOwned = false
		}
		cursorAltEscaped = true
		return
	}
	releaseCursorLock()
}
