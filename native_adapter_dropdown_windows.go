//go:build native && windows

package main

import (
	"fmt"
)

var nativeAdaptersOpen bool
var nativeAdapterOffset int
var nativeAdapterVisible int
var nativeAdapterFieldBounds, nativeAdapterPopupBounds rect

func nativeAdapterField(dc uintptr, r rect) {
	nativeAdapterFieldBounds = r
	nbox(dc, r, 0x39291D)
	ntxt(dc, nativeAdapterLabel(), rect{r.Left + 12, r.Top + 6, r.Right - 36, r.Bottom}, 0xF5F5F5, 13)
	ncenter(dc, "▼", rect{r.Right - 32, r.Top + 5, r.Right - 4, r.Bottom}, 0xF5F5F5, 13)
	nativeHits = append(nativeHits, nativeHit{r, "adapters", 0})
}
func insideNativeRect(x, y int32, r rect) bool {
	return x >= r.Left && x < r.Right && y >= r.Top && y < r.Bottom
}
func nativeAdapterDropdown(dc uintptr, bottom int32) {
	top := nativeAdapterFieldBounds.Bottom + 5
	nativeAdapterVisible = max(1, min(len(captureAdapterList), int((bottom-top-44)/42)))
	nativeAdapterOffset = max(0, min(max(0, len(captureAdapterList)-nativeAdapterVisible), nativeAdapterOffset))
	r := rect{nativeAdapterFieldBounds.Left, top, nativeAdapterFieldBounds.Right, top + 40 + int32(max(1, nativeAdapterVisible)*42)}
	nativeAdapterPopupBounds = r
	nbox(dc, r, 0x302219)
	nativeHits = append(nativeHits, nativeHit{r, "adapter-noop", 0})
	title := "アダプターを選択（スクロール可）"
	running := captureCtl.running()
	if running {
		title = "取得中：停止すると変更できます"
	}
	ntxt(dc, title, rect{r.Left + 12, top + 7, r.Right - 10, top + 33}, 0xDDD4C4, 14)
	if len(captureAdapterList) == 0 {
		ntxt(dc, "利用可能なアダプターがありません", rect{r.Left + 12, top + 46, r.Right - 10, r.Bottom}, 0xDDD4C4, 14)
	}
	selected := int(send(controls[idCaptureAdapters], 0x0147, 0, 0))
	for n := 0; n < nativeAdapterVisible && nativeAdapterOffset+n < len(captureAdapterList); n++ {
		i := nativeAdapterOffset + n
		yy := top + 36 + int32(n*42)
		row := rect{r.Left + 6, yy, r.Right - 6, yy + 38}
		color := uint32(0x39291D)
		if i == selected {
			color = 0x514231
		}
		nbox(dc, row, color)
		label := captureAdapterList[i].Label
		if i == selected {
			label = "✓ " + label
		}
		textColor := uint32(0xF5F5F5)
		if running {
			textColor = 0x9D9285
		}
		ntxt(dc, label, rect{row.Left + 10, yy + 8, row.Right - 6, yy + 36}, textColor, 14)
		if !running {
			nativeHits = append(nativeHits, nativeHit{row, "adapter-select", i})
		}
	}
}
func nativeToggleButton(dc uintptr, r rect, label, action string, on bool, tint uint32) {
	state := "OFF"
	if on {
		state = "ON"
	}

	nativeToggleLabel(dc, r, fmt.Sprintf("%s %s", label, state), action, on, tint)
}

func nativeToggleLabel(dc uintptr, r rect, label, action string, on bool, tint uint32) {
	color := uint32(0x39291D)
	if on {
		color = tint
	}
	nbox(dc, r, color)
	nativeControlIcon(dc, action, r.Left+10, r.Top+9, 0xFFFFFF)
	ntxt(dc, label, rect{r.Left + 33, r.Top + 8, r.Right - 48, r.Bottom}, 0xFFFFFF, 13)
	track := rect{r.Right - 44, r.Top + 11, r.Right - 10, r.Top + 25}
	trackColor := uint32(0x796D60)
	if on {
		trackColor = 0x95D46E
	}
	nbox(dc, track, trackColor)
	x := track.Left + 2
	if on {
		x = track.Right - 13
	}
	brush, _, _ := pCreateBrush.Call(0xFFFFFF)
	old, _, _ := pSelect.Call(dc, brush)
	nullPen, _, _ := gdi32.NewProc("GetStockObject").Call(8)
	oldPen, _, _ := pSelect.Call(dc, nullPen)
	gdi32.NewProc("Ellipse").Call(dc, signed(x), signed(track.Top+1), signed(x+11), signed(track.Bottom-1))
	pSelect.Call(dc, oldPen)
	pSelect.Call(dc, old)
	pDelete.Call(brush)
	hit := r
	if nativeBody {
		hit.Top = max(hit.Top, 208)
		hit.Bottom = min(hit.Bottom, nativeBottom)
	}
	if hit.Bottom > hit.Top {
		nativeHits = append(nativeHits, nativeHit{hit, action, 0})
	}
}

func drawNativeChoice(dc uintptr, r rect, labels, actions [2]string, active int) {
	radius := r.Bottom - r.Top
	fantasyCard(dc, r, 0x46403B, 0x2D2926, radius)
	mid := (r.Left + r.Right) / 2
	for i, hit := range [2]rect{{r.Left, r.Top, mid, r.Bottom}, {mid, r.Top, r.Right, r.Bottom}} {
		color := uint32(0xD3CAC0)
		if i == active {
			pill := rect{hit.Left + 3, hit.Top + 3, hit.Right - 3, hit.Bottom - 3}
			fantasyCard(dc, pill, 0x507B32, 0x385727, pill.Bottom-pill.Top)
			color = 0xFFFFFF
		}
		draw(dc, labels[i], hit, color, nf(13), 0x8025)
		if nativeBody {
			hit.Top = max(hit.Top, 208)
			hit.Bottom = min(hit.Bottom, nativeBottom)
		}
		if hit.Bottom > hit.Top {
			nativeHits = append(nativeHits, nativeHit{hit, actions[i], 0})
		}
	}
}
