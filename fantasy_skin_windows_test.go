//go:build native && windows

package main

import "testing"

func TestBarrierBadgeStaysRoundWithUnequalScaling(t *testing.T) {
	source, _, _ := user32.NewProc("GetDC").Call(0)
	if source == 0 {
		t.Fatal("no color DC")
	}
	defer user32.NewProc("ReleaseDC").Call(0, source)
	dc, _, _ := gdi32.NewProc("CreateCompatibleDC").Call(source)
	bitmap, _, _ := gdi32.NewProc("CreateCompatibleBitmap").Call(source, 60, 60)
	if dc == 0 || bitmap == 0 {
		t.Fatal("bitmap allocation")
	}
	defer gdi32.NewProc("DeleteDC").Call(dc)
	defer pDelete.Call(bitmap)
	old, _, _ := pSelect.Call(dc, bitmap)
	defer pSelect.Call(dc, old)
	for _, viewport := range []point{{720, 346}, {360, 692}} {
		gdi32.NewProc("SetMapMode").Call(dc, 1)
		fill(dc, rect{0, 0, 60, 60}, 0)
		gdi32.NewProc("SetMapMode").Call(dc, 8)
		gdi32.NewProc("SetWindowExtEx").Call(dc, 360, 346, 0)
		gdi32.NewProc("SetViewportExtEx").Call(dc, uintptr(viewport.X), uintptr(viewport.Y), 0)
		drawBarrierBadge(dc, 5, 5, 20)
		gdi32.NewProc("SetMapMode").Call(dc, 1)
		bounds := rect{60, 60, 0, 0}
		getPixel := gdi32.NewProc("GetPixel")
		for y := int32(0); y < 60; y++ {
			for x := int32(0); x < 60; x++ {
				if pixel, _, _ := getPixel.Call(uintptr(dc), uintptr(x), uintptr(y)); pixel != 0 {
					bounds = rect{min(bounds.Left, x), min(bounds.Top, y), max(bounds.Right, x+1), max(bounds.Bottom, y+1)}
				}
			}
		}
		width, height := bounds.Right-bounds.Left, bounds.Bottom-bounds.Top
		if width != height || width < 19 || width > 20 {
			t.Fatalf("viewport %+v: badge bounds %+v, want a square of about 20 pixels", viewport, bounds)
		}
	}
}

func TestFantasyGradientClipsInDeviceCoordinates(t *testing.T) {
	source, _, _ := user32.NewProc("GetDC").Call(0)
	if source == 0 {
		t.Fatal("no color DC")
	}
	defer user32.NewProc("ReleaseDC").Call(0, source)
	for _, scale := range []int32{100, 125, 150} {
		w, h := 360*scale/100, 400*scale/100
		dc, _, _ := gdi32.NewProc("CreateCompatibleDC").Call(source)
		bitmap, _, _ := gdi32.NewProc("CreateCompatibleBitmap").Call(source, uintptr(w), uintptr(h))
		if dc == 0 || bitmap == 0 {
			t.Fatal("DIB allocation")
		}
		old, _, _ := pSelect.Call(dc, bitmap)
		gdi32.NewProc("SetMapMode").Call(dc, 8)
		gdi32.NewProc("SetWindowExtEx").Call(dc, 360, 400, 0)
		gdi32.NewProc("SetViewportExtEx").Call(dc, uintptr(w), uintptr(h), 0)
		fill(dc, rect{0, 0, 360, 400}, 0x211911)
		fantasyCard(dc, rect{162, 340, 204, 373}, 0x7DCFFF, 0x35A7E8, 20)
		pixel, _, _ := gdi32.NewProc("GetPixel").Call(dc, uintptr(183), uintptr(344))
		pSelect.Call(dc, old)
		pDelete.Call(bitmap)
		gdi32.NewProc("DeleteDC").Call(dc)
		r, g, b := pixel&255, (pixel>>8)&255, (pixel>>16)&255
		if r < 150 || g < 100 || b > 200 {
			t.Fatalf("%d%% gradient not filled at mapped position: %X", scale, pixel)
		}
	}
}
