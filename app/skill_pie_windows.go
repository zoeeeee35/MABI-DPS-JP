//go:build native && windows

package main

import (
	"fmt"
	"math"
	"unicode/utf16"
	"unsafe"
)

var nativePieSlices []SkillPieSlice
var nativePieCX, nativePieCY, nativePieRadius float64
var nativePieMouseX, nativePieMouseY int32
var nativePieMouseInside bool
var nativePieHoverKey string
var nativePieColors = []uint32{0xDFB34F, 0x72D8F6, 0xB295DC, 0xB6D46E, 0xB581F2, 0xE3C3A2, 0xB8A754, 0x8AACE8, 0xCFA793, 0xDA99C9}

func nativeSkillPie(dc uintptr, b *Battle, left, right, y, height int32) {
	nbox(dc, rect{left, y, right, y + height}, 0x261912)
	ntxt(dc, "スキル別ダメージの割合（扇形にマウスを置くと詳細）", rect{left + 16, y + 12, right - 16, y + 39}, 0xE9E3D9, 15)
	nativePieSlices = skillPieData(nativeSoloRows(b))
	nativePieRadius = 0
	if len(nativePieSlices) == 0 {
		ntxt(dc, "ダメージを受信するとスキル別の割合を表示します", rect{left + 24, y + height/2, right - 24, y + height/2 + 32}, 0xAF997D, 13)
		return
	}
	radius := float64(min(int32(105), (height-62)/2))
	cx, cy := float64(left+145), float64(y+height/2+16)
	nativePieCX, nativePieCY, nativePieRadius = cx, cy, radius
	for i, s := range nativePieSlices {
		points := []point{{int32(cx), int32(cy)}}
		steps := max(2, int(math.Ceil((s.End-s.Start)*180)))
		for k := 0; k <= steps; k++ {
			angle := (s.Start+(s.End-s.Start)*float64(k)/float64(steps))*2*math.Pi - math.Pi/2
			points = append(points, point{int32(cx + radius*math.Cos(angle)), int32(cy + radius*math.Sin(angle))})
		}
		color := nativePieColors[i%len(nativePieColors)]
		brush, _, _ := pCreateBrush.Call(uintptr(color))
		old, _, _ := pSelect.Call(dc, brush)
		pen, _, _ := gdi32.NewProc("CreatePen").Call(0, 1, 0x261912)
		oldPen, _, _ := pSelect.Call(dc, pen)
		gdi32.NewProc("Polygon").Call(dc, uintptr(unsafe.Pointer(&points[0])), uintptr(len(points)))
		pSelect.Call(dc, old)
		pSelect.Call(dc, oldPen)
		pDelete.Call(brush)
		pDelete.Call(pen)
	}
	legendX := left + 285
	limit := min(8, len(nativePieSlices))
	for i := 0; i < limit; i++ {
		s := nativePieSlices[i]
		yy := y + 48 + int32(i*27)
		fill(dc, rect{legendX, yy + 5, legendX + 12, yy + 17}, nativePieColors[i%len(nativePieColors)])
		ntxt(dc, s.Name, rect{legendX + 22, yy, right - 175, yy + 25}, 0xE9E3D9, 13)
		ntxt(dc, fmt.Sprintf("%s / %.1f%%", formatNum(float64(s.Damage)), s.Share*100), rect{right - 165, yy, right - 12, yy + 25}, 0xADCBA6, 12)
	}
	if len(nativePieSlices) > limit {
		ntxt(dc, fmt.Sprintf("ほか%d項目も円グラフに含みます", len(nativePieSlices)-limit), rect{legendX, y + height - 27, right - 16, y + height}, 0xAF997D, 11)
	}
}
func nativePieMove(window uintptr, x, y int32) {
	oldX, oldY := nativePieMouseX, nativePieMouseY
	nativePieMouseX, nativePieMouseY = x, y
	nativePieMouseInside = true
	if nativeSettingHover != "" && nativeSettingHover != nativeSettingHelpAt(x, y) {
		nativeSettingHover = ""
		invalidate(window)
	}
	key := ""
	if nativePage == "分析" && !nativeAdaptersOpen && !nativeHistoryOpen && !nativeSkillTargetsOpen {
		if i := skillPieHit(nativePieSlices, float64(x), float64(y), nativePieCX, nativePieCY, nativePieRadius); i >= 0 && y >= nativeBodyTop && y < nativeBottom {
			key = nativePieSlices[i].Key
		}
	}
	if key != nativePieHoverKey || (key != "" && (abs32(x-oldX) > 6 || abs32(y-oldY) > 6)) {
		nativePieHoverKey = key
		invalidate(window)
	}
	tracking := struct {
		Size, Flags uint32
		Window      uintptr
		Hover       uint32
	}{Flags: 3, Window: window, Hover: 500}
	tracking.Size = uint32(unsafe.Sizeof(tracking))
	user32.NewProc("TrackMouseEvent").Call(uintptr(unsafe.Pointer(&tracking)))
}
func abs32(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}
func nativeTextWidth(dc uintptr, text string) int32 {
	u := utf16.Encode([]rune(text))
	if len(u) == 0 {
		return 0
	}
	var size struct{ X, Y int32 }
	gdi32.NewProc("GetTextExtentPoint32W").Call(dc, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)), uintptr(unsafe.Pointer(&size)))
	return size.X
}
func nativeWrapTooltip(dc uintptr, text string, width int32) []string {
	lines := []string{}
	line := ""
	for _, r := range text {
		if r == '\n' {
			lines = append(lines, line)
			line = ""
			continue
		}
		next := line + string(r)
		if line != "" && nativeTextWidth(dc, next) > width {
			lines = append(lines, line)
			line = string(r)
		} else {
			line = next
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
func nativePieTooltip(dc uintptr, width, height int32) {
	if nativePage != "分析" || !nativePieMouseInside || nativeAdaptersOpen || nativeHistoryOpen || nativeSkillTargetsOpen {
		return
	}
	i := skillPieHit(nativePieSlices, float64(nativePieMouseX), float64(nativePieMouseY), nativePieCX, nativePieCY, nativePieRadius)
	if i < 0 || nativePieMouseY < nativeBodyTop || nativePieMouseY >= nativeBottom {
		return
	}
	s := nativePieSlices[i]
	popupWidth := min(int32(540), width-32)
	font := nf(14)
	old, _, _ := pSelect.Call(dc, font)
	lines := nativeWrapTooltip(dc, s.Name, popupWidth-28)
	pSelect.Call(dc, old)
	lines = append(lines, fmt.Sprintf("ダメージ %s　割合 %.2f%%", formatNum(float64(s.Damage)), s.Share*100))
	popupHeight := int32(22*len(lines) + 24)
	x := min(max(16, nativePieMouseX+18), width-popupWidth-16)
	y := min(max(nativeBodyTop, nativePieMouseY+18), height-popupHeight-16)
	y = max(16, y)
	nbox(dc, rect{x + 3, y + 3, x + popupWidth + 3, y + popupHeight + 3}, 0x100C09)
	nbox(dc, rect{x, y, x + popupWidth, y + popupHeight}, 0x493C2B)
	for j, line := range lines {
		ntxt(dc, line, rect{x + 14, y + 12 + int32(j*22), x + popupWidth - 14, y + 34 + int32(j*22)}, 0xFFFFFF, 14)
	}
}
