//go:build native && windows

package main

import "unsafe"

func updateOverlayRegion(window uintptr) {
	if !nativeModern || !soloMode {
		user32.NewProc("SetWindowRgn").Call(window, 0, 1)
		return
	}
	var bounds rect
	if ok, _, _ := pGetClient.Call(window, uintptr(unsafe.Pointer(&bounds))); ok == 0 {
		return
	}
	width, height := bounds.Right, bounds.Bottom
	if width <= 0 || height <= 0 {
		return
	}
	// Match the outer card drawn in paintFantasyOverlay, in device pixels.
	left, top := 3*width/360, 3*height/fantasyOverlayHeight
	right, bottom := 357*width/360, (fantasyOverlayHeight-2)*height/fantasyOverlayHeight
	region, _, _ := gdi32.NewProc("CreateRoundRectRgn").Call(
		signed(left), signed(top), signed(right), signed(bottom),
		uintptr(max(1, 25*width/360)), uintptr(max(1, 25*height/fantasyOverlayHeight)),
	)
	if region == 0 {
		return
	}
	// A successful SetWindowRgn transfers ownership to Windows.
	if ok, _, _ := user32.NewProc("SetWindowRgn").Call(window, region, 1); ok == 0 {
		pDelete.Call(region)
	}
}
