//go:build native && windows

package main

import (
	"fmt"
	"time"
	"unsafe"
)

const overlayResourceGap = 54
const fantasyOverlayHeight = 400 - overlayResourceGap + overlaySkillTableHeight

var nativeOverlayHover = -1
var skinJPFonts = map[int32]uintptr{}

func skinLabel(dc uintptr, text string, r rect, color uint32, size int32, center bool) {
	font := skinJPFonts[size]
	if font == 0 {
		font, _, _ = pFont.Call(signed(-size), 0, 0, 0, 700, 0, 0, 0, 128, 0, 0, 5, 0, uintptr(unsafe.Pointer(u16("Rounded Mplus 1c Bold"))))
		skinJPFonts[size] = font
	}
	flags := uintptr(0x8020)
	if center {
		flags = 0x8021
	}
	draw(dc, text, r, color, font, flags)
}
func fantasyButton(dc uintptr, r rect, label, icon string, selected bool, kind string, index int) {
	top, bottom := uint32(0x44372C), uint32(0x30261F)
	text := uint32(0xECE9E5)
	if kind == "reset" {
		top, bottom = 0x63405B, 0x43263D
	}
	if kind == "timer" {
		top, bottom = 0x765336, 0x4F3524
	}
	if selected {
		top, bottom = 0x7DCFFF, 0x35A7E8
		text = 0x302417
	}
	if nativeOverlayHover == index {
		top += 0x080808
	}
	fantasyCard(dc, r, top, bottom, 20)
	if icon != "" {
		skinIcon(dc, icon, r.Left+9, r.Top+(r.Bottom-r.Top-11)/2, 11, text)
		skinLabel(dc, label, rect{r.Left + 24, r.Top + 7, r.Right - 3, r.Bottom}, text, 11, false)
	} else {
		skinLabel(dc, label, rect{r.Left + 2, r.Top + 7, r.Right - 2, r.Bottom}, text, 12, true)
	}
}
func fantasyOverlayMove(window uintptr, x, y int32) {
	var r rect
	pGetClient.Call(window, uintptr(unsafe.Pointer(&r)))
	if r.Right <= 0 || r.Bottom <= 0 {
		return
	}
	x = x * 360 / r.Right
	y = y*fantasyOverlayHeight/r.Bottom + overlayResourceGap
	oldSkillHover := overlaySkillHover
	overlaySkillHover = -1
	if y >= overlaySkillTableTop+28 && y < overlaySkillTableTop+28+23*overlaySkillVisibleRows && x >= 19 && x < 341 {
		overlaySkillHover = int((y - overlaySkillTableTop - 28) / 23)
	}
	if oldSkillHover != overlaySkillHover {
		invalidate(window)
	}
	y = overlayControlY(y)
	index := -1
	if y >= 305 && y < 329 {
		for i := 0; i < 4; i++ {
			left := int32(18 + i*82)
			if x >= left && x < left+76 {
				index = i
			}
		}
	}
	if y >= 340 && y < 373 {
		if x >= 18 && x < 110 {
			index = 4
		}
		for i := 0; i < 3; i++ {
			left := int32(116 + i*46)
			if x >= left && x < left+42 {
				index = 5 + i
			}
		}
		if x >= 254 && x < 342 {
			index = 8
		}
	}
	if nativeOverlayHover != index {
		nativeOverlayHover = index
		invalidate(window)
	}
	track := struct {
		Size, Flags uint32
		Window      uintptr
		Hover       uint32
	}{Flags: 2, Window: window}
	track.Size = uint32(unsafe.Sizeof(track))
	user32.NewProc("TrackMouseEvent").Call(uintptr(unsafe.Pointer(&track)))
}
func paintFantasyOverlay(window, dc uintptr) {
	var bounds rect
	pGetClient.Call(window, uintptr(unsafe.Pointer(&bounds)))
	saved, _, _ := gdi32.NewProc("SaveDC").Call(dc)
	defer gdi32.NewProc("RestoreDC").Call(dc, saved)
	gdi32.NewProc("SetMapMode").Call(dc, 8)
	gdi32.NewProc("SetWindowExtEx").Call(dc, 360, uintptr(fantasyOverlayHeight), 0)
	gdi32.NewProc("SetViewportExtEx").Call(dc, uintptr(bounds.Right), uintptr(bounds.Bottom), 0)
	fill(dc, rect{0, 0, 360, fantasyOverlayHeight}, 0x211911)
	fantasyCard(dc, rect{3, 3, 357, fantasyOverlayHeight - 2}, 0x2E251D, 0x211911, 25)
	name := "自分のIDを選択してください"
	if e, ok := soloIdentity(); ok {
		name = e.Class + " " + entityDisplayID(e)
	}
	countdown, countdownVisible := barrierCountdown(time.Now(), barrierConfig)
	nameRight := int32(345)
	if countdownVisible {
		nameRight = 252
	}
	// Keep class/ID and target on separate rows; never replace either with the timer.
	skinLabel(dc, name, rect{16, 8, nameRight, 26}, 0xF4F2EE, 12, false)
	skinLabel(dc, targetFilterText(), rect{16, 27, 345, 43}, 0xB9B7B1, 10, false)
	if countdownVisible {
		drawBarrierBadge(dc, 251, 7, 20)
		label := countdown
		if label != barrierStartedLabel {
			label = "結界 " + label
		}
		skinLabel(dc, label, rect{273, 10, 349, 31}, 0x72C9FF, 11, false)
	}
	gdi32.NewProc("SetWindowOrgEx").Call(dc, 0, overlayResourceGap, 0)
	current := displayBattle()
	dps, damage, elapsed, hits := "—", "—", "00:00", "—"
	elapsedLabel := "戦闘時間"
	if displayCaptureTotal {
		elapsedLabel = "経過時間"
	}
	if current != nil {
		dps = formatNum(float64(current.Damage) / max(1, current.Duration))
		damage = formatNum(float64(current.Damage))
		elapsed = duration(current.Duration)
		n := 0
		for _, p := range current.Players {
			n += p.Hits
		}
		hits = fmt.Sprint(n)
	}
	fantasyCard(dc, rect{12, 105, 348, 173}, 0x42372D, 0x30271F, 20)
	dpsLabel := "DPS"
	if soloMeasurement != nil || (historyIndex >= 0 && historyIndex < len(history) && history[historyIndex].Archive.Request.Window != nil) {
		elapsedLabel = "計測時間"
	}
	waitingIdentity := soloMeasurement != nil && soloMeasurement.Party == nil
	if waitingIdentity {
		dpsLabel = "自分のID判定待ち"
		dps = "—"
		damage = "—"
		hits = "—"
	}
	preparing := soloPreparationRemaining(time.Now())
	if preparing > 0 {
		dpsLabel = "計測開始まで"
		dps = fmt.Sprintf("%d", preparing)
		damage = "0"
		elapsed = "00:00"
		hits = "0"
	}
	skinLabel(dc, dpsLabel, rect{20, 111, 340, 131}, 0xBBB5AA, 11, true)
	overlayFitText(dc, dps, rect{20, 131, 340, 170}, 0xA7E5FF, 32, 15)
	fantasyCard(dc, rect{12, 180, 348, 232}, 0x3C3027, 0x2C241D, 18)
	for i, v := range []struct {
		label, value, icon string
		color              uint32
	}{{"累計ダメージ", damage, "sword", 0xF1B6D1}, {elapsedLabel, elapsed, "clock", 0xD7E87A}, {"ヒット数", hits, "hit", 0xF1B6D1}} {
		x := []int32{23, 145, 224}[i]
		width := []int32{111, 70, 48}[i]
		skinIcon(dc, v.icon, x, 185, 10, 0xB6B0A5)
		skinLabel(dc, v.label, rect{x + 15, 183, x + width + 15, 198}, 0xC2B8AC, 9, false)
		overlayFitText(dc, v.value, rect{x, 201, x + width, 228}, v.color, 19, 12)
	}
	soloMiniChart(dc, current, rect{281, 188, 338, 224})
	fantasyCard(dc, rect{12, 239, 348, 295}, 0x3C3027, 0x2B241D, 18)
	var events []Hit
	if current != nil {
		events = current.DamageEvents
	}
	flags := damageFlagStatistics(events, "")
	for i, label := range []string{"クリティカル", "強打", "連打", "追加攻撃"} {
		x := int32(20 + i*82)
		value := "—"
		if i < 3 && flags.Observed > 0 {
			n := []int{flags.Critical, flags.Strong, flags.Combo}[i]
			value = fmt.Sprintf("%.1f%%", 100*float64(n)/float64(flags.Total))
		}
		if i == 3 {
			if rate, ok := flags.additionalRate(); ok {
				value = fmt.Sprintf("%.1f%%", 100*rate)
			}
		}
		if i == 0 && current != nil && !isProvisionalDisplay() && len(current.Players) > 0 {
			value = fmt.Sprintf("%.1f%%", current.Players[0].CritRate*100)
		}
		if i > 0 {
			fill(dc, rect{x - 5, 250, x - 4, 284}, 0x4C3D30)
		}
		color := []uint32{0x7580F0, 0x7EB4F1, 0xF8B7A8, 0xC8BDAF}[i]
		overlayFitText(dc, value, rect{x, 244, x + 76, 270}, color, 19, 12)
		skinLabel(dc, label, rect{x - 2, 275, x + 77, 292}, 0xC5BCB0, 10, true)
	}
	paintOverlaySkillBars(dc, current)
	gdi32.NewProc("SetWindowOrgEx").Call(dc, 0, signed(overlayResourceGap-overlaySkillTableHeight), 0)
	for i, label := range []string{"現在戦闘", "記録全体", "スキル", "再判定"} {
		x := int32(18 + i*82)
		selected := (i == 0 && !displayCaptureTotal) || (i == 1 && displayCaptureTotal)
		fantasyButton(dc, rect{x, 305, x + 76, 329}, label, []string{"chart", "file", "skill", "reset"}[i], selected, "", i)
	}
	fantasyButton(dc, rect{18, 340, 110, 373}, "初期化", "reset", false, "reset", 4)
	for i := 1; i <= 3; i++ {
		x := int32(116 + (i-1)*46)
		fantasyButton(dc, rect{x, 340, x + 42, 373}, fmt.Sprintf("%d分", i), "", i == soloTimerMinutes, "", 4+i)
	}
	timer := "開始"
	if !soloTimerEnd.IsZero() {
		remaining := time.Until(soloTimerEnd)
		if remaining <= 0 {
			timer = "終了 / 再開"
		} else {
			timer = duration(remaining.Seconds()) + " ■"
		}
	}
	if preparing > 0 {
		timer = "中止"
	}
	fantasyButton(dc, rect{254, 340, 342, 373}, timer, "", false, "timer", 8)
	footer := "自己計測 / 解析"
	if historyIndex >= 0 {
		footer = "履歴表示中 / 現在戦闘でライブへ戻る"
	}
	if soloMeasurement != nil {
		footer = "時間指定計測中 / 終了時に自動保存"
	}
	if waitingIdentity {
		footer = "攻撃後に自分を判定 / 判定後に区間全体を集計"
	}
	if preparing > 0 {
		footer = fmt.Sprintf("開始まで %d秒 / 中止ボタンでキャンセル", preparing)
	}
	if soloMeasurement == nil && soloMeasurementNotice != "" {
		footer = "開始できません：メイン画面の取得設定を確認"
	}
	skinLabel(dc, footer, rect{12, 382, 348, 399}, 0xA79D91, 9, true)
	gdi32.NewProc("SetWindowOrgEx").Call(dc, 0, overlayResourceGap, 0)
	paintOverlaySkillTooltip(dc)
}
