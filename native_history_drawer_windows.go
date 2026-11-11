//go:build native && windows

package main

import (
	"fmt"
	"strings"
	"time"
)

var nativeHistoryOpen bool
var nativeHistoryOffset int
var nativeHistoryPageSize = 1
var nativeHistoryBounds rect

func nativeHistoryDrawer(dc uintptr, left, right, bottom int32) {
	r := rect{left, 208, min(right, left+360), bottom}
	nativeHistoryBounds = r
	fill(dc, rect{r.Left + 4, r.Top + 4, r.Right + 4, r.Bottom}, 0x100A06)
	nbox(dc, r, 0x302219)
	nativeHits = append(nativeHits, nativeHit{r, "history-noop", 0})
	ntxt(dc, "履歴（新しい順）", rect{r.Left + 14, 220, r.Right - 88, 248}, 0xEEE7E1, 16)
	nbutton(dc, rect{r.Right - 80, 217, r.Right - 12, 250}, "閉じる", "history-toggle", false, 0)
	nativeHistoryPageSize = max(1, int((bottom-310)/104))
	nativeHistoryOffset = max(0, min(nativeHistoryOffset, max(0, len(history)-nativeHistoryPageSize)))
	end := min(len(history), nativeHistoryOffset+nativeHistoryPageSize)
	for i := nativeHistoryOffset; i < end; i++ {
		h := history[i]
		yy := int32(262 + (i-nativeHistoryOffset)*104)
		card := rect{r.Left + 10, yy, r.Right - 10, yy + 94}
		color := uint32(0x39291D)
		if i == historyIndex {
			color = 0x283C52
		}
		nbox(dc, card, color)
		stamp, _ := historyRecordTime(h)
		date := "日時不明"
		if !stamp.IsZero() {
			date = stamp.In(time.FixedZone("JST", 9*3600)).Format("2006/01/02 15:04:05")
		}
		ntxt(dc, date, rect{card.Left + 10, yy + 5, card.Right - 6, yy + 27}, 0xEEE7E1, 14)
		ntxt(dc, fmt.Sprintf("%s / %.1f秒", historyClassLabel(h), h.Battle.Duration), rect{card.Left + 10, yy + 29, card.Right - 6, yy + 51}, 0xC8BBA9, 14)
		ntxt(dc, fmt.Sprintf("累計 %s / DPS %s", formatNum(float64(h.Battle.Damage)), formatNum(float64(h.Battle.Damage)/max(1, h.Battle.Duration))), rect{card.Left + 10, yy + 53, card.Right - 6, yy + 73}, 0xB0D656, 13)
		place := h.Archive.LocationNote
		if place == "" {
			place = "場所未取得"
		}
		if len([]rune(place)) > 20 {
			place = string([]rune(place)[:20]) + "…"
		}
		ntxt(dc, strings.TrimSpace(place), rect{card.Left + 10, yy + 73, card.Right - 6, yy + 93}, 0xA99882, 13)
		nativeHits = append(nativeHits, nativeHit{card, "history", i})
	}
	if len(history) == 0 {
		ntxt(dc, "保存した履歴はまだありません。", rect{r.Left + 14, 274, r.Right - 10, 320}, 0xC8BBA9, 14)
	}
	yy := bottom - 43
	nbutton(dc, rect{r.Left + 10, yy, r.Left + 72, yy + 32}, "前へ", "history-prev", false, 0)
	nbutton(dc, rect{r.Left + 79, yy, r.Left + 141, yy + 32}, "次へ", "history-next", false, 0)
	nbutton(dc, rect{r.Left + 148, yy, r.Right - 10, yy + 32}, "一覧・場所メモ", "history-full", false, 0)
}
