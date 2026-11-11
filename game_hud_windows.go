//go:build native && windows

package main

import (
	"time"
	"unsafe"
)

var hudProbe struct {
	window                  uintptr
	bounds                  rect
	next                    time.Time
	hidden, previousMissing bool
}

func hideMiniForMissingHUD(now time.Time, client rect) bool {
	if !gameOverlay.HideWithoutHUD || gameOverlayAdjust {
		hudProbe.window = 0
		return false
	}
	if hudProbe.window != gameOverlayTarget || hudProbe.bounds != client {
		hudProbe.window, hudProbe.bounds = gameOverlayTarget, client
		hudProbe.next, hudProbe.hidden, hudProbe.previousMissing = time.Time{}, false, false
	}
	if now.Before(hudProbe.next) {
		return hudProbe.hidden
	}
	hudProbe.next = now.Add(100 * time.Millisecond)
	visible, valid := sampleGameHUD(client)
	if !valid {
		hudProbe.previousMissing = false
		return hudProbe.hidden
	}
	missing := !visible
	// Confirm a second missing sample to ignore a single transient frame.
	hudProbe.hidden = missing && (hudProbe.hidden || hudProbe.previousMissing)
	hudProbe.previousMissing = missing
	return hudProbe.hidden
}

func sampleGameHUD(client rect) (visible, valid bool) {
	// Reuse one in-memory bitmap for the HUD and the loading-screen check.
	width := min(int32(420), client.Right-client.Left)
	height := min(int32(210), client.Bottom-client.Top)
	source, _, _ := user32.NewProc("GetDC").Call(0)
	if source == 0 {
		return false, false
	}
	defer user32.NewProc("ReleaseDC").Call(0, source)
	dc, _, _ := gdi32.NewProc("CreateCompatibleDC").Call(source)
	if dc == 0 {
		return false, false
	}
	defer gdi32.NewProc("DeleteDC").Call(dc)
	header := struct {
		Size          uint32
		Width, Height int32
		Planes, Bits  uint16
		Rest          [6]uint32
	}{Size: 40, Width: width, Height: -height, Planes: 1, Bits: 32}
	var bits unsafe.Pointer
	bitmap, _, _ := gdi32.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 {
		return false, false
	}
	defer pDelete.Call(bitmap)
	old, _, _ := pSelect.Call(dc, bitmap)
	defer pSelect.Call(dc, old)
	copyRegion := func(x, y int32) bool {
		ok, _, _ := gdi32.NewProc("BitBlt").Call(dc, 0, 0, uintptr(width), uintptr(height), source, signed(x), signed(y), 0x00CC0020)
		if ok == 0 {
			return false
		}
		gdi32.NewProc("GdiFlush").Call()
		return true
	}
	if !copyRegion(client.Left, client.Top) {
		return false, false
	}
	pixels := unsafe.Slice((*uint32)(bits), int(width*height))
	if visible, valid := detectGameHUD(pixels, int(width), int(height)); valid {
		return visible, true
	}

	// A dark HUD corner is also returned when the capture is unavailable.
	// Check for visible loading-screen text in the center before hiding.
	x := client.Left + (client.Right-client.Left-width)/2
	y := client.Top + (client.Bottom-client.Top-height)/2
	return false, copyRegion(x, y) && hasLoadingScreenText(pixels)
}

func hasLoadingScreenText(pixels []uint32) bool {
	bright, dark := 0, 0
	for _, p := range pixels {
		r, g, b := p>>16&255, p>>8&255, p&255
		if r >= 160 && g >= 160 && b >= 160 {
			bright++
		}
		if r <= 20 && g <= 20 && b <= 20 {
			dark++
		}
	}
	return bright >= 32 && dark*10 >= len(pixels)*8
}

func detectGameHUD(pixels []uint32, width, height int) (visible, valid bool) {
	// A flat, vivid green horizontal bar is expected to the right of a
	// cyan portrait rim. Check multiple rows so stray text does not qualify.
	nonBlack := 0
	for _, pixel := range pixels {
		if pixel&0xFFFFFF != 0 {
			nonBlack++
		}
	}
	if nonBlack < width*height/10 {
		return false, false
	}
	green := func(pixel uint32) bool {
		r, g, b := int(pixel>>16&255), int(pixel>>8&255), int(pixel&255)
		return g >= 130 && g-r >= 68 && g-b >= 45
	}
	for y := 14; y+2 < height; y++ {
		run := 0
		for x := 25; x < width; x++ {
			if green(pixels[y*width+x]) && green(pixels[(y+2)*width+x]) {
				run++
			} else {
				run = 0
			}
			if run != 14 {
				continue
			}
			start := x - run + 1
			cyan := 0
			for yy := max(0, y-80); yy < min(height, y+22); yy++ {
				for xx := max(0, start-145); xx < start-5; xx++ {
					pixel := pixels[yy*width+xx]
					r, g, b := int(pixel>>16&255), int(pixel>>8&255), int(pixel&255)
					if g >= 130 && b >= 155 && g-r >= 40 && b-r >= 60 {
						cyan++
					}
				}
			}
			if cyan >= 24 {
				return true, true
			}
		}
	}
	return false, true
}
