//go:build native && windows

package main

import (
	"fmt"
	"hash/fnv"
	"time"
	"unsafe"
)

var overlaySkillOffset int
var overlaySkillHover = -1
var overlaySkillCache []overlaySkillBar
var overlaySkillCacheAt time.Time

func paintOverlaySkillBars(dc uintptr, b *Battle) {
	if time.Since(overlaySkillCacheAt) >= 250*time.Millisecond {
		overlaySkillCache = overlaySkillBars(nativeSoloRows(b))
		overlaySkillCacheAt = time.Now()
	}
	rows := overlaySkillCache
	overlaySkillOffset = max(0, min(overlaySkillOffset, max(0, len(rows)-overlaySkillVisibleRows)))
	y := overlaySkillTableTop
	fantasyCard(dc, rect{12, y, 348, y + overlaySkillTableHeight - 8}, 0x3C3027, 0x2B241D, 14)
	skinLabel(dc, "スキル", rect{21, y + 6, 183, y + 25}, 0xC5BCB0, 10, false)
	skinLabel(dc, "ヒット", rect{183, y + 6, 218, y + 25}, 0xC5BCB0, 9, true)
	skinLabel(dc, "ダメージ", rect{218, y + 6, 292, y + 25}, 0xC5BCB0, 9, true)
	skinLabel(dc, "割合", rect{292, y + 6, 340, y + 25}, 0xC5BCB0, 9, true)
	if len(rows) == 0 {
		skinLabel(dc, "攻撃するとスキル別ダメージを表示", rect{20, y + 65, 340, y + 95}, 0xC5BCB0, 11, true)
		return
	}
	for i := 0; i < overlaySkillVisibleRows && i+overlaySkillOffset < len(rows); i++ {
		r := rows[i+overlaySkillOffset]
		top := y + 28 + int32(i)*23
		fill(dc, rect{19, top, 341, top + 21}, 0x2C241D)
		h := fnv.New32a()
		_, _ = h.Write([]byte(r.Key))
		color := nativePieColors[int(h.Sum32())%len(nativePieColors)]
		// Darker bars keep the white skill names and values readable.
		color = (color >> 1) & 0x7F7F7F
		fill(dc, rect{19, top, 19 + int32(322*r.Width), top + 21}, color)
		if i == overlaySkillHover {
			fill(dc, rect{19, top, 21, top + 21}, 0xF4F2EE)
		}
		skinLabel(dc, r.Name, rect{24, top + 3, 182, top + 20}, 0xF4F2EE, 11, false)
		skinLabel(dc, fmt.Sprint(r.Hits), rect{184, top + 3, 216, top + 20}, 0xF4F2EE, 10, true)
		skinLabel(dc, formatNum(float64(r.Damage)), rect{216, top + 3, 291, top + 20}, 0xF4F2EE, 10, true)
		skinLabel(dc, fmt.Sprintf("%.1f%%", r.Share*100), rect{292, top + 3, 339, top + 20}, 0xF4F2EE, 10, true)
	}
	if len(rows) > overlaySkillVisibleRows {
		skinLabel(dc, fmt.Sprintf("%d–%d / %d　ホイールでスクロール", overlaySkillOffset+1, min(len(rows), overlaySkillOffset+overlaySkillVisibleRows), len(rows)), rect{20, y + 166, 340, y + 180}, 0xC5BCB0, 8, true)
	}
}
func overlaySkillsScroll(window uintptr, w, l uintptr) bool {
	var bounds rect
	pGetClient.Call(window, uintptr(unsafe.Pointer(&bounds)))
	if bounds.Right <= 0 || bounds.Bottom <= 0 {
		return false
	}
	p := point{int32(int16(l & 0xffff)), int32(int16(l >> 16))}
	user32.NewProc("ScreenToClient").Call(window, uintptr(unsafe.Pointer(&p)))
	y := p.Y*fantasyOverlayHeight/bounds.Bottom + overlayResourceGap
	if y < overlaySkillTableTop || y >= overlaySkillTableTop+overlaySkillTableHeight {
		return false
	}
	delta := int(int16(w >> 16))
	if delta == 0 {
		return true
	}
	if delta > 0 {
		overlaySkillOffset -= 3
	} else {
		overlaySkillOffset += 3
	}
	overlaySkillOffset = max(0, min(overlaySkillOffset, max(0, len(overlaySkillCache)-overlaySkillVisibleRows)))
	invalidate(window)
	return true
}

func paintOverlaySkillTooltip(dc uintptr) {
	index := overlaySkillOffset + overlaySkillHover
	if overlaySkillHover < 0 || index >= len(overlaySkillCache) {
		return
	}
	name := overlaySkillCache[index].Name
	top := overlaySkillTableTop - 42
	fantasyCard(dc, rect{15, top, 345, top + 40}, 0x493B30, 0x30271F, 12)
	draw(dc, name, rect{22, top + 4, 338, top + 37}, 0xF4F2EE, soloFont(11), 0x10|0x8000)
}
