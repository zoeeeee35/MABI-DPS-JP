//go:build native && windows

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	idSample  = 101
	idPause   = 102
	idImport  = 103
	idReset   = 104
	idExit    = 105
	idOverlay = 106
	idClick   = 107
	idOpacity = 108
	idSpeed   = 109
	idIdle    = 110
	idLive    = 111
	idExport  = 112
	idPlayers = 201
	idSkills  = 202
	idHistory = 203
)

var mainWindow, overlayWindow uintptr
var controls = map[int]uintptr{}
var normalFont, boldFont, overlayFont, overlayBold uintptr
var session *Session
var history []SavedBattle
var historyDir string
var historyIndex = -1
var selectedPlayer string
var overlayShown = true
var clickThrough = false
var opacity = uint8(220)
var lastTick time.Time
var errorText string
var lastRows, lastSkills, lastHistory string
var remembered = map[string]bool{}
var dictionaries map[string]struct {
	Name                     string `json:"name"`
	Classes, Skills, Enemies map[string]string
}
var nativeLabels map[string]string
var speed int64 = 1
var idle int64 = 10000
var hotkeyWarning string
var nativeTestPath string
var overlayUserSized bool
var nativeTestStart time.Time
var trace []map[string]any
var lastTraceKey string

func nt(key, fallback string) string {
	if s := nativeLabels[key]; s != "" {
		return s
	}
	return fallback
}
func settingsData(name string) []byte {
	data, _ := assets.ReadFile("config/" + name + ".json")
	exe, _ := os.Executable()
	external, e := os.ReadFile(filepath.Join(filepath.Dir(exe), "config", name+".json"))
	if e == nil && json.Valid(external) {
		data = external
	}
	return data
}
func loadDictionaries() {
	skillNamesOnce.Do(loadSkillDisplayNames)
	_ = json.Unmarshal(settingsData("regions"), &dictionaries)
	_ = json.Unmarshal(settingsData("native-ja"), &nativeLabels)
}
func mapped(kind, id string) string {
	region := "jp"
	source := ""
	if session != nil {
		region = session.Log.Region
		source = session.Log.Source
	}
	if historyIndex >= 0 && historyIndex < len(history) {
		region = history[historyIndex].Archive.Request.Log.Region
		source = history[historyIndex].Archive.Request.Log.Source
	}
	if source == jpProvisionalSource {
		if kind == "skill" {
			if id == "jp5243:unidentified" {
				return "スキル未特定（0x5243）"
			}
			if strings.HasPrefix(id, "jp-group:") {
				return "スキル群（候補）"
			}
			return japaneseSkillName(strings.TrimPrefix(id, "jp-skill:"))
		}
		if id != "unknown" {
			return id
		}
		return "職業不明"
	}
	if source == "synthetic" {
		region = "synthetic"
	}
	d := dictionaries[region]
	var value string
	switch kind {
	case "class":
		value = d.Classes[id]
	case "skill":
		value = d.Skills[id]
	}
	if value == "" {
		return "未確認 (" + id + ")"
	}
	return value
}
func duration(sec float64) string {
	return fmt.Sprintf("%02d:%04.1f", int(sec)/60, float64(int(sec*10)%600)/10)
}

var displayCaptureTotal bool

func displayBattle() *Battle { return filterBattleTarget(baseDisplayBattle(), currentTargetFilter()) }
func baseDisplayBattle() *Battle {
	if soloMeasurement != nil && historyIndex < 0 && soloMeasurement.Preview != nil {
		return soloMeasurement.Preview
	}
	if soloMode && nativeModern {
		if _, ok := soloIdentity(); !ok {
			return nil
		}
	}
	if historyIndex >= 0 && historyIndex < len(history) {
		if soloMode && nativeModern {
			who, ok := soloIdentity()
			if !ok {
				return nil
			}
			return soloBattleView(&history[historyIndex].Battle, who.ID)
		}
		return &history[historyIndex].Battle
	}
	if session != nil {
		if displayCaptureTotal {
			b := captureSummary(session.Result.Battles)
			if soloMode && nativeModern {
				who, _ := soloIdentity()
				return soloBattleView(b, who.ID)
			}
			return b
		}
		if soloMode && nativeModern {
			who, _ := soloIdentity()
			return soloBattleView(session.current(), who.ID)
		}
		return session.current()
	}
	return nil
}
func sourceLabel() string {
	source := ""
	if session != nil {
		source = session.Log.Source
	}
	if historyIndex >= 0 && historyIndex < len(history) {
		source = history[historyIndex].Archive.Request.Log.Source
	}
	if source == jpProvisionalSource {
		if jpFromCapture && captureCtl.running() {
			return "実受信DPS"
		}
		if historyIndex >= 0 {
			return "実通信履歴DPS"
		}
		if jpFromCapture {
			return "実受信結果DPS"
		}
		return "実通信再生DPS"
	}
	if source == "synthetic" {
		return "合成サンプル"
	}
	if source == "capture-experimental" {
		return "実受信候補（解釈未検証）"
	}
	if source == "normalized-offline" {
		return "正規化オフライン"
	}
	return "データ待機"
}
func selectedRow(b *Battle) *Row {
	if b == nil || len(b.Players) == 0 {
		return nil
	}
	for i := range b.Players {
		if b.Players[i].ID == selectedPlayer {
			return &b.Players[i]
		}
	}
	return &b.Players[0]
}
func button(id int, text string, x, y, w int32) {
	controls[id] = createWindow("BUTTON", text, 0, wsVisible|wsChild|wsTabStop, x, y, w, 32, mainWindow, uintptr(id))
	send(controls[id], 0x0030, normalFont, 1)
}
func label(id int, text string, x, y, w, h int32) {
	controls[id] = createWindow("STATIC", text, 0, wsVisible|wsChild, x, y, w, h, mainWindow, uintptr(id))
	send(controls[id], 0x0030, normalFont, 1)
}
func list(id int, x, y, w, h int32) {
	extra := uintptr(0)
	if id == idPlayers {
		extra = 0x0050
	}
	controls[id] = createWindow("LISTBOX", "", 0x200, wsVisible|wsChild|wsTabStop|0x00200000|0x00100000|0x0001|0x0100|extra, x, y, w, h, mainWindow, uintptr(id))
	send(controls[id], 0x0030, normalFont, 1)
	send(controls[id], 0x0194, 1800, 0)
}
func buildControls() {
	label(1, "MABI DPS JP  /  独立型オーバーレイ", 20, 16, 900, 28)
	send(controls[1], 0x0030, boldFont, 1)
	label(2, "日本版実通信のDPSとサンプル再生。敵味方・回復などの種別は未確定です。", 20, 50, 980, 24)
	button(idSample, nt("sample", "サンプル再生"), 20, 83, 140)
	button(idPause, nt("pause", "一時停止"), 170, 83, 120)
	button(idImport, nt("import", "データ読込"), 300, 83, 130)
	button(idReset, nt("reset", "リセット"), 440, 83, 110)
	button(idLive, nt("live", "ライブに戻る"), 560, 83, 140)
	button(idExport, nt("export", "選択戦闘を保存"), 710, 83, 160)
	button(idExit, nt("exit", "終了"), 880, 83, 95)
	button(idOverlay, "オーバーレイ: ON", 20, 126, 175)
	button(idClick, "クリック透過: OFF", 205, 126, 180)
	button(idOpacity, "不透明度: 86%", 395, 126, 150)
	button(idSpeed, "再生速度: 1×", 555, 126, 140)
	button(idIdle, fmt.Sprintf("無ダメージ: %d秒", idle/1000), 705, 126, 160)
	button(idCaptureSettings, "キャプチャ設定", 875, 126, 130)
	button(114, "プレイヤー設定", 870, 164, 135)
	label(3, "画面のボタンで表示・クリック透過を切り替えられます", 20, 170, 1000, 24)
	label(4, "サンプル再生を押してください。", 20, 201, 1000, 25)
	label(5, "プレイヤー別ランキング（選択するとスキル内訳）", 20, 240, 980, 24)
	list(idPlayers, 20, 268, 960, 145)
	label(6, "スキル別ダメージ", 20, 429, 500, 24)
	list(idSkills, 20, 457, 960, 110)
	label(7, "戦闘履歴  /  戦闘終了時に自動保存（選択して分析）", 20, 583, 960, 24)
	list(idHistory, 20, 611, 960, 110)
	label(8, "", 20, 733, 960, 42)
	label(9, "DPS＝総ダメージ÷共通戦闘時間（最小1秒）。実受信では攻撃者を確認して集計対象に追加してください。", 20, 784, 960, 24)
}
func resizeControls() {
	var r rect
	pGetClient.Call(mainWindow, uintptr(unsafe.Pointer(&r)))
	w := r.Right - 40
	if w < 700 {
		w = 700
	}
	for _, id := range []int{idPlayers, idSkills, idHistory} {
		var y, h int32
		switch id {
		case idPlayers:
			y = 268
			h = 145
		case idSkills:
			y = 457
			h = 110
		case idHistory:
			y = 611
			h = 110
		}
		move(controls[id], 20, y, w, h)
	}
	move(controls[8], 20, 733, w, 42)
}
func refreshHistory() {
	var bad int
	history, bad = loadHistory(historyDir)
	if bad > 0 {
		errorText = fmt.Sprintf("読めない履歴を%d件スキップしました。", bad)
	}
	rows := []string{}
	for _, v := range history {
		date, _ := time.Parse(time.RFC3339, v.Archive.CreatedAt)
		source := "オフライン"
		if v.Archive.Request.Log.Source == jpProvisionalSource {
			source = "実通信DPS"
		}
		if v.Archive.Request.Log.Source == "synthetic" {
			source = "合成サンプル"
		}
		_ = date
		_ = source
		rows = append(rows, historyDisplayTitle(v)+" | "+historyDisplayDetail(v))
	}
	joined := strings.Join(rows, "\n")
	if joined != lastHistory {
		send(controls[idHistory], lbReset, 0, 0)
		for _, row := range rows {
			addList(controls[idHistory], row)
		}
		lastHistory = joined
	}
}
func saveCompleted() {
	if session == nil {
		return
	}
	q := Request{Log: session.Log, Until: session.Cursor, Idle: session.Idle}
	changed := false
	for _, b := range session.Result.Battles {
		if b.Active {
			continue
		}
		key := archiveKey(q, b)
		if remembered[key] {
			continue
		}
		if _, e := saveBattle(historyDir, q, b); e != nil {
			errorText = e.Error()
			continue
		}
		remembered[key] = true
		changed = true
	}
	if changed {
		refreshHistory()
	}
}
func updateDisplay() {
	if nativeModern {
		invalidate(mainWindow)
	}
	if jpCandidateView && !isProvisionalDisplay() {
		if nativeModern {
			invalidate(overlayWindow)
		} else {
			updateCandidateDisplay()
		}
		return
	}
	var b *Battle
	if !soloMode || !nativeModern {
		b = displayBattle()
	}
	var overlayRect rect
	pGetClient.Call(overlayWindow, uintptr(unsafe.Pointer(&overlayRect)))
	count := 3
	if b != nil {
		count = max(3, min(8, len(b.Players)))
	}
	desiredHeight := int32(166 + 44*count)
	desiredWidth := uintptr(440)
	if soloMode && nativeModern {
		desiredHeight = fantasyOverlayHeight
		desiredWidth = 360
	}
	if !gameOverlay.Enabled && !overlayUserSized && overlayWindow != 0 && overlayRect.Bottom != desiredHeight {
		pSetWindowPos.Call(overlayWindow, 0, 0, 0, desiredWidth, uintptr(desiredHeight), 0x0016)
	}
	if nativeModern {
		if overlayShown && !gameOverlay.Enabled {
			invalidate(overlayWindow)
		}
		return
	}
	callText(controls[5], "プレイヤー別ランキング（選択するとスキル内訳）")
	var summary string
	if b == nil {
		summary = "データ待機 | サンプルを再生するか、JSONデータを読み込んでください。"
	} else {
		state := "戦闘中"
		if !b.Active {
			state = "終了（無ダメージ判定）"
			if b.Reason == "explicit" {
				state = "終了（終了イベント）"
			}
		}
		mode := "ライブ再生"
		if historyIndex >= 0 {
			mode = "履歴分析"
		}
		summary = fmt.Sprintf("%s | %s | 戦闘%d %s | 時間 %s | 総ダメージ %s | パーティDPS %s", mode, sourceLabel(), b.Number, state, duration(b.Duration), formatNum(float64(b.Damage)), formatNum(float64(b.Damage)/max(1, b.Duration)))
	}
	if isProvisionalDisplay() {
		summary = strings.ReplaceAll(summary, "パーティDPS", "全体DPS")
		summary += " | 方向・回復未分類を含む"
		callText(controls[5], "検出プレイヤー一覧（職業推定・PT所属未確認／DPS）")
	}
	callText(controls[4], summary)
	rows := []string{}
	selected := 0
	if b != nil {
		for i, p := range b.Players {
			if p.ID == selectedPlayer {
				selected = i
			}
			rows = append(rows, fmt.Sprintf("%d. %s  / %s  | DPS %s  | 総ダメージ %s  | 割合 %.1f%%  | クリティカル %.1f%%", i+1, p.Name, mapped("class", p.ClassID), formatNum(p.DPS), formatNum(float64(p.Damage)), p.Share*100, p.CritRate*100))
			if isProvisionalDisplay() {
				rows[len(rows)-1] = fmt.Sprintf("%d. %s / %s | DPS %s | 総ダメージ %s | 割合 %s | 種別未分類", i+1, p.Name, p.ClassID, rowDPSText(p), rowDamageText(p), rowShareText(p))
			}
		}
	}
	joined := strings.Join(rows, "\n")
	if joined != lastRows {
		send(controls[idPlayers], 0x000b, 0, 0)
		send(controls[idPlayers], lbReset, 0, 0)
		for _, row := range rows {
			addList(controls[idPlayers], row)
		}
		if len(rows) > 0 {
			send(controls[idPlayers], lbSetSel, uintptr(selected), 0)
		}
		send(controls[idPlayers], 0x000b, 1, 0)
		invalidate(controls[idPlayers])
		lastRows = joined
	}
	skills := []string{}
	if row := selectedRow(b); row != nil {
		callText(controls[6], row.Name+" / スキル別ダメージ")
		for _, s := range row.Skills {
			skills = append(skills, fmt.Sprintf("%s  | 総ダメージ %s  | DPS %s  | 命中 %d  | クリティカル %.1f%%", mapped("skill", s.ID), formatNum(float64(s.Damage)), formatNum(s.DPS), s.Hits, float64(s.Crits)/max(1, float64(s.Hits))*100))
			if isProvisionalDisplay() {
				skills[len(skills)-1] = fmt.Sprintf("スキル未特定（0x5243） | 値合計 %s | DPS %s | %d件 | クリティカル未判定", formatNum(float64(s.Damage)), formatNum(s.DPS), s.Hits)
			}
		}
	} else {
		callText(controls[6], "スキル別ダメージ")
	}
	joined = strings.Join(skills, "\n")
	if joined != lastSkills {
		send(controls[idSkills], lbReset, 0, 0)
		for _, s := range skills {
			addList(controls[idSkills], s)
		}
		lastSkills = joined
	}
	state := "一時停止"
	if session == nil || !session.Playing {
		state = "再生 / 再開"
	}
	callText(controls[idPause], state)
	note := errorText
	if hotkeyWarning != "" {
		note += " " + hotkeyWarning
	}
	if note == "" {
		note = "履歴保存先: " + historyDir
	}
	callText(controls[8], note)
	invalidate(overlayWindow)
}
func loadSample() {
	data, _ := assets.ReadFile("native_sample.json")
	var l CombatLog
	if e := decode(bytes.NewReader(data), &l); e != nil {
		alert(mainWindow, e.Error())
		return
	}
	loadSession(l, true)
}
func loadSession(l CombatLog, play bool) {
	displayCaptureTotal = false
	jpCandidateView = false
	s, e := newSession(l, idle)
	if e != nil {
		alert(mainWindow, e.Error())
		return
	}
	session = s
	if soloMode && nativeModern && l.Source == "synthetic" {
		c := newDetectedConfig()
		c.discover(l)
		for _, p := range l.Players {
			e := c.Entities[p.ID]
			e.Name = p.Name
			e.Class = mapped("class", p.ClassID)
			e.DisplayID = p.ID
			c.Entities[p.ID] = e
		}
		if len(l.Players) > 0 {
			_ = selectSelf(c, l.Players[0].ID)
		}
		session.Log.Party = c
		_ = session.recompute()
	}
	session.Speed = speed
	session.Playing = play
	historyIndex = -1
	selectedPlayer = ""
	lastTick = time.Now()
	errorText = ""
	updateDisplay()
}
func updateOverlayStyle() {
	style := uintptr(exTopmost | exToolwindow | exLayered | exNoActivate)
	if overlayPassesClicks() {
		style |= exTransparent
	}
	pSetLong.Call(overlayWindow, exIndex(), style)
	pLayer.Call(overlayWindow, 0, uintptr(opacity), 2)
	updateOverlayRegion(overlayWindow)
	pSetWindowPos.Call(overlayWindow, ^uintptr(0), 0, 0, 0, 0, 0x0013|0x0020)
	show(overlayWindow, overlayShown && !gameOverlay.Enabled)
	syncGameOverlay()
	click := "OFF"
	if overlayPassesClicks() {
		click = "ON"
	}
	callText(controls[idClick], "クリック透過: "+click)
	visibility := "OFF"
	if overlayShown {
		visibility = "ON"
	}
	callText(controls[idOverlay], "オーバーレイ: "+visibility)
	callText(controls[idOpacity], fmt.Sprintf("不透明度: %d%%", int(opacity)*100/255))
	invalidate(overlayWindow)
}
func onCommand(id, notification int) {
	if id == idReset && captureActive && jpInput != nil {
		var e error
		jpInput, e = newJPProvisionalInput(time.Now(), idle)
		if e != nil {
			errorText = e.Error()
			return
		}
		enableLiveRotation(jpInput.Live)
		captureInput = jpInput.Live
		session = nil
		historyIndex = -1
		errorText = "DPSをリセットしました。キャプチャは継続中です。"
		updateDisplay()
		return
	}
	if id == idSample || id == idImport || id == idReset || (id == idHistory && notification == 1) {
		pauseAutoCapture()
		jpFromCapture = false
		jpInput = nil
		jpCandidateView = false
	}
	if captureActive && (id == idSample || id == idImport || id == idReset || (id == idHistory && notification == 1)) {
		stopCapture()
	}
	if id == 114 {
		openPartySettings()
		return
	}
	if id == idCaptureSettings {
		openCaptureSettings()
		return
	}
	switch id {
	case idSample:
		loadSample()
	case idPause:
		if captureActive {
			return
		}
		if session != nil {
			if session.Cursor >= session.endTime() {
				session.Cursor = 0
				_ = session.recompute()
			}
			session.Playing = !session.Playing
			historyIndex = -1
			lastTick = time.Now()
		}
	case idImport:
		if session != nil {
			session.Playing = false
		}
		path, e := dialogPath(mainWindow, false)
		if e != nil {
			alert(mainWindow, e.Error())
		} else if path != "" {
			l, e := readLog(path)
			if e != nil {
				alert(mainWindow, e.Error())
			} else {
				loadSession(l, false)
			}
		}
	case idReset:
		session = nil
		historyIndex = -1
		selectedPlayer = ""
		errorText = "データをリセットしました。保存済み履歴は保持しています。"
	case idExit:
		pDestroy.Call(mainWindow)
	case idOverlay:
		overlayShown = !overlayShown
		updateOverlayStyle()
	case idClick:
		if gameOverlay.Enabled {
			gameOverlayAdjust = !gameOverlayAdjust
		} else {
			clickThrough = !clickThrough
		}
		updateOverlayStyle()
	case idOpacity:
		switch opacity {
		case 220:
			opacity = 170
		case 170:
			opacity = 110
		case 110:
			opacity = 255
		default:
			opacity = 220
		}
		updateOverlayStyle()
	case idSpeed:
		if speed == 1 {
			speed = 2
		} else if speed == 2 {
			speed = 4
		} else {
			speed = 1
		}
		if session != nil {
			session.Speed = speed
		}
		callText(controls[idSpeed], fmt.Sprintf("再生速度: %d×", speed))
	case idIdle:
		switch idle {
		case 5000:
			idle = 10000
		case 10000:
			idle = 15000
		case 15000:
			idle = 3000
		default:
			idle = 5000
		}
		if session != nil {
			session.Idle = idle
			_ = session.recompute()
		}
		callText(controls[idIdle], fmt.Sprintf("無ダメージ: %d秒", idle/1000))
	case idLive:
		historyIndex = -1
		selectedPlayer = ""
	case idExport:
		b := displayBattle()
		if b != nil {
			path, e := dialogPath(mainWindow, true)
			if e != nil {
				alert(mainWindow, e.Error())
			} else if path != "" {
				var q Request
				if historyIndex >= 0 {
					q = history[historyIndex].Archive.Request
				} else {
					q = Request{Log: session.Log, Until: session.Cursor, Idle: session.Idle}
				}
				payload := struct {
					Version      int     `json:"version"`
					Source       string  `json:"source"`
					Region       string  `json:"region"`
					LiveVerified bool    `json:"liveVerified"`
					Battle       *Battle `json:"battle"`
				}{1, q.Log.Source, q.Log.Region, false, b}
				data, _ := json.MarshalIndent(payload, "", "  ")
				if e = os.WriteFile(path, data, 0600); e != nil {
					alert(mainWindow, "保存できません: "+e.Error())
				} else {
					errorText = "選択した戦闘の分析結果を保存しました。"
				}
			}
		}
	case idPlayers:
		if notification == 1 {
			if b := displayBattle(); b != nil {
				i := int(send(controls[idPlayers], lbGetSel, 0, 0))
				if i >= 0 && i < len(b.Players) {
					selectedPlayer = b.Players[i].ID
					lastSkills = ""
				}
			}
		}
	case idHistory:
		if notification == 1 {
			i := int(send(controls[idHistory], lbGetSel, 0, 0))
			if i >= 0 && i < len(history) {
				if session != nil {
					session.Playing = false
				}
				historyIndex = i
				selectedPlayer = ""
				lastRows = ""
				lastSkills = ""
			}
		}
	}
	updateDisplay()
}
func paintOverlayContent(window, dc uintptr) {

	if soloMode && nativeModern {
		paintSoloOverlay(window, dc)
		return
	}
	var r rect
	pGetClient.Call(window, uintptr(unsafe.Pointer(&r)))
	fill(dc, r, 0x211A16)
	if jpCandidateView && !isProvisionalDisplay() {
		paintCandidateOverlay(dc, r)
		return
	}
	overlayMode := "MABI DPS JP  /  OFFLINE"
	if captureActive || (session != nil && session.Log.Source == "capture-experimental") {
		overlayMode = "MABI DPS JP / 実受信候補"
	}
	if isProvisionalDisplay() {
		overlayMode = "MABI DPS JP / DPS"
	}
	draw(dc, overlayMode, rect{16, 12, r.Right - 12, 40}, 0xB5D873, overlayBold, 0x20|0x8000)
	subtitle := "日本版の解釈は未検証"
	if isProvisionalDisplay() {
		subtitle = "検出プレイヤー / PT所属・種別未確定"
	}
	draw(dc, subtitle, rect{16, 43, r.Right - 12, 66}, 0x91BDE2, overlayFont, 0x20|0x8000)
	b := displayBattle()
	if b == nil {
		draw(dc, "サンプル再生を押してください", rect{16, 85, r.Right - 12, 115}, 0xE9E5E0, overlayFont, 0x20|0x8000)
	} else {
		state := "戦闘中"
		if !b.Active {
			state = "戦闘終了"
		}
		battleLabel := fmt.Sprintf("戦闘%d %s", b.Number, state)
		if b.Reason == "capture-summary" {
			battleLabel = "記録全体（待機含む）"
		}
		draw(dc, fmt.Sprintf("%s | %s | %s", sourceLabel(), duration(b.Duration), battleLabel), rect{16, 70, r.Right - 12, 94}, 0xCCC0AE, overlayFont, 0x20|0x8000)
		draw(dc, "プレイヤー                   DPS         ダメージ", rect{16, 102, r.Right - 12, 124}, 0xAAA49B, overlayFont, 0x20|0x8000)
		for i, p := range b.Players {
			if i >= 8 {
				break
			}
			y := int32(132 + i*44)
			fill(dc, rect{12, y, r.Right - 12, y + 38}, 0x302720)
			bar := int32(float64(r.Right-24) * p.Share)
			fill(dc, rect{12, y, 12 + bar, y + 38}, 0x524332)
			draw(dc, fmt.Sprintf("%d  [%s] %s", i+1, classGlyph(mapped("class", p.ClassID)), p.Name), rect{22, y + 2, r.Right - 175, y + 22}, 0xF2EEEA, overlayFont, 0x20|0x8000)
			draw(dc, rowDPSText(p), rect{r.Right - 172, y + 2, r.Right - 98, y + 22}, 0xB5D873, overlayBold, 0x20|0x2|0x8000)
			draw(dc, rowDamageText(p), rect{r.Right - 90, y + 2, r.Right - 20, y + 22}, 0xF2EEEA, overlayFont, 0x20|0x2|0x8000)
			sub := fmt.Sprintf("割合 %.1f%% / クリティカル %.1f%%", p.Share*100, p.CritRate*100)
			if isProvisionalDisplay() {
				sub = fmt.Sprintf("取得済み分の割合 %.1f%% / 種別未分類", p.Share*100)
				if p.DamageStatus == "unobserved" {
					sub = "対象イベント未取得 / 実ダメージ0とは未判定"
				}
			}
			draw(dc, sub, rect{22, y + 20, r.Right - 20, y + 38}, 0xBDB3A8, overlayFont, 0x20|0x8000)
		}
	}
	mode := "ドラッグで移動"
	if clickThrough {
		mode = "クリック透過ON：メイン画面で解除"
	}
	draw(dc, mode, rect{16, r.Bottom - 28, r.Right - 12, r.Bottom - 5}, 0xA79B8C, overlayFont, 0x20|0x8000)

}
func overlayPaint(window uintptr) {
	var ps paintStruct
	dc, _, _ := pBeginPaint.Call(window, uintptr(unsafe.Pointer(&ps)))
	paintBuffered(window, dc, paintOverlayContent)
	pEndPaint.Call(window, uintptr(unsafe.Pointer(&ps)))
}
func overlayProc(window uintptr, msg uint32, w, l uintptr) uintptr {
	if window == miniDPS.handle || window == miniBarrier.handle || window == miniNotice.handle {
		return miniOverlayProc(window, msg, w, l)
	}
	if msg == 0x0200 && !clickThrough {
		fantasyOverlayMove(window, int32(int16(l&0xffff)), int32(int16(l>>16)))
		return 0
	}
	if msg == 0x02A3 {
		nativeOverlayHover = -1
		overlaySkillHover = -1
		invalidate(window)
		return 0
	}

	switch msg {
	case 0x020A:
		if nativeModern && soloMode && !clickThrough && overlaySkillsScroll(window, w, l) {
			return 0
		}
	case wmDestroy:
		releasePaintBuffer(window)
		return 0
	case 0x0083:
		return 0
	case 0x0084:
		if clickThrough {
			return ^uintptr(0)
		}
		var bounds rect
		user32.NewProc("GetWindowRect").Call(window, uintptr(unsafe.Pointer(&bounds)))
		x, y := int32(int16(l&0xffff))-bounds.Left, int32(int16((l>>16)&0xffff))-bounds.Top
		return overlayResizeHit(x, y, bounds.Right-bounds.Left, bounds.Bottom-bounds.Top)
	case 0x0232:
		overlayUserSized = true
		return 0
	case wmSize:
		width, height := int32(l&0xffff), int32((l>>16)&0xffff)
		minimumHeight := int32(210)
		if nativeModern && soloMode {
			minimumHeight = 480
		}
		if w != 1 && width > 0 && height > 0 && (width < 340 || height < minimumHeight) {
			pSetWindowPos.Call(window, 0, 0, 0, uintptr(max(width, 340)), uintptr(max(height, minimumHeight)), 0x0016)
		}
		updateOverlayRegion(window)
		return 0
	case wmPaint:
		overlayPaint(window)
		return 0
	case wmErase:
		return 1
	case wmMouseActivate:
		return 3
	case wmLButtonDown:
		if !clickThrough {
			if soloMode && nativeModern && soloOverlayClick(window, int32(int16(l&0xffff)), int32(int16(l>>16))) {
				return 0
			}
			pReleaseCapture.Call()
			pSendMessage.Call(window, 0x00a1, 2, 0)
		}
		return 0
	case wmClose:
		overlayShown = false
		show(window, false)
		return 0
	}
	v, _, _ := pDefWindow.Call(window, uintptr(msg), w, l)
	return v
}
func mainProc(window uintptr, msg uint32, w, l uintptr) uintptr {
	if msg == 0x001e || (msg == 0x0218 && (w == 7 || w == 18)) {
		barrierScheduler.last = time.Time{}
		barrierLastSecond = -1
		barrierBannerActive = false
		show(barrierBannerWindow, false)
		tickBarrierRuntime(time.Now())
		syncGameOverlay()
		invalidate(overlayWindow)
		return 0
	}
	if msg == 0x0218 && w == 4 {
		stopBarrierAudio()
		return 1
	}
	if nativeModern {
		switch msg {
		case wmErase:
			return 1
		case 0x0100:
			if w == 27 {
				nativeAdaptersOpen = false
				nativeHistoryOpen = false
				nativeSkillTargetsOpen = false
				invalidate(window)
				return 0
			}
		case wmPaint:
			var ps paintStruct
			dc, _, _ := pBeginPaint.Call(window, uintptr(unsafe.Pointer(&ps)))
			paintBuffered(window, dc, nativeLayout)
			pEndPaint.Call(window, uintptr(unsafe.Pointer(&ps)))
			return 0
		case 0x0318:
			nativeLayout(window, w)
			return 0
		case wmLButtonDown:
			nativeSettingHover = ""
			nativeModernClick(int32(int16(l&0xffff)), int32(int16(l>>16)))
			return 0
		case 0x02A1: // WM_MOUSEHOVER
			if help := nativeSettingHelpAt(nativePieMouseX, nativePieMouseY); help != "" {
				nativeSettingHover = help
				invalidate(window)
			}
			return 0
		case 0x02A3: // WM_MOUSELEAVE
			nativeSettingHover = ""
			nativePieMouseInside = false
			nativePieHoverKey = ""
			invalidate(window)
			return 0
		case 0x0200:
			nativePieMove(window, int32(int16(l&0xffff)), int32(int16(l>>16)))
			if w&1 != 0 {
				nativeModernDrag(int32(int16(l&0xffff)), int32(int16(l>>16)))
			}
			return 0
		case 0x020a:
			nativeSettingHover = ""
			if nativeSkillTargetsOpen {
				step := 1
				if int16(w>>16) > 0 {
					step = -1
				}
				nativeSkillTargetOffset = max(0, min(max(0, len(nativeTargetRows)+1-nativeSkillTargetVisible), nativeSkillTargetOffset+step))
				invalidate(window)
				return 0
			}
			if nativeAdaptersOpen {
				step := 1
				if int16(w>>16) > 0 {
					step = -1
				}
				nativeAdapterOffset = max(0, min(max(0, len(captureAdapterList)-nativeAdapterVisible), nativeAdapterOffset+step))
				invalidate(window)
				return 0
			}
			if nativeHistoryOpen {
				step := 1
				if int16(w>>16) > 0 {
					step = -1
				}
				nativeHistoryOffset = max(0, min(max(0, len(history)-nativeHistoryPageSize), nativeHistoryOffset+step))
				invalidate(window)
				return 0
			}
			nativeScroll = max(0, min(nativeMaxScroll, nativeScroll-int32(int16(w>>16))/2))
			invalidate(window)
			return 0
		case 0x0024:
			if l != 0 {
				m := (*[10]int32)(nativeMessagePointer(l))
				m[6] = 1000
				m[7] = 730
				return 0
			}
		}
	}
	switch msg {
	case 0x002b:
		if drawPlayerRanking(l) {
			return 1
		}
	case 0x002c:
		if l != 0 {
			m := (*[6]uint32)(nativeMessagePointer(l))
			if int(m[1]) == idPlayers {
				m[4] = 36
				return 1
			}
		}
	case wmCommand:
		onCommand(int(w&0xffff), int(w>>16))
		return 0
	case wmSize:
		if controls[idPlayers] != 0 {
			resizeControls()
		}
		return 0
	case wmTimer:
		if w == 2 {
			tickCursorLock()
			return 0
		}
		tickCursorLock()
		pumpUpdateEvents()
		if updateExiting {
			return 0
		}
		tickAutoCapture(time.Now())
		tickBarrierRuntime(time.Now())
		syncGameOverlay()
		if !soloTimerEnd.IsZero() && time.Until(soloTimerEnd) > -time.Second {
			invalidate(overlayWindow)
		}
		pumpCapture()
		now := time.Now()
		updateNativeNetworkMeter(now)
		updateSoloMeasurement(now)
		appDiagnostics.tick(now, captureActive, soloMeasurement != nil)
		elapsed := now.Sub(lastTick).Milliseconds()
		lastTick = now
		if session != nil && session.Playing {
			if e := session.advance(elapsed); e != nil {
				session.Playing = false
				errorText = e.Error()
			}
			saveCompleted()
			updateDisplay()
		}
		if nativeTestPath != "" {
			if session != nil && session.Cursor > 1000 && !clickThrough {
				clickThrough = true
				updateOverlayStyle()
			}
			recordNativeTrace()
			if session != nil && !session.Playing && session.Cursor >= session.endTime() {
				writeNativeTest()
				pDestroy.Call(mainWindow)
			}
		}
		return 0
	case wmClose:
		pDestroy.Call(window)
		return 0
	case wmDestroy:
		releasePaintBuffer(window)
		captureCtl.stop()
		barrierCancel()
		stopBarrierToast()
		stopBarrierAudio()
		if barrierBannerWindow != 0 {
			pDestroy.Call(barrierBannerWindow)
		}
		if overlayWindow != 0 {
			pDestroy.Call(overlayWindow)
		}
		pPostQuit.Call(0)
		return 0
	}
	v, _, _ := pDefWindow.Call(window, uintptr(msg), w, l)
	return v
}
func recordNativeTrace() {
	if session == nil {
		return
	}
	b := session.current()
	if b == nil {
		return
	}
	key := fmt.Sprintf("%d-%v", b.Number, b.Active)
	if len(b.Players) > 0 {
		key += "-" + b.Players[0].ID
	}
	if key == lastTraceKey {
		return
	}
	lastTraceKey = key
	trace = append(trace, map[string]any{"atMs": session.Cursor, "battle": b.Number, "active": b.Active, "damage": b.Damage, "players": b.Players})
}
func writeNativeTest() {
	style, _, _ := pGetLong.Call(overlayWindow, exIndex())
	previewError := ""
	if e := captureOwnOverlay(filepath.Join(filepath.Dir(nativeTestPath), "overlay-render.png")); e != nil {
		previewError = e.Error()
	}
	var actualAlpha uint8
	var layerColor, layerFlags uint32
	layerOK, _, _ := user32.NewProc("GetLayeredWindowAttributes").Call(overlayWindow, uintptr(unsafe.Pointer(&layerColor)), uintptr(unsafe.Pointer(&actualAlpha)), uintptr(unsafe.Pointer(&layerFlags)))
	payload := map[string]any{"version": 1, "liveVerified": false, "browserUsed": false, "overlayWindowCreated": overlayWindow != 0, "layered": style&exLayered != 0, "clickThrough": style&exTransparent != 0, "opacity": actualAlpha, "layerAttributesApplied": layerOK != 0 && layerFlags&2 != 0, "previewError": previewError, "topmost": style&exTopmost != 0, "noActivate": style&exNoActivate != 0, "trace": trace, "historyCount": len(history), "result": session.Result}
	payload["captureSettingsReviewError"] = reviewCaptureSettingsSurface(filepath.Dir(nativeTestPath))
	data, _ := json.MarshalIndent(payload, "", "  ")
	_ = os.WriteFile(nativeTestPath, data, 0600)
}
func main() {
	defer releaseCursorLock()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	updateHelper := flag.Bool("update-helper", false, "検証済み更新の適用用（内部処理）")
	dir := flag.String("data-dir", "", "履歴保存先（既定: EXEと同じ場所のdata）")
	uiTest := flag.String("ui-self-test", "", "サンプル再生後に検証結果をJSON保存して終了")
	modernUITest := flag.Bool("modern-ui-test", false, "ui-self-testで通常オーバーレイの新UIを描画")
	probe := flag.String("capture-probe", "", "生データを保存せず診断件数をJSONへ出力")
	adapter := flag.Int("adapter", -1, "診断用のアダプター番号")
	seconds := flag.Int("seconds", 8, "診断時間（1〜60秒）")
	region := flag.String("region", "jp", "取得プロファイルjp/kr")
	inventory := flag.String("capture-inventory", "", "取得なしでアダプター番号と対象接続件数をJSON出力")
	rawOption := func(name, help string) *string {
		if rawCaptureTools {
			return flag.String(name, "", help)
		}
		return new(string)
	}
	pcapReplay := rawOption("pcap-replay", "PCAP/PCAPNGをオフライン診断（生データ出力なし）")
	var serverPort int
	if rawCaptureTools {
		flag.IntVar(&serverPort, "server-port", 0, "PCAP診断時の対象TCP送信元ポート")
	}
	diagnosticOut := rawOption("diagnostic-out", "PCAP診断JSONの出力先")
	compareDir := rawOption("analyze-capture-dir", "3記録のローカル差分解析")
	deleteDir := rawOption("delete-capture-dir", "指定フォルダーの3記録と記録メタデータを削除")
	candidateCheck := rawOption("candidate-ui-test", "候補表示・DPS分離の検証JSONを保存して終了")
	candidateReplay := rawOption("candidate-replay", "PCAPNGの値を既存画面で表示（確定DPSなし）")
	selfID := flag.String("self-id", "", "この記録で自分として表示するエンティティID（明示指定）")
	classicUI := flag.Bool("classic-ui", false, "旧標準UIを使用")
	skillListDir := rawOption("skill-list-dir", "PCAPNGフォルダーからスキル一覧CSVを作成")
	skillListOut := rawOption("skill-list-out", "スキル一覧CSVの出力先")
	skillImport := flag.String("skill-dictionary-import", "", "確認済み日本語訳CSVを読み込む")
	skillDictionaryOut := flag.String("skill-dictionary-out", "", "新しい日本語辞書JSONの出力先")
	versionOut := flag.String("version-info", "", "バージョン情報をJSONに保存して終了")
	barrierProbe := flag.String("barrier-probe", "", "結界通知のAPI検証 toast/sound")
	barrierProbeReport := flag.String("barrier-probe-report", "", "通知API検証JSONの出力先")
	barrierProbeSound := flag.String("barrier-probe-sound", "", "試聴用の音声ファイル")
	flag.Parse()
	if *updateHelper {
		runUpdateHelper()
		return
	}
	if *barrierProbe != "" && *barrierProbeReport != "" {
		runBarrierProbe(*barrierProbe, *barrierProbeReport, *barrierProbeSound)
		return
	}
	if *versionOut != "" {
		data, e := json.MarshalIndent(versionInformation(), "", "  ")
		if e != nil {
			os.Exit(1)
		}
		if os.WriteFile(*versionOut, data, 0600) != nil {
			os.Exit(1)
		}
		return
	}
	if rawCaptureTools && *skillListDir != "" {
		if *skillListOut == "" {
			os.Exit(2)
		}
		if e := exportSkillCatalogFiles(*skillListDir, *skillListOut); e != nil {
			os.Exit(1)
		}
		return
	}
	if *skillImport != "" {
		if *skillDictionaryOut == "" {
			os.Exit(2)
		}
		if e := importSkillCatalogFile(*skillImport, *skillDictionaryOut); e != nil {
			os.Exit(1)
		}
		return
	}
	nativeModern = !*classicUI && (*uiTest == "" || *modernUITest)
	if rawCaptureTools && *deleteDir != "" {
		if e := deleteLocalRecordings(*deleteDir); e != nil {
			os.Exit(1)
		}
		return
	}
	if rawCaptureTools && *compareDir != "" {
		if *diagnosticOut == "" {
			os.Exit(2)
		}
		report, e := compareCaptureDirectory(*compareDir)
		if e != nil {
			report = map[string]any{"error": e.Error()}
		}
		data, _ := json.MarshalIndent(report, "", "  ")
		writeErr := os.WriteFile(*diagnosticOut, data, 0600)
		if e != nil || writeErr != nil {
			os.Exit(1)
		}
		return
	}
	if rawCaptureTools && *pcapReplay != "" {
		if serverPort < 1 || serverPort > 65535 || *diagnosticOut == "" {
			os.Exit(2)
		}
		file, e := os.Open(*pcapReplay)
		if e != nil {
			os.Exit(1)
		}
		stats, e := replayPCAP(file, uint16(serverPort))
		file.Close()
		detail := ""
		if e != nil {
			detail = e.Error()
		}
		data, _ := json.MarshalIndent(map[string]any{"stats": stats, "error": detail, "liveVerified": false, "rawPacketsWrittenByReplay": false}, "", "  ")
		_ = os.WriteFile(*diagnosticOut, data, 0600)
		if e != nil {
			os.Exit(1)
		}
		return
	}
	if *inventory != "" {
		if e := runCaptureInventory(*inventory); e != nil {
			data, _ := json.Marshal(map[string]any{"error": e.Error()})
			_ = os.WriteFile(*inventory, data, 0600)
			os.Exit(1)
		}
		return
	}
	if *probe != "" {
		if *seconds < 1 || *seconds > 60 {
			os.Exit(2)
		}
		if e := runCaptureProbe(*probe, *adapter, *seconds, *region); e != nil {
			data, _ := json.Marshal(map[string]any{"error": e.Error(), "liveVerified": false})
			_ = os.WriteFile(*probe, data, 0600)
			os.Exit(1)
		}
		return
	}
	if *uiTest == "" && *candidateCheck == "" && *candidateReplay == "" {
		instance, err := openAppInstance(applicationInstanceName)
		if err == windows.ERROR_ALREADY_EXISTS {
			pMessageBox.Call(0, uintptr(unsafe.Pointer(u16("MABI DPS JP はすでに起動しています。別の版に切り替える場合は、起動中のアプリを終了してください。"))), uintptr(unsafe.Pointer(u16("MABI DPS JP"))), 0x40)
			return
		}
		if err != nil {
			alert(0, "多重起動を確認できません。起動中の MABI DPS JP を終了してから再実行してください。")
			return
		}
		defer windows.CloseHandle(instance)
	}
	exe, _ := os.Executable()
	if *dir == "" {
		*dir = filepath.Join(filepath.Dir(exe), "data")
	}
	historyDir = filepath.Join(*dir, "history")
	if *uiTest == "" && *candidateCheck == "" && *candidateReplay == "" {
		promptFirstRunSettings(*dir)
	}
	appDiagnostics, _ = startApplicationDiagnostics(filepath.Join(*dir, "logs"))
	defer appDiagnostics.finishAfterRun()
	initBarrierRuntime()
	nativeTestPath = *uiTest
	user32.NewProc("SetProcessDPIAware").Call()
	loadDictionaries()
	loadSoloFonts()
	normalFont = newFont(15, false)
	boldFont = newFont(22, true)
	overlayFont = newFont(14, false)
	overlayBold = newFont(17, true)
	brush, _, _ := gdi32.NewProc("GetStockObject").Call(0)
	if e := registerCaptureClass(brush); e != nil {
		alert(0, e.Error())
		return
	}
	if e := registerClass("MabiDPSControl", syscall.NewCallback(mainProc), brush); e != nil {
		alert(0, e.Error())
		return
	}
	if e := registerClass("MabiDPSOverlay", syscall.NewCallback(overlayProc), 0); e != nil {
		alert(0, e.Error())
		return
	}
	mainWindow = createWindow("MabiDPSControl", appWindowTitle(), 0, 0x00cf0000, 90, 80, 1040, 870, 0, 0)
	if mainWindow == 0 {
		alert(0, "アプリ画面を作成できません")
		return
	}
	if nativeModern {
		applyNativeDarkTitle(mainWindow)
	}
	buildControls()
	if nativeModern {
		for _, h := range controls {
			show(h, false)
		}
		pSetWindowPos.Call(mainWindow, 0, 0, 0, 1180, 970, 0x0016)
	}
	screenW, _, _ := pMetrics.Call(0)
	overlayWindow = createWindow("MabiDPSOverlay", "MABI DPS JP オーバーレイ", exTopmost|exToolwindow|exLayered|exNoActivate, 0x80040000, int32(screenW)-470, 120, 440, 298, 0, 0)
	if overlayWindow == 0 {
		alert(mainWindow, "オーバーレイを作成できません")
		pDestroy.Call(mainWindow)
		return
	}
	// New installations of the current UI start in game-mini mode.
	gameOverlay.Enabled = nativeModern
	loadGameOverlay()
	updateOverlayStyle()
	if nativeModern {
		captureEmbedded = true
		openCaptureSettings()
	}
	refreshHistory()
	if rawCaptureTools && *candidateReplay != "" {
		if e := loadCandidatePCAP(*candidateReplay); e != nil {
			errorText = e.Error()
		}
	}
	if *selfID != "" {
		if e := setSoloDisplayID(*selfID); e != nil {
			errorText = e.Error()
		}
	}
	updateDisplay()
	pShowWindow.Call(mainWindow, 1)
	user32.NewProc("UpdateWindow").Call(mainWindow)
	lastTick = time.Now()
	initUpdateRuntime(*dir, *uiTest != "" || *candidateCheck != "" || *candidateReplay != "")
	initAutoCapture(*dir, !nativeModern || *uiTest != "" || *candidateCheck != "" || *candidateReplay != "")
	pSetTimer.Call(mainWindow, 1, 100, 0)
	pSetTimer.Call(mainWindow, 2, 25, 0)
	if nativeTestPath != "" {
		if *modernUITest {
			setGameOverlay(false)
		}
		speed = 4
		loadSample()
		session.Speed = 4
		nativeTestStart = time.Now()
	}
	if rawCaptureTools && *candidateCheck != "" {
		if isProvisionalDisplay() {
			session.Cursor = session.endTime()
			session.Playing = false
			_ = session.recompute()
			saveCompleted()
			updateDisplay()
		}
		imagePath := filepath.Join(filepath.Dir(*candidateCheck), "candidate-overlay.png")
		imageErr := captureOwnOverlay(imagePath)
		_ = captureOwnSurface(filepath.Join(filepath.Dir(*candidateCheck), "player-main.png"), mainWindow, true)
		if nativeModern {
			oldPage, oldScroll := nativePage, nativeScroll
			nativePage = "設定"
			nativeScroll = 0
			_ = captureOwnSurface(filepath.Join(filepath.Dir(*candidateCheck), "capture-settings.png"), mainWindow, true)
			nativePage = "分析"
			nativeScroll = 590
			_ = captureOwnSurface(filepath.Join(filepath.Dir(*candidateCheck), "analysis-details.png"), mainWindow, true)
			nativePage, nativeScroll = oldPage, oldScroll
		}
		detail := ""
		if imageErr != nil {
			detail = imageErr.Error()
		}
		data, _ := json.MarshalIndent(map[string]any{"candidateMode": jpCandidateView, "jp9": jpDisplay, "battlePresent": displayBattle() != nil, "provisional": isProvisionalDisplay(), "result": sessionResultForTest(), "historyCount": len(history), "renderError": detail, "uiReview": nativeReferenceReview(filepath.Dir(*candidateCheck))}, "", "  ")
		_ = os.WriteFile(*candidateCheck, data, 0600)
		pDestroy.Call(mainWindow)
	}
	var msg message
	for {
		v, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(v) <= 0 {
			break
		}
		pTranslate.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatch.Call(uintptr(unsafe.Pointer(&msg)))
	}
	for _, f := range []uintptr{normalFont, boldFont, overlayFont, overlayBold} {
		if f != 0 {
			pDelete.Call(f)
		}
	}
}
