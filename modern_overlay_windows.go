//go:build modernpreview && native && windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

func windowsDwm() *syscall.LazyProc {
	return syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
}
func modernResizeOverlay() {
	if overlayUserSized {
		return
	}
	n := 3
	if b := displayBattle(); b != nil {
		n = max(1, min(8, len(b.Players)))
	}
	h := int32(164 + 58*n)
	if modernOverlayMode == "compact" {
		h = int32(115 + 36*n)
	}
	pSetWindowPos.Call(overlayWindow, 0, 0, 0, 460, uintptr(max(210, h)), 0x0016)
}
func paintModernOverlay(window, dc uintptr) {
	var r rect
	pGetClient.Call(window, uintptr(unsafe.Pointer(&r)))
	fill(dc, r, 0x20170F)
	draw(dc, "MABI DPS JP", rect{18, 12, r.Right - 18, 38}, 0xF5EDE5, overlayBold, 0x8020)
	label := "DPS · 実通信"
	if !isProvisionalDisplay() {
		label = sourceLabel()
	}
	draw(dc, label, rect{18, 40, r.Right - 18, 62}, 0x77C9F5, overlayFont, 0x8020)
	b := displayBattle()
	if b == nil {
		draw(dc, "戦闘データを待っています", rect{18, 88, r.Right - 18, 120}, 0xB8A492, overlayFont, 0x8020)
		return
	}
	draw(dc, fmt.Sprintf("%s   /   合計 %s", duration(b.Duration), formatNum(float64(b.Damage))), rect{18, 68, r.Right - 18, 92}, 0xD5C1A9, overlayFont, 0x8020)
	compact := modernOverlayMode == "compact"
	step := int32(58)
	if compact {
		step = 36
	}
	y := int32(101)
	for i, p := range b.Players {
		if i >= 8 || y+step > r.Bottom-24 {
			break
		}
		fill(dc, rect{12, y, r.Right - 12, y + step - 6}, 0x30251B)
		fill(dc, rect{12, y, 12 + int32(float64(r.Right-24)*p.Share), y + step - 6}, 0x4A3C21)
		draw(dc, fmt.Sprintf("%d  %s", i+1, p.Name), rect{22, y + 4, r.Right - 145, y + 28}, 0xF3EDE6, overlayFont, 0x8020)
		draw(dc, formatNum(p.DPS), rect{r.Right - 143, y + 3, r.Right - 22, y + 29}, 0xACDC75, overlayBold, 0x8022)
		if !compact {
			draw(dc, fmt.Sprintf("累計 %s   ·   %.1f%%", formatNum(float64(p.Damage)), p.Share*100), rect{22, y + 29, r.Right - 22, y + 48}, 0xBDA58F, overlayFont, 0x8020)
		}
		y += step
	}
	tip := "ドラッグで移動 · 端でサイズ変更"
	if clickThrough {
		tip = "クリック透過中：メイン画面で解除"
	}
	draw(dc, tip, rect{18, r.Bottom - 23, r.Right - 18, r.Bottom - 3}, 0xA08A74, overlayFont, 0x8020)
}

