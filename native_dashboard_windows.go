//go:build native && windows

package main

import (
	"fmt"
	"math"

	"unsafe"
)

var nativeModern bool
var nativePage = "分析"
var nativeScroll int32
var nativeFonts = map[int32]uintptr{}

type nativeHit struct {
	R      rect
	Action string
	Index  int
}

var nativeHits []nativeHit
var nativeBody bool
var nativeBottom int32
var nativeMaxScroll int32

func nf(size int32) uintptr {
	if nativeFonts[size] == 0 {
		weight := uintptr(700)
		if size >= 17 {
			weight = 700
		}
		face := u16("Rounded Mplus 1c Bold")
		h, _, _ := pFont.Call(signed(-max(14, size)), 0, 0, 0, weight, 0, 0, 0, 128, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
		nativeFonts[size] = h
	}
	return nativeFonts[size]
}
func ntxt(dc uintptr, text string, r rect, color uint32, size int32) {
	draw(dc, text, r, color, nf(size), 0x8020)
}
func ncenter(dc uintptr, text string, r rect, color uint32, size int32) {
	draw(dc, text, r, color, nf(size), 0x8021)
}
func nbox(dc uintptr, r rect, color uint32) {
	b, _, _ := pCreateBrush.Call(uintptr(color))
	old, _, _ := pSelect.Call(dc, b)
	pen, _, _ := gdi32.NewProc("CreatePen").Call(0, 1, 0x4B3C30)
	op, _, _ := pSelect.Call(dc, pen)
	gdi32.NewProc("RoundRect").Call(dc, signed(r.Left), signed(r.Top), signed(r.Right), signed(r.Bottom), 12, 12)
	pSelect.Call(dc, old)
	pSelect.Call(dc, op)
	pDelete.Call(b)
	pDelete.Call(pen)
}
func nbutton(dc uintptr, r rect, label, action string, active bool, index int) {
	color := uint32(0x39291D)
	if active {
		color = 0x283C52
	}
	switch action {
	case "sample":
		color = 0xA15F2B
	case "save":
		color = 0x49412A
	case "capture":
		if active {
			color = 0x2847B7
		}
	}
	nbox(dc, r, color)
	if action == "settings" && label == "設定" {
		old, _, _ := pSelect.Call(dc, nf(13))
		var size point
		text := u16(label)
		gdi32.NewProc("GetTextExtentPoint32W").Call(dc, uintptr(unsafe.Pointer(text)), 2, uintptr(unsafe.Pointer(&size)))
		pSelect.Call(dc, old)
		left := r.Left + (r.Right-r.Left-(16+6+size.X))/2
		nativeControlIcon(dc, "settings", left, r.Top+(r.Bottom-r.Top-16)/2, 0xE9E2D7)
		draw(dc, label, rect{left + 22, r.Top, left + 22 + size.X + 1, r.Bottom}, 0xE9E2D7, nf(13), 0x8025)
	} else if action == "capture" {
		draw(dc, label, r, 0xFFFFFF, nf(13), 0x8025)
	} else {
		ntxt(dc, label, rect{r.Left + 12, r.Top + 8, r.Right - 6, r.Bottom}, 0xE9E2D7, 13)
	}
	hit := r
	if nativeBody {
		hit.Top = max(hit.Top, 208)
		hit.Bottom = min(hit.Bottom, nativeBottom)
		if hit.Bottom <= hit.Top {
			return
		}
	}
	nativeHits = append(nativeHits, nativeHit{hit, action, index})
}
func nativeLayout(window, dc uintptr) {
	var r rect
	pGetClient.Call(window, uintptr(unsafe.Pointer(&r)))
	nativeHits = nil
	nativePieSlices = nil
	nativePieRadius = 0
	nativeBody = false
	defer func() { nativeBody = false }()
	nativeBottom = r.Bottom - 18
	fill(dc, r, 0x140B07)
	// Two compact connection rows stay visible while the analysis body scrolls.
	fill(dc, rect{0, 0, r.Right, 43}, 0x211510)
	ntxt(dc, "MABI DPS JP", rect{14, 10, 140, 37}, 0x42C4F5, 17)
	ntxt(dc, "v"+appVersion(), rect{142, 13, 198, 37}, 0xE9E3D9, 11)

	nbutton(dc, rect{r.Right - 94, 6, r.Right - 14, 37}, "設定", "settings", nativePage == "設定", 0)
	fill(dc, rect{0, 43, r.Right, 92}, 0x251A14)
	nativeAdapterField(dc, rect{14, 51, r.Right - 425, 83})
	nativeNetworkMeter(dc, rect{r.Right - 415, 51, r.Right - 230, 83})
	captureLabel := "キャプチャ開始"
	if captureCtl.running() {
		captureLabel = "キャプチャ停止"
	}
	nbutton(dc, rect{r.Right - 225, 51, r.Right - 14, 83}, captureLabel, "capture", captureCtl.running(), 0)
	left, right := int32(16), r.Right-18
	nbox(dc, rect{left, 101, left + 122, 134}, 0x283C52)
	nativeControlIcon(dc, "layers", left+9, 110, 0x42C4F5)
	ncenter(dc, "オーバーレイ", rect{left + 25, 109, left + 122, 134}, 0x42C4F5, 12)
	nativeToggleLabel(dc, rect{left + 130, 101, left + 317, 134}, "クリック透過", "click", overlayPassesClicks(), 0x40372D)
	nativeToggleLabel(dc, rect{left + 325, 101, left + 454, 134}, "表示", "overlay", overlayShown, 0xA95C7C)
	nativeToggleLabel(dc, rect{left + 462, 101, left + 622, 134}, "結界通知", "barrier-master", barrierConfig.Enabled, 0x685140)
	scope := 0
	if historyIndex >= 0 {
		scope = -1
	} else if displayCaptureTotal {
		scope = 1
	}
	mode := 0
	if gameOverlay.Enabled {
		mode = 1
	}
	drawNativeChoice(dc, rect{left + 630, 101, left + 785, 134}, [2]string{"現在戦闘", "記録全体"}, [2]string{"scope-current", "scope-all"}, scope)
	drawNativeChoice(dc, rect{left + 793, 101, left + 963, 134}, [2]string{"通常表示", "ミニ表示"}, [2]string{"overlay-regular", "overlay-game"}, mode)
	nbox(dc, rect{left, 147, right, r.Bottom - 12}, 0x251810)
	historyLabel := "履歴 ▸"
	if nativeHistoryOpen {
		historyLabel = "履歴 ▾"
	}
	nbutton(dc, rect{left + 12, 157, left + 110, 190}, historyLabel, "history-toggle", nativeHistoryOpen, 0)
	for i, name := range []string{"分析", "検出プレイヤー", "バフ・スキル", "オーバーレイ", "結界通知"} {
		x := left + 118 + int32(i*143)
		page := name
		if name == "バフ・スキル" {
			page = "スキル"
		}
		label := name
		if name == "攻撃対象" && currentTargetFilter() != "" {
			label = "攻撃対象 ●"
		}
		if name == "検出プレイヤー" {
			label = "自分の設定"
		}
		if name == "バフ・スキル" {
			label = "スキル"
		}
		nbutton(dc, rect{x, 157, x + 134, 190}, label, "page:"+page, nativePage == page, 0)
	}
	fill(dc, rect{left + 1, 200, right - 1, 201}, 0x4B382A)
	saved, _, _ := gdi32.NewProc("SaveDC").Call(dc)
	defer func() {
		gdi32.NewProc("RestoreDC").Call(dc, saved)
		nativeBody = false
		nativeScrollRail(dc, right, r.Bottom-18)
		if nativeSkillTargetsOpen && (nativePage == "スキル" || nativePage == "分析") {
			nativeSkillTargetDropdown(dc, r.Bottom-18)
		}
		nativePieTooltip(dc, r.Right, r.Bottom)
		nativeSettingTooltip(dc, r.Right, r.Bottom)
		if nativeHistoryOpen {
			nativeHistoryDrawer(dc, left+8, right-8, r.Bottom-22)
		}
		if nativeAdaptersOpen {
			nativeAdapterDropdown(dc, r.Bottom-22)
		}
	}()
	gdi32.NewProc("IntersectClipRect").Call(dc, signed(left+9), 208, signed(right-9), signed(r.Bottom-18))
	nativeBody = true
	x0, x1 := left+10, right-10
	var b *Battle
	if nativePage != "設定" && nativePage != "オーバーレイ" && nativePage != "結界通知" {
		b = displayBattle()
	}
	contentBottom := int32(650)
	switch nativePage {
	case "分析":
		contentBottom = 1444 + int32(max(1, len(nativeSoloRows(b)))*44)
	case "攻撃対象":
		contentBottom = 450 + int32(len(damageTargets(baseDisplayBattle(), soloLog()))*49)
	case "設定", "オーバーレイ", "結界通知":
		contentBottom = nativeSettingsBottom()
	case "履歴":
		contentBottom = 275 + int32(len(history)*78)
	case "アダプター選択":
		contentBottom = 280 + int32(len(captureAdapterList)*48)
	case "検出プレイヤー":
		contentBottom = 380 + int32(len(soloCandidates())*47)
	case "スキル":
		contentBottom = 531 + int32(max(1, len(nativeSoloRows(b)))*44)
	}
	nativeMaxScroll = max(0, contentBottom-nativeBottom)
	nativeScroll = min(nativeScroll, nativeMaxScroll)
	y := int32(213) - nativeScroll
	switch nativePage {
	case "攻撃対象":
		nativeTargetPanel(dc, x0, x1, y)
	case "分析":
		nbox(dc, rect{x0, y, x1, y + 47}, 0x281C15)
		for i, tool := range []struct{ label, action string }{{"初期化", "reset"}, {"戦闘結果を保存", "save"}, {"サンプル再生", "sample"}, {"スクリーンショット", "screenshot"}, {"取得設定", "settings"}} {
			xx := x0 + 10 + int32(i*160)
			nbutton(dc, rect{xx, y + 7, xx + 151, y + 39}, tool.label, tool.action, false, 0)
		}
		y += 57
		nbox(dc, rect{x0, y, x1, y + 43}, 0x241810)
		ntxt(dc, "対象：自分のみ   /   "+targetFilterText(), rect{x0 + 14, y + 12, x1 - 10, y + 38}, 0xAEB9D6, 12)
		y += 53
		width := (x1 - x0 - 20) / 3
		damage, dps, elapsed := "—", "—", "00:00.0"
		timeLabel := "実戦闘時間"
		if displayCaptureTotal && historyIndex < 0 {
			timeLabel = "記録の経過時間"
		}
		if b != nil {
			damage = formatNum(float64(b.Damage))
			dps = formatNum(float64(b.Damage) / math.Max(1, b.Duration))
			elapsed = duration(b.Duration)
		}
		for i, m := range []struct {
			label, value string
			color        uint32
		}{{"取得済み累計ダメージ", damage, 0xFFAA77}, {"取得済み分のDPS", dps, 0x40C4F7}, {timeLabel, elapsed, 0xF8F3EE}} {
			xx := x0 + int32(i)*(width+10)
			nbox(dc, rect{xx, y, xx + width, y + 125}, 0x261912)
			ncenter(dc, m.label, rect{xx + 18, y + 19, xx + width - 12, y + 44}, 0xBB9D7F, 13)
			soloValue(dc, m.value, rect{xx + 18, y + 58, xx + width - 12, y + 111}, m.color, 34)
		}
		y += 137
		nbox(dc, rect{x0, y, x1, y + 79}, 0x271A12)
		var flagEvents []Hit
		if b != nil {
			flagEvents = b.DamageEvents
		}
		fs := damageFlagStatistics(flagEvents, "")
		for i, label := range []string{"クリ率", "追加率", "強打率", "連打率"} {
			xx := x0 + int32(i)*(x1-x0)/4
			ncenter(dc, label, rect{xx + 20, y + 12, xx + (x1-x0)/4 - 8, y + 34}, 0xB5997D, 12)
			value := "未取得"
			if i == 1 {
				if rate, ok := fs.additionalRate(); ok {
					value = fmt.Sprintf("%d件 / %.1f%%", fs.Additional, 100*rate)
				}
			} else if fs.Observed > 0 {
				n := []int{fs.Critical, 0, fs.Strong, fs.Combo}[i]
				value = fmt.Sprintf("%d件 / %.1f%%", n, 100*float64(n)/float64(fs.Total))
			}
			ncenter(dc, value, rect{xx + 20, y + 42, xx + (x1-x0)/4 - 8, y + 69}, 0x9A816A, 16)
		}
		y += 92
		nativeSkillPie(dc, b, x0, x1, y, 300)
		y += 316
		nativeReferenceChart(dc, b, x0, x1, y, 230)
		y += 246
		nativeRank(dc, b, x0+5, x1-5, y)
		rows := 1
		if b != nil {
			rows = max(1, len(b.Players))
		}
		y += 48 + int32(rows*50) + 16
		nativeSkillPanel(dc, b, x0, x1, y)
		y += 196
		if !soloMode {
			nativeBuffPanel(dc, x0, x1, y)
		}
	case "設定", "オーバーレイ", "結界通知":
		nativeSettingsPanel(dc, x0, x1, y, mode)
	case "アダプター選択":
		ntxt(dc, "ネットワークアダプターを選択", rect{x0 + 12, y + 8, x1, y + 40}, 0xE5D6C5, 17)
		for i, d := range captureAdapterList {
			yy := y + 52 + int32(i*48)
			nbutton(dc, rect{x0 + 10, yy, x1 - 10, yy + 39}, d.Label, "adapter-select", false, i)
		}
	case "履歴":
		ntxt(dc, "履歴 / 記録日時・職業・計測時間（新しい順）", rect{x0 + 12, y + 8, x1, y + 40}, 0xDDC4A6, 16)
		for i, h := range history {
			yy := y + 50 + int32(i*78)
			r := rect{x0 + 10, yy, x1 - 112, yy + 68}
			color := uint32(0x39291D)
			if historyIndex == i {
				color = 0x283C52
			}
			nbox(dc, r, color)
			ntxt(dc, historyDisplayTitle(h), rect{r.Left + 12, yy + 8, r.Right - 8, yy + 33}, 0xE9E2D7, 15)
			ntxt(dc, historyDisplayDetail(h), rect{r.Left + 12, yy + 36, r.Right - 8, yy + 61}, 0xB9A184, 13)
			clipped := r
			clipped.Top = max(208, clipped.Top)
			clipped.Bottom = min(nativeBottom, clipped.Bottom)
			if clipped.Bottom > clipped.Top {
				nativeHits = append(nativeHits, nativeHit{clipped, "history", i})
			}
			nbutton(dc, rect{x1 - 102, yy + 17, x1 - 10, yy + 52}, "場所メモ", "history-note", false, i)
		}
	case "検出プレイヤー":
		nativeSoloSelectPanel(dc, x0, x1, y)
	case "スキル":
		nativeSkillPanel(dc, b, x0, x1, y)
		if !soloMode {
			nativeBuffPanel(dc, x0, x1, y+198)
		}
	case "判定詳細":
		nativeFlagPanel(dc, b, x0, x1, y)
		nbutton(dc, rect{x0 + 14, y + 380, x0 + 220, y + 420}, "詳細設定へ戻る", "settings-diagnostics", false, 0)

	}
}
func nativeReferenceChart(dc uintptr, b *Battle, left, right, y, height int32) {
	nbox(dc, rect{left, y, right, y + height}, 0x261912)
	ntxt(dc, "⌄  リアルタイムDPSグラフ（取得済みイベント）", rect{left + 15, y + 13, right - 12, y + 41}, 0xDFC9B0, 14)
	if b == nil || len(b.DamageEvents) == 0 {
		ntxt(dc, "戦闘が始まるとグラフを表示します", rect{left + 25, y + height/2, right - 15, y + height/2 + 30}, 0x937957, 12)
		return
	}
	var sum int64
	peak := 1.0
	for _, e := range b.DamageEvents {
		sum += e.Damage
		peak = math.Max(peak, float64(sum)/math.Max(1, float64(e.At-b.Start)/1000))
	}
	pen, _, _ := gdi32.NewProc("CreatePen").Call(0, 2, 0xB0D656)
	old, _, _ := pSelect.Call(dc, pen)
	sum = 0
	for i, e := range b.DamageEvents {
		sum += e.Damage
		if i%max(1, len(b.DamageEvents)/300) != 0 && i != len(b.DamageEvents)-1 {
			continue
		}
		sec := float64(e.At-b.Start) / 1000
		x := left + 20 + int32(sec/math.Max(1, b.Duration)*float64(right-left-40))
		yy := y + height - 24 - int32(float64(sum)/math.Max(1, sec)/peak*float64(height-82))
		if i == 0 {
			gdi32.NewProc("MoveToEx").Call(dc, signed(x), signed(yy), 0)
		} else {
			gdi32.NewProc("LineTo").Call(dc, signed(x), signed(yy))
		}
	}
	pSelect.Call(dc, old)
	pDelete.Call(pen)
}
func nativeSkillPanel(dc uintptr, b *Battle, left, right, y int32) {
	if soloMode {
		nativeSoloSkillPanel(dc, b, left, right, y)
		return
	}
	nbox(dc, rect{left, y, right, y + 180}, 0x261912)
	ntxt(dc, "⌄  スキル分析", rect{left + 16, y + 13, right - 14, y + 40}, 0xDFC9B0, 14)
	nbox(dc, rect{left + 14, y + 48, right - 14, y + 78}, 0x453125)
	ntxt(dc, "スキル                                      累計ダメージ         命中数          クリティカル", rect{left + 26, y + 54, right - 20, y + 76}, 0xD1B294, 11)
	if b == nil || isProvisionalDisplay() {
		ntxt(dc, "スキル別の実ダメージ対応は未特定です。職業推定とダメージの帰属は分けて扱います。", rect{left + 25, y + 106, right - 20, y + 143}, 0x9D8264, 12)
	} else {
		if p := selectedRow(b); p != nil {
			for i, skill := range p.Skills {
				if i >= 2 {
					break
				}
				yy := y + 91 + int32(i*33)
				ntxt(dc, fmt.Sprintf("%s    %s    %d件", mapped("skill", skill.ID), formatNum(float64(skill.Damage)), skill.Hits), rect{left + 25, yy, right - 20, yy + 28}, 0xC8B396, 12)
			}
		}
	}
}
func nativeBuffPanel(dc uintptr, left, right, y int32) {
	nbox(dc, rect{left, y, right, y + 170}, 0x261912)
	ntxt(dc, "⌄  バフ解析", rect{left + 16, y + 13, right - 14, y + 40}, 0xDFC9B0, 14)
	nbox(dc, rect{left + 14, y + 48, right - 14, y + 78}, 0x453125)
	ntxt(dc, "バフ名                                       発動数              継続時間          稼働率", rect{left + 26, y + 54, right - 20, y + 76}, 0xD1B294, 11)
	ntxt(dc, "バフデータの取得・解析は未対応です", rect{left + 25, y + 105, right - 20, y + 141}, 0x9D8264, 12)
}
func nativeRank(dc uintptr, b *Battle, left, right, y int32) {
	ntxt(dc, "自分のDPS / 職業＋エンティティID", rect{left, y, right, y + 30}, 0xDCC6AB, 16)
	y += 39
	if b == nil {
		ntxt(dc, "自分の設定でIDを指定してください。指定済みの場合は対象ダメージの受信待ちです。", rect{left, y, right, y + 40}, 0xAC987D, 12)
		return
	}
	for i, p := range b.Players {
		yy := y + int32(i*50)
		hit := rect{left, max(208, yy), right, min(nativeBottom, yy+43)}
		if hit.Bottom > hit.Top {
			nativeHits = append(nativeHits, nativeHit{hit, "player-select", i})
		}
		nbox(dc, rect{left, yy, right, yy + 43}, 0x2B1D14)
		fill(dc, rect{left + 2, yy + 2, left + 2 + int32(float64(right-left-4)*p.Share), yy + 41}, 0x4C3824)
		ntxt(dc, fmt.Sprintf("%d  [%s] %s", i+1, classGlyph(p.ClassID), p.Name), rect{left + 14, yy + 11, left + 330, yy + 37}, 0xEEE7E1, 14)
		ntxt(dc, fmt.Sprintf("DPS %s   累計 %s   %s", rowDPSText(p), rowDamageText(p), rowShareText(p)), rect{left + 335, yy + 11, right - 12, yy + 37}, 0xB0D656, 14)
	}
}
func nativeModernClick(x, y int32) {
	if nativeSkillTargetsOpen && !insideNativeRect(x, y, nativeSkillTargetBounds) && !insideNativeRect(x, y, nativeSkillTargetField) {
		nativeSkillTargetsOpen = false
		invalidate(mainWindow)
		return
	}
	if nativeAdaptersOpen && !insideNativeRect(x, y, nativeAdapterPopupBounds) && !insideNativeRect(x, y, nativeAdapterFieldBounds) {
		nativeAdaptersOpen = false
		invalidate(mainWindow)
		return
	}
	for hitIndex := len(nativeHits) - 1; hitIndex >= 0; hitIndex-- {
		h := nativeHits[hitIndex]
		if x < h.R.Left || x >= h.R.Right || y < h.R.Top || y >= h.R.Bottom {
			continue
		}
		if barrierSettingClick(h.Action, h.Index) {
			invalidate(mainWindow)
			return
		}
		switch h.Action {
		case "settings-section", "settings-section-advanced", "settings-section-other":
			openNativeSettings(settingsTab(h.Index))
		case "settings-display-details":
			nativeDisplayDetails = !nativeDisplayDetails
		case "settings-notice-details":
			nativeNoticeDetails = !nativeNoticeDetails
		case "barrier-volume":
			barrierConfig.Volume = int(max(0, min(100, (x-h.R.Left)*100/max(1, h.R.Right-h.R.Left))))
			saveBarrierConfig()
		case "skill-target-toggle":
			nativeSkillTargetsOpen = !nativeSkillTargetsOpen
			nativeSkillTargetOffset = 0
		case "skill-target-noop":
		case "target-all":
			nativeSkillTargetsOpen = false
			selectedDamageTarget = ""
		case "target-select":
			nativeSkillTargetsOpen = false
			if h.Index >= 0 && h.Index < len(nativeTargetRows) {
				currentTargetFilter()
				selectedDamageTarget = nativeTargetRows[h.Index].ID
			}
		case "unknown-effects":
			unknownEffectsExpanded = !unknownEffectsExpanded
		case "history-toggle":
			nativeAdaptersOpen = false
			nativeHistoryOpen = !nativeHistoryOpen
			if nativeHistoryOpen {
				nativeHistoryOffset = 0
			}
		case "history-noop":
		case "history-prev":
			nativeHistoryOffset = max(0, nativeHistoryOffset-nativeHistoryPageSize)
		case "history-next":
			nativeHistoryOffset = min(max(0, len(history)-nativeHistoryPageSize), nativeHistoryOffset+nativeHistoryPageSize)
		case "history-full":
			nativeHistoryOpen = false
			nativePage = "履歴"
			nativeScroll = 0
		case "scroll-to":
			nativeScroll = max(0, min(nativeMaxScroll, (y-h.R.Top)*nativeMaxScroll/max(1, h.R.Bottom-h.R.Top)))
		case "player-select":
			if b := displayBattle(); b != nil && h.Index >= 0 && h.Index < len(b.Players) {
				selectedPlayer = b.Players[h.Index].ID
				nativePage = "スキル"
				nativeScroll = 0
			}
		case "skill-sort":
			if skillSortColumn == h.Index {
				skillSortDescending = !skillSortDescending
			} else {
				skillSortColumn = h.Index
				skillSortDescending = true
			}
			updateDisplay()
		case "flag-review":
			nativePage = "判定詳細"
			nativeScroll = 0
		case "flags-csv":
			nativeSaveFlags()
		case "solo-filter":
			soloShowAll = !soloShowAll
			updateDisplay()
		case "solo-autotoggle":
			soloAutoEnabled = !soloAutoEnabled
			soloAutomaticSelection()
			updateDisplay()
		case "solo-auto":
			candidates := soloCandidates()
			if len(candidates) == 1 {
				_ = setSoloDisplayID(entityDisplayID(candidates[0]))
			} else {
				captureSetupError = "候補が複数、または未受信です。件数・スキル回数を確認して指定してください。"
			}
			updateDisplay()
		case "solo-select":
			candidates := soloCandidates()
			if h.Index >= 0 && h.Index < len(candidates) {
				if e := setSoloDisplayID(entityDisplayID(candidates[h.Index])); e != nil {
					captureSetupError = e.Error()
				}
			}
		case "scope-current", "scope-all":
			historyIndex = -1
			displayCaptureTotal = h.Action == "scope-all"
			updateDisplay()
		case "adapters":
			ensureNativeCapture()
			if !captureCtl.running() {
				refreshCaptureAdapters()
			}
			nativeAdaptersOpen = !nativeAdaptersOpen
			nativeHistoryOpen = false
			nativeAdapterOffset = 0
		case "adapter-noop":
		case "adapter-select":
			if !captureCtl.running() {
				send(controls[idCaptureAdapters], 0x014e, uintptr(h.Index), 0)
				nativeAdaptersOpen = false
				if nativePage == "アダプター選択" {
					nativePage = "分析"
					nativeScroll = 0
				}
			}
		case "record-start":
			if !rawCaptureTools {
				break
			}
			ensureNativeCapture()
			captureCommand(idRecordIdle)
		case "record-stop":
			if !rawCaptureTools {
				break
			}
			if captureCtl.snapshot().Recording.Active {
				captureCommand(idRecordStop)
			}
		case "record-delete":
			if !rawCaptureTools {
				break
			}
			if answer, _, _ := pMessageBox.Call(mainWindow, uintptr(unsafe.Pointer(u16("この記録フォルダー内のアプリ所有PCAPNGを削除しますか？"))), uintptr(unsafe.Pointer(u16("記録の削除"))), 0x34); answer == 6 {
				captureCommand(idRecordDelete)
			}
		case "replay":
			if !rawCaptureTools {
				break
			}
			path, e := dialogPCAPPath(mainWindow)
			if e == nil && path != "" {
				e = loadCandidatePCAP(path)
			}
			if e != nil {
				captureSetupError = e.Error()
			}
			nativePage = "分析"
			nativeScroll = 0
		case "effect-csv":
			nativeOpenEffectCSV()
		case "app-logs":
			nativeOpenApplicationLogs()
		case "translation-csv":
			nativeOpenTranslationCSV()
		case "diagnostic":
			nativeSaveDiagnostic()
		case "legacy-settings":
			captureEmbedded = false
			openCaptureSettings()
			captureEmbedded = true
		case "save":
			onCommand(idExport, 0)
		case "screenshot":
			nativeSavePNG()
		case "opacity-slider":
			opacity = uint8(60 + max(0, min(195, int(x-h.R.Left)*195/int(max(1, h.R.Right-h.R.Left)))))
			updateOverlayStyle()
		case "capture":
			if captureCtl.running() {
				stopCapture()
			} else {
				ensureNativeCapture()
				startOrStopCapture()
				if captureSetupError != "" {
					openNativeSettings(settingsMeasure)
				}
			}
		case "settings":
			openNativeSettings(settingsMeasure)
		case "page:オーバーレイ":
			openNativeSettings(settingsDisplay)
		case "page:結界通知":
			openNativeSettings(settingsNotice)
		case "settings-diagnostics":
			openNativeSettings(settingsAdvanced)
		case "mini-adjust":
			onCommand(idClick, 0)
		case "players":
			openPartySettings()
		case "sample":
			onCommand(idSample, 0)
		case "reset":
			soloReset()
		case "settings-import":
			nativeImportPreviousSettings()
		case "idle-edit":
			openMiniSizeEditor(nil)
		case "overlay-regular":
			setGameOverlay(false)
		case "overlay-game":
			setGameOverlay(true)
		case "mini-hide-without-hud":
			gameOverlay.HideWithoutHUD = !gameOverlay.HideWithoutHUD
			hudProbe.window = 0
			saveGameOverlay()
			syncGameOverlay()
		case "mini-size", "mini-font", "mini-opacity", "mini-text-color", "mini-background", "mini-reset-position":
			updateMiniCardSetting(h.Action, h.Index)
		case "overlay":
			onCommand(idOverlay, 0)
		case "click":
			onCommand(idClick, 0)
		case "opacity":
			onCommand(idOpacity, 0)
		case "history-note":
			openHistoryLocation(h.Index)
		case "history":
			onCommand(idHistory, 0)
			historyIndex = h.Index
			nativeHistoryOpen = false
			nativePage = "分析"
			nativeScroll = 0
			if session != nil {
				session.Playing = false
			}
			updateDisplay()
		default:
			if len(h.Action) > 5 && h.Action[:5] == "page:" {
				nativePage = h.Action[5:]
				nativeScroll = 0
			}
		}
		invalidate(mainWindow)
		break
	}
}

func classGlyph(class string) string {
	r := []rune(class)
	if len(r) == 0 {
		return "人"
	}
	return string(r[0])
}

func nativeScrollRail(dc uintptr, right, bottom int32) {
	if nativeMaxScroll <= 0 {
		return
	}
	rail := rect{right - 7, 208, right - 1, bottom}
	fill(dc, rail, 0x2A1D16)
	height := max(int32(25), (bottom-208)*(bottom-208)/(bottom-208+nativeMaxScroll))
	top := int32(208) + nativeScroll*(bottom-208-height)/max(1, nativeMaxScroll)
	nbox(dc, rect{rail.Left, top, rail.Right, top + height}, 0x695240)
	nativeHits = append(nativeHits, nativeHit{rail, "scroll-to", 0})
}
func nativeModernDrag(x, y int32) {
	for _, h := range nativeHits {
		if (h.Action == "opacity-slider" || h.Action == "scroll-to" || h.Action == "barrier-volume") && x >= h.R.Left && x < h.R.Right && y >= h.R.Top && y < h.R.Bottom {
			nativeModernClick(x, y)
			return
		}
	}
}
