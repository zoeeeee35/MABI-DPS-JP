//go:build native && windows

package main

import (
	"fmt"
	"time"
)

var nativeReceiveMeter ReceiveMeter
var nativeMeterAt time.Time

func updateNativeNetworkMeter(now time.Time) {
	if now.Sub(nativeMeterAt) < 500*time.Millisecond {
		return
	}
	nativeMeterAt = now
	s := captureCtl.snapshot()
	total := uint64(0)
	for _, f := range s.Traffic {
		if f.Protocol == "tcp" && f.Direction == "receive" && f.PeerPort == 16000 {
			total += f.PayloadBytes
		}
	}
	wasRunning := nativeReceiveMeter.Running
	nativeReceiveMeter.Update(now, total, s.Running)
	if nativeModern && (s.Running || wasRunning) {
		invalidate(mainWindow)
	}
}
func nativeNetworkMeter(dc uintptr, r rect) {
	nbox(dc, r, 0x39291D)
	label := "停止"
	color := uint32(0x998877)
	if nativeReceiveMeter.Running {
		label = fmt.Sprintf("%.1fKB/s", nativeReceiveMeter.Rate/1024)
		color = 0x9AE08B
	}
	ncenter(dc, label, rect{r.Left + 66, r.Top + 6, r.Right - 5, r.Bottom}, color, 13)
	samples := nativeReceiveMeter.Samples
	if len(samples) < 2 {
		return
	}
	peak := 1.0
	for _, v := range samples {
		peak = max(peak, v)
	}
	pen, _, _ := gdi32.NewProc("CreatePen").Call(0, 2, uintptr(color))
	old, _, _ := pSelect.Call(dc, pen)
	for i, v := range samples {
		x := r.Left + 8 + int32(i)*50/int32(len(samples)-1)
		y := r.Bottom - 5 - int32(v/peak*float64(r.Bottom-r.Top-10))
		if i == 0 {
			gdi32.NewProc("MoveToEx").Call(dc, signed(x), signed(y), 0)
		} else {
			gdi32.NewProc("LineTo").Call(dc, signed(x), signed(y))
		}
	}
	pSelect.Call(dc, old)
	pDelete.Call(pen)
}
