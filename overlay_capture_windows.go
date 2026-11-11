//go:build native && windows

package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"unsafe"
)

// captureOwnOverlay copies only this application's overlay client surface.
// It does not capture the desktop, the game window, or other applications.
func captureOwnOverlay(path string) error { return captureOwnSurface(path, overlayWindow, false) }
func captureOwnSurface(path string, window uintptr, main bool) error {
	var r rect
	pGetClient.Call(window, uintptr(unsafe.Pointer(&r)))
	width, height := int(r.Right), int(r.Bottom)
	if width <= 0 || height <= 0 {
		return fmt.Errorf("オーバーレイのサイズが不正です")
	}
	source, _, _ := user32.NewProc("GetDC").Call(window)
	if source == 0 {
		return fmt.Errorf("描画面を取得できません")
	}
	defer user32.NewProc("ReleaseDC").Call(window, source)
	dc, _, _ := gdi32.NewProc("CreateCompatibleDC").Call(source)
	if dc == 0 {
		return fmt.Errorf("描画領域を作成できません")
	}
	defer gdi32.NewProc("DeleteDC").Call(dc)
	bitmap, _, _ := gdi32.NewProc("CreateCompatibleBitmap").Call(source, uintptr(width), uintptr(height))
	if bitmap == 0 {
		return fmt.Errorf("画像を作成できません")
	}
	defer pDelete.Call(bitmap)
	old, _, _ := pSelect.Call(dc, bitmap)
	if window != mainWindow && window != overlayWindow {
		fill(dc, r, 0x251A14)
		send(window, 0x0317, dc, 0x1c)
	} else if main && nativeModern {
		paintBuffered(window, dc, nativeLayout)
	} else if main {
		user32.NewProc("PrintWindow").Call(window, dc, 3)
	} else {
		paintBuffered(window, dc, paintOverlayContent)
	}
	pSelect.Call(dc, old)
	type bitmapHeader struct {
		Size                   uint32
		Width, Height          int32
		Planes, BitCount       uint16
		Compression, ImageSize uint32
		XPels, YPels           int32
		Used, Important        uint32
	}
	header := bitmapHeader{Size: 40, Width: int32(width), Height: -int32(height), Planes: 1, BitCount: 32}
	pixels := make([]byte, width*height*4)
	lines, _, _ := gdi32.NewProc("GetDIBits").Call(dc, bitmap, 0, uintptr(height), uintptr(unsafe.Pointer(&pixels[0])), uintptr(unsafe.Pointer(&header)), 0)
	if int(lines) != height {
		return fmt.Errorf("描画画像を取得できません")
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(pixels); i += 4 {
		img.Pix[i] = pixels[i+2]
		img.Pix[i+1] = pixels[i+1]
		img.Pix[i+2] = pixels[i]
		img.Pix[i+3] = 255
	}
	f, e := os.Create(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return png.Encode(f, img)
}
