//go:build native && windows

package main

import (
	"syscall"
	"unsafe"
)

type paintBuffer struct {
	dc, bitmap, original uintptr
	width, height        int32
}

var nativePaintBuffers = map[uintptr]paintBuffer{}
var nativeBufferedFrames uint64

func releasePaintBuffer(window uintptr) {
	b, ok := nativePaintBuffers[window]
	if !ok {
		return
	}
	pSelect.Call(b.dc, b.original)
	pDelete.Call(b.bitmap)
	gdi32.NewProc("DeleteDC").Call(b.dc)
	delete(nativePaintBuffers, window)
}
func paintBuffered(window, dest uintptr, paint func(uintptr, uintptr)) {
	var r rect
	pGetClient.Call(window, uintptr(unsafe.Pointer(&r)))
	if r.Right <= 0 || r.Bottom <= 0 {
		return
	}
	b, ok := nativePaintBuffers[window]
	if !ok || b.width != r.Right || b.height != r.Bottom {
		releasePaintBuffer(window)
		dc, _, _ := gdi32.NewProc("CreateCompatibleDC").Call(dest)
		bitmap, _, _ := gdi32.NewProc("CreateCompatibleBitmap").Call(dest, uintptr(r.Right), uintptr(r.Bottom))
		if dc == 0 || bitmap == 0 {
			if bitmap != 0 {
				pDelete.Call(bitmap)
			}
			if dc != 0 {
				gdi32.NewProc("DeleteDC").Call(dc)
			}
			paint(window, dest)
			return
		}
		old, _, _ := pSelect.Call(dc, bitmap)
		b = paintBuffer{dc, bitmap, old, r.Right, r.Bottom}
		nativePaintBuffers[window] = b
	}
	paint(window, b.dc)
	gdi32.NewProc("BitBlt").Call(dest, 0, 0, uintptr(r.Right), uintptr(r.Bottom), b.dc, 0, 0, 0x00cc0020)
	nativeBufferedFrames++
}

func nativeFontFace() string {
	dc, _, _ := gdi32.NewProc("CreateCompatibleDC").Call(0)
	if dc == 0 {
		return ""
	}
	defer gdi32.NewProc("DeleteDC").Call(dc)
	old, _, _ := pSelect.Call(dc, nf(15))
	defer pSelect.Call(dc, old)
	var buf [128]uint16
	gdi32.NewProc("GetTextFaceW").Call(dc, 128, uintptr(unsafe.Pointer(&buf[0])))
	return syscall.UTF16ToString(buf[:])
}
