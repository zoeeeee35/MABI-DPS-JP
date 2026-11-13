//go:build native && windows

package main

import (
	"time"
	"unsafe"
)

var soloFontData [][]byte
var soloFontsLoaded uint32
var soloFonts = map[int32]uintptr{}

func loadSoloFonts() {
	for _, name := range []string{"Paperlogy-4Regular.ttf", "Paperlogy-7Bold.ttf", "MPLUSRounded1c-Bold.ttf"} {
		b, e := assets.ReadFile("fonts/" + name)
		if e != nil {
			continue
		}
		var count uint32
		gdi32.NewProc("AddFontMemResourceEx").Call(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0, uintptr(unsafe.Pointer(&count)))
		soloFontsLoaded += count
		soloFontData = append(soloFontData, b)
	}
}
func soloFont(size int32) uintptr {
	if f := soloFonts[size]; f != 0 {
		return f
	}
	face := u16("Paperlogy 7 Bold")
	h, _, _ := pFont.Call(signed(-size), 0, 0, 0, 700, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
	soloFonts[size] = h
	return h
}
func soloValue(dc uintptr, s string, r rect, color uint32, size int32) {
	draw(dc, s, r, color, soloFont(size), 0x8021)
}
func paintSoloOverlay(window, dc uintptr) { paintFantasyOverlay(window, dc) }
func soloOverlayClick(window uintptr, x, y int32) bool {
	var r rect
	pGetClient.Call(window, uintptr(unsafe.Pointer(&r)))
	if r.Right == 0 || r.Bottom == 0 {
		return false
	}
	x = x * 360 / r.Right
	y = y*fantasyOverlayHeight/r.Bottom + overlayResourceGap
	y = overlayControlY(y)
	if y < 0 {
		return true
	}
	if y >= 305 && y < 329 {
		switch int((x - 18) / 82) {
		case 0:
			historyIndex = -1
			displayCaptureTotal = false
		case 1:
			historyIndex = -1
			displayCaptureTotal = true
		case 2:
			nativePage = "スキル"
			nativeScroll = 0
			show(mainWindow, true)
		case 3:
			soloReidentify()
		}
		updateDisplay()
		return true
	}
	if y >= 340 && y < 373 {
		if x >= 18 && x < 110 {
			soloReset()
			invalidate(window)
			return true
		}
		action := soloTimerHit(x, y)
		if action >= 1 && action <= 3 {
			soloTimerMinutes = action
			cancelSoloMeasurement()
			invalidate(window)
			return true
		}
		if action == 4 {
			if soloTimerEnd.IsZero() || time.Now().After(soloTimerEnd) {
				beginSoloMeasurement()
			} else {
				cancelSoloMeasurement()
			}
			invalidate(window)
			return true
		}
	}
	return false
}

func soloMiniChart(dc uintptr, b *Battle, r rect) {
	if b == nil || len(b.DamageEvents) == 0 {
		return
	}
	type point struct{ x, mean, instant float64 }
	points := []point{}
	sum, window := int64(0), int64(0)
	left := 0
	peak := 1.0
	for i, e := range b.DamageEvents {
		sum += e.Damage
		window += e.Damage
		for left < i && b.DamageEvents[left].At < e.At-1000 {
			window -= b.DamageEvents[left].Damage
			left++
		}
		if i%max(1, len(b.DamageEvents)/40) != 0 && i != len(b.DamageEvents)-1 {
			continue
		}
		sec := float64(e.At-b.Start) / 1000
		p := point{sec, float64(sum) / max(1, sec), float64(window)}
		peak = max(peak, p.mean, p.instant)
		points = append(points, p)
	}
	for line, color := range []uint32{0x9E85E1, 0x3ABCE9} {
		pen, _, _ := gdi32.NewProc("CreatePen").Call(0, 1, uintptr(color))
		old, _, _ := pSelect.Call(dc, pen)
		for i, p := range points {
			value := p.mean
			if line == 1 {
				value = p.instant
			}
			x := r.Left + int32(p.x/max(1, b.Duration)*float64(r.Right-r.Left))
			y := r.Bottom - int32(value/peak*float64(r.Bottom-r.Top))
			if i == 0 {
				gdi32.NewProc("MoveToEx").Call(dc, signed(x), signed(y), 0)
			} else {
				gdi32.NewProc("LineTo").Call(dc, signed(x), signed(y))
			}
		}
		pSelect.Call(dc, old)
		pDelete.Call(pen)
	}
}

// Overlay-only soft cards. Main-window widgets keep their own styling.
func soloCard(dc uintptr, r rect, color uint32) {
	brush, _, _ := pCreateBrush.Call(uintptr(color))
	old, _, _ := pSelect.Call(dc, brush)
	pen, _, _ := gdi32.NewProc("GetStockObject").Call(8)
	oldPen, _, _ := pSelect.Call(dc, pen)
	gdi32.NewProc("RoundRect").Call(dc, signed(r.Left), signed(r.Top), signed(r.Right), signed(r.Bottom), 18, 18)
	pSelect.Call(dc, oldPen)
	pSelect.Call(dc, old)
	pDelete.Call(brush)
}
