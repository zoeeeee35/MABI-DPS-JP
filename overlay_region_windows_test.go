//go:build native && windows

package main

import (
	"runtime"
	"syscall"
	"testing"
)

var regionTestProc = syscall.NewCallback(func(window uintptr, msg uint32, w, l uintptr) uintptr {
	result, _, _ := pDefWindow.Call(window, uintptr(msg), w, l)
	return result
})

func TestOverlayCornersClippedAfterResize(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	oldModern, oldSolo := nativeModern, soloMode
	nativeModern, soloMode = true, true
	defer func() { nativeModern, soloMode = oldModern, oldSolo }()
	if err := registerClass("MabiDPSRegionTest", regionTestProc, 0); err != nil {
		t.Fatal(err)
	}
	window := createWindow("MabiDPSRegionTest", "", 0, 0x80000000, 0, 0, 360, 346, 0, 0)
	if window == 0 {
		t.Fatal("test window creation failed")
	}
	defer pDestroy.Call(window)
	for _, size := range []point{{360, 346}, {720, 500}} {
		pSetWindowPos.Call(window, 0, 0, 0, uintptr(size.X), uintptr(size.Y), 0x0016)
		updateOverlayRegion(window)
		region, _, _ := gdi32.NewProc("CreateRectRgn").Call(0, 0, 0, 0)
		if kind, _, _ := user32.NewProc("GetWindowRgn").Call(window, region); kind == 0 {
			pDelete.Call(region)
			t.Fatal("window region missing")
		}
		for _, p := range []point{{0, 0}, {size.X - 1, 0}, {0, size.Y - 1}, {size.X - 1, size.Y - 1}} {
			if inside, _, _ := gdi32.NewProc("PtInRegion").Call(region, signed(p.X), signed(p.Y)); inside != 0 {
				t.Errorf("corner %+v remains visible at size %+v", p, size)
			}
		}
		center, _, _ := gdi32.NewProc("PtInRegion").Call(region, signed(size.X/2), signed(size.Y/2))
		pDelete.Call(region)
		if center == 0 {
			t.Fatal("center was clipped")
		}
	}
}
