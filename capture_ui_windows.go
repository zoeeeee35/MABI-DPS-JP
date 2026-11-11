//go:build native && windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

const (
	idCaptureSettings = 113
	idCaptureStart    = 301
	idCaptureAdapters = 302
	idCaptureRegion   = 303
	idCaptureActor    = 304
	idCaptureInclude  = 305
	idCaptureRefresh  = 306
	idCaptureDiag     = 307
	idMeasureIdle     = 308
	idMeasureAttack   = 309
	idMeasureSkill    = 310
	idRecordIdle      = 311
	idRecordNormal    = 312
	idRecordSkill     = 313
	idRecordStop      = 314
	idRecordDelete    = 315
)

var captureEmbedded bool
var recordingDirectory string
var captureWindow uintptr
var captureCtl = &CaptureController{}
var captureAdapterList []CaptureDevice
var captureInput *LiveInput
var jpInput *JPProvisionalInput
var jpUpdateAt time.Time
var captureActive bool
var actorListSignature string
var captureSetupError string

func captureChild(class, text string, id int, x, y, w, h int32, style uintptr) uintptr {
	handle := createWindow(class, text, 0, wsChild|wsVisible|style, x, y, w, h, captureWindow, uintptr(id))
	controls[id] = handle
	send(handle, 0x0030, normalFont, 1)
	return handle
}
func openCaptureSettings() {
	if captureWindow != 0 {
		show(captureWindow, !captureEmbedded)
		if !captureEmbedded {
			pSetForeground.Call(captureWindow)
		}
		return
	}
	captureWindow = createWindow("MabiDPSCapture", "MABI DPS JP — データ取得設定（互換性実験）", 0, 0x00ca0000, 160, 140, 790, 720, mainWindow, 0)
	captureChild("STATIC", "Npcapによる受信コピーのみ。通信改変・復号・注入は行いません。", 320, 18, 15, 740, 24, 0)
	captureChild("STATIC", "ネットワークアダプター", 321, 18, 48, 170, 24, 0)
	captureChild("COMBOBOX", "", idCaptureAdapters, 190, 44, 560, 200, 0x0003|wsTabStop|0x00200000)
	captureChild("STATIC", "地域 / 実験プロファイル", 322, 18, 87, 180, 24, 0)
	h := captureChild("COMBOBOX", "", idCaptureRegion, 205, 82, 340, 150, 0x0003|wsTabStop)
	sendString(h, 0x0143, "日本版 / 韓国旧形式の互換性検査")
	sendString(h, 0x0143, "韓国版 / 公開旧形式（未実測）")
	send(h, 0x014e, 0, 0)
	captureChild("BUTTON", "キャプチャ開始", idCaptureStart, 18, 127, 170, 32, wsTabStop)
	captureChild("BUTTON", "アダプター再読込", idCaptureRefresh, 200, 127, 180, 32, wsTabStop)
	captureChild("BUTTON", "診断JSONを保存", idCaptureDiag, 392, 127, 180, 32, wsTabStop)
	captureChild("STATIC", "", 323, 18, 175, 740, 120, 0)
	captureChild("STATIC", "検出した攻撃者（敵か味方かは自動判定できません）", 324, 18, 302, 740, 24, 0)
	captureChild("COMBOBOX", "", idCaptureActor, 18, 335, 490, 180, 0x0003|wsTabStop|0x00200000)
	captureChild("BUTTON", "PT分類・名前を設定", idCaptureInclude, 520, 335, 230, 32, wsTabStop)
	captureChild("STATIC", "日本版は識別子別に集計します。敵等と確認できた識別子は除外してください。", 325, 18, 383, 740, 24, 0)
	captureChild("STATIC", "診断は方向・ポート・件数・バイト数だけ。診断JSONに通信内容は出しません。", 326, 18, 415, 740, 24, 0)
	captureChild("STATIC", "職業・名前・パーティ識別と、日本版のデコード精度は未確認です。", 327, 18, 447, 740, 24, 0)
	if rawCaptureTools {
		captureChild("BUTTON", "A: 待機30秒", idMeasureIdle, 18, 475, 180, 30, wsTabStop)
		captureChild("BUTTON", "B: 通常攻撃30秒", idMeasureAttack, 210, 475, 180, 30, wsTabStop)
		captureChild("BUTTON", "C: スキル30秒", idMeasureSkill, 402, 475, 180, 30, wsTabStop)
		captureChild("BUTTON", "保存A: 待機30秒", idRecordIdle, 18, 515, 210, 30, wsTabStop)
		captureChild("BUTTON", "保存B: 通常攻撃30秒", idRecordNormal, 240, 515, 220, 30, wsTabStop)
		captureChild("BUTTON", "保存C: スキル30秒", idRecordSkill, 472, 515, 220, 30, wsTabStop)
		captureChild("BUTTON", "保存停止", idRecordStop, 18, 551, 180, 30, wsTabStop)
		captureChild("BUTTON", "この記録3ファイルを削除", idRecordDelete, 210, 551, 270, 30, wsTabStop)
		captureChild("BUTTON", "PCAPNG候補を表示", 316, 490, 551, 260, 30, wsTabStop)
		captureChild("STATIC", "", 328, 18, 590, 740, 60, 0)
		for _, id := range []int{idMeasureIdle, idMeasureAttack, idMeasureSkill, idRecordNormal, idRecordSkill} {
			show(controls[id], false)
		}
		callText(controls[idRecordIdle], "敵味方判定用の記録を開始")
		move(controls[idRecordIdle], 18, 475, 320, 36)
		callText(controls[idRecordStop], "記録を停止して保存")
		move(controls[idRecordStop], 350, 475, 330, 36)
		callText(controls[idRecordDelete], "このフォルダーの記録を削除")
		move(controls[idRecordDelete], 18, 525, 330, 36)
		move(controls[316], 360, 525, 330, 36)
		callText(controls[327], "手動停止まで保存します（64MiB上限）。ログイン・アカウント操作中は記録しないでください。")
	}
	refreshCaptureAdapters()
	updateCaptureSettings()
	show(captureWindow, !captureEmbedded)
}
func sendString(h uintptr, msg uint32, text string) {
	p := u16(text)
	send(h, msg, 0, uintptr(unsafe.Pointer(p)))
}
func refreshCaptureAdapters() {
	if captureCtl.running() {
		return
	}
	var err error
	captureAdapterList, err = captureDevices()
	send(controls[idCaptureAdapters], 0x014b, 0, 0)
	if err != nil {
		captureSetupError = err.Error()
	} else {
		captureSetupError = ""
		for i, d := range captureAdapterList {
			sendString(controls[idCaptureAdapters], 0x0143, fmt.Sprintf("%d: %s", i, d.Label))
		}
		if len(captureAdapterList) > 0 {
			selected := -1
			for i, d := range captureAdapterList {
				if d.Suggested {
					selected = i
					break
				}
			}
			send(controls[idCaptureAdapters], 0x014e, uintptr(selected), 0)
		}
	}
}
func startOrStopCapture() {
	if captureCtl.running() {
		stopCapture()
		return
	}
	index := int(send(controls[idCaptureAdapters], 0x0147, 0, 0))
	if index < 0 || index >= len(captureAdapterList) {
		captureSetupError = "利用するアダプターを選択してください"
		return
	}
	region := "jp"
	if send(controls[idCaptureRegion], 0x0147, 0, 0) == 1 {
		region = "kr"
	}
	profile, _ := profileFor(region)
	live, e := newLiveInput(region, time.Now(), idle)
	if e != nil {
		captureSetupError = "取得セッションを作成できません"
		return
	}
	if e = captureCtl.start(captureAdapterList[index], profile); e != nil {
		captureSetupError = e.Error()
		return
	}
	enableLiveRotation(live)
	trafficMeasurements = nil
	trafficMeasureLabel = ""
	trafficMeasureNote = ""
	jpCandidateView = region == "jp"
	jpFromCapture = jpCandidateView
	jpDisplay = JPStats{}
	jpInput = nil
	if region == "jp" {
		live.Party = newDetectedConfig()
		if soloMode && nativeModern {
			live.Party.Scope = "self"
		}
		jpInput = &JPProvisionalInput{Live: live, Seen: map[string]bool{}, Roles: map[string]JPRole{}}
	}
	soloAutoEnabled = true
	soloAutoSelected = false
	soloIdentityFloor = 0
	captureInput = live
	if jpInput != nil && region == "jp" {
		captureInput = jpInput.Live
	}
	displayCaptureTotal = false
	captureActive = true
	session = nil
	historyIndex = -1
	selectedPlayer = ""
	actorListSignature = ""
	captureSetupError = ""
	errorText = "日本版はDPS。方向/回復未分類を含み、実名やパーティ構成は未確定です。"
}
func stopCapture() {
	captureCtl.stop()
	captureActive = false
	if captureInput != nil {
		_ = captureInput.tick(time.Now())
		if captureInput.Session != nil {
			session = captureInput.Session
		}
	}
}
func pumpCapture() {
	tickTrafficMeasure()
	if captureInput != nil && jpInput != nil && jpFromCapture {
		for i := 0; i < 512; i++ {
			select {
			case value := <-captureCtl.jpEvents:
				if _, e := jpInput.append(value); e != nil {
					errorText = e.Error()
					captureSetupError = e.Error()
					recordCaptureStop(errorText)
					stopCapture()
					i = 512
				}
			default:
				i = 512
			}
		}
	}
	if jpInput != nil && captureActive && time.Since(jpUpdateAt) < 250*time.Millisecond {
		updateCaptureSettings()
		return
	}
	if captureActive && captureInput != nil {
		for i := 0; i < 512; i++ {
			select {
			case event := <-captureCtl.events:
				if jpCandidateView {
					continue
				} // JP values are isolated from the DPS engine.
				if e := captureInput.append(event); e != nil {
					errorText = e.Error()
					captureSetupError = e.Error()
					recordCaptureStop(errorText)
					stopCapture()
					i = 512
				}
			default:
				i = 512
			}
		}
		captureInput.idle = idle
		if captureInput.Session != nil {
			captureInput.Session.Idle = idle
		}
		if e := captureInput.tick(time.Now()); e != nil {
			errorText = e.Error()
			recordCaptureStop(errorText)
			stopCapture()
		}
		if captureInput.Session != nil {
			session = captureInput.Session
			session.Idle = idle
			saveCompleted()
		}
		captureCtl.mu.Lock()
		captureCtl.stats.Aggregated = captureInput.Aggregated
		captureCtl.stats.AggregationSegments = captureInput.Rotations
		captureCtl.stats.LastSegmentReason = captureInput.LastRotation
		captureCtl.mu.Unlock()
		if !captureCtl.running() {
			reason := captureCtl.snapshot().Detail
			if captureSetupError != "" {
				reason = captureSetupError
			}
			recordCaptureStop(reason)
			captureActive = false
		}
		if jpCandidateView {
			jpDisplay = captureCtl.snapshot().JP
		}
		updateDisplay()
	}
	if jpFromCapture && jpInput != nil && jpInput.Live.Session != nil && time.Since(jpUpdateAt) >= 250*time.Millisecond {
		jpInput.Live.Session.Idle = idle
		_ = jpInput.tick(time.Now())
		session = jpInput.Live.Session
		soloAutomaticSelection()
		saveCompleted()
		jpUpdateAt = time.Now()
		updateDisplay()
	}
	if jpCandidateView && jpFromCapture {
		jpDisplay = captureCtl.snapshot().JP
		updateDisplay()
	}
	updateCaptureSettings()
}
func updateCaptureSettings() {
	if captureWindow == 0 {
		return
	}
	s := captureCtl.snapshot()
	if captureInput != nil {
		s.Aggregated = captureInput.Aggregated
		s.Stage = diagnosticStage(s)
	}
	record := s.Recording
	state := "停止"
	if !record.Active && record.Path != "" {
		reason := map[string]string{"manual_stop": "手動停止", "capacity_limit": "容量上限", "capture_stopped": "キャプチャ終了", "write_error": "書込エラー", "finalize_error": "保存処理エラー"}[record.Reason]
		if reason == "" {
			reason = "記録終了"
		}
		state = "停止・保存済み / " + reason
	}
	if record.Active {
		if record.Manual {
			state = fmt.Sprintf("記録中・%d秒経過／手動停止待ち", int(time.Since(record.Start).Seconds()))
		} else {
			state = fmt.Sprintf("記録中・残り約%d秒", 30-int(time.Since(record.Start).Seconds()))
		}
	}
	callText(controls[328], fmt.Sprintf("PCAPNG %s / %d件 / %dB（手動記録64MiB／旧測定16MiB上限）\r\n%s", state, record.Packets, record.Bytes, record.Path))
	detail := s.Detail
	if trafficMeasureLabel != "" {
		detail = fmt.Sprintf("%s: 残り約%d秒", trafficMeasureLabel, int(time.Until(trafficMeasureUntil).Seconds())+1)
	} else if trafficMeasureNote != "" {
		detail = trafficMeasureNote
	}
	if captureSetupError != "" {
		detail = captureSetupError
	}
	callText(controls[323], trafficStatus(s, detail))
	text := "キャプチャ開始"
	if s.Running {
		text = "キャプチャ停止"
	}
	callText(controls[idCaptureStart], text)
	if captureInput != nil && captureInput.Session != nil {
		signature := ""
		for _, p := range captureInput.Session.Log.Players {
			signature += fmt.Sprintf("%s:%v;", p.ID, p.Party)
		}
		if signature != actorListSignature {
			selected := send(controls[idCaptureActor], 0x0147, 0, 0)
			send(controls[idCaptureActor], 0x014b, 0, 0)
			for _, p := range captureInput.Session.Log.Players {
				state := "除外中"
				if p.Party {
					state = "集計対象"
				}
				sendString(controls[idCaptureActor], 0x0143, p.Name+" / "+state)
			}
			if int(selected) < 0 {
				selected = 0
			}
			send(controls[idCaptureActor], 0x014e, selected, 0)
			actorListSignature = signature
		}
	}
}
func captureCommand(id int) {
	switch id {
	case 316:
		if rawCaptureTools {
			if captureCtl.running() {
				captureSetupError = "キャプチャ停止後に読み込んでください"
				break
			}
			path, e := dialogPCAPPath(captureWindow)
			if e == nil && path != "" {
				e = loadCandidatePCAP(path)
			}
			if e != nil {
				captureSetupError = e.Error()
			}
		}
	case idRecordIdle, idRecordNormal, idRecordSkill, idRecordStop:
		if rawCaptureTools {
			label := map[int]string{idRecordIdle: "manual", idRecordNormal: "normal_attack", idRecordSkill: "skill_attack", idRecordStop: "stop"}[id]
			if recordingDirectory == "" {
				exe, _ := os.Executable()
				root := filepath.Join(filepath.Dir(exe), "captures")
				_ = os.MkdirAll(root, 0700)
				dir, e := os.MkdirTemp(root, "session-")
				if e != nil {
					captureSetupError = "記録先を作成できません"
					return
				}
				recordingDirectory = dir
			}
			if e := captureCtl.record(recordingDirectory, label); e != nil {
				captureSetupError = e.Error()
			} else {
				captureSetupError = ""
			}
		}
	case idRecordDelete:
		if rawCaptureTools {
			if captureCtl.running() {
				captureSetupError = "削除前にキャプチャを停止してください"
			} else if recordingDirectory != "" {
				if e := deleteLocalRecordings(recordingDirectory); e != nil {
					captureSetupError = "記録の削除に失敗しました"
				} else {
					captureSetupError = "このフォルダーの判定用記録を削除しました"
					captureCtl.mu.Lock()
					captureCtl.stats.Recording = RecordingStats{}
					captureCtl.mu.Unlock()
				}
			}
		}
	case idMeasureSkill:
		beginTrafficMeasure("C-skill")
	case idMeasureIdle:
		beginTrafficMeasure("A-idle")
	case idMeasureAttack:
		beginTrafficMeasure("B-attack")
	case idCaptureStart:
		startOrStopCapture()
	case idCaptureRefresh:
		refreshCaptureAdapters()
	case idCaptureInclude:
		openPartySettings()
	case idCaptureDiag:
		path, e := dialogPath(captureWindow, true)
		if e == nil && path != "" {
			s := captureCtl.snapshot()
			if captureInput != nil {
				s.Aggregated = captureInput.Aggregated
			}
			payload := map[string]any{"version": 2, "measurements": trafficMeasurements, "stats": s, "liveVerified": false, "rawPacketsSaved": s.Recording.Path != ""}
			data, _ := json.MarshalIndent(payload, "", "  ")
			if e = os.WriteFile(path, data, 0600); e != nil {
				captureSetupError = "診断JSONを書き込めません"
			}
		}
	}
	updateCaptureSettings()
}
func captureProc(window uintptr, msg uint32, w, l uintptr) uintptr {
	switch msg {
	case wmCommand:
		captureCommand(int(w & 0xffff))
		return 0
	case wmClose:
		pDestroy.Call(window)
		return 0
	case wmDestroy:
		captureWindow = 0
		return 0
	}
	v, _, _ := pDefWindow.Call(window, uintptr(msg), w, l)
	return v
}
func registerCaptureClass(brush uintptr) error {
	return registerClass("MabiDPSCapture", syscall.NewCallback(captureProc), brush)
}

// Thirty-second windows share a capture session, so flow IDs can be compared.
type TrafficMeasurement struct {
	Label    string        `json:"label"`
	Start    time.Time     `json:"start"`
	End      time.Time     `json:"end"`
	Complete bool          `json:"complete"`
	Before   CaptureStats  `json:"before"`
	After    CaptureStats  `json:"after"`
	Delta    []TrafficFlow `json:"delta"`
}

var trafficMeasureLabel, trafficMeasureNote string
var trafficMeasureStart, trafficMeasureUntil time.Time
var trafficMeasureBefore CaptureStats
var trafficMeasurements []TrafficMeasurement

func beginTrafficMeasure(label string) {
	if !captureCtl.running() {
		captureSetupError = "先にキャプチャを開始してください"
		return
	}
	if trafficMeasureLabel != "" {
		captureSetupError = "現在の30秒計測が終わるまでお待ちください"
		return
	}
	snapshot, e := captureCtl.beginStructureWindow()
	if e != nil {
		captureSetupError = e.Error()
		return
	}
	trafficMeasureBefore = snapshot
	trafficMeasureLabel = label
	trafficMeasureStart = time.Now()
	trafficMeasureUntil = trafficMeasureStart.Add(30 * time.Second)
	trafficMeasureNote = ""
	captureSetupError = ""
}
func tickTrafficMeasure() {
	if trafficMeasureLabel == "" {
		return
	}
	now := time.Now()
	running := captureCtl.running()
	if now.Before(trafficMeasureUntil) && running {
		return
	}
	after := captureCtl.snapshot()
	m := TrafficMeasurement{trafficMeasureLabel, trafficMeasureStart, now, running, trafficMeasureBefore, after, trafficDelta(trafficMeasureBefore, after)}
	trafficMeasurements = append(trafficMeasurements, m)
	trafficMeasureNote = trafficMeasureLabel + ": 計測終了（診断JSONへ保存済み）"
	if !running {
		trafficMeasureNote = trafficMeasureLabel + ": 中断・30秒未完了"
	}
	trafficMeasureLabel = ""
	exe, e := os.Executable()
	if e == nil {
		data, _ := json.MarshalIndent(map[string]any{"measurements": trafficMeasurements, "rawPacketsSaved": after.Recording.Path != "", "liveVerified": false}, "", "  ")
		e = os.WriteFile(filepath.Join(filepath.Dir(exe), "traffic-measurements.json"), data, 0600)
	}
	if e != nil {
		trafficMeasureNote = "計測終了・自動保存に失敗。診断JSONを保存してください"
	}
}

func trafficStatus(s CaptureStats, detail string) string {
	var tr, ts, ur, us, rb, sb, urb, usb, empty, reb uint64
	for _, v := range s.Traffic {
		if v.Protocol == "tcp" {
			empty += v.EmptyPayloadPackets
			reb += v.ReassembledBytes
			if v.Direction == "receive" {
				tr += v.Packets
				rb += v.PayloadBytes
			} else {
				ts += v.Packets
				sb += v.PayloadBytes
			}
		} else if v.Direction == "receive" {
			ur += v.Packets
			urb += v.PayloadBytes
		} else {
			us += v.Packets
			usb += v.PayloadBytes
		}
	}
	return fmt.Sprintf("状態: %s\r\nゲームTCP 受信%d/%dB 送信%d/%dB 空%d\r\nゲームUDP候補 受信%d/%dB 送信%d/%dB\r\n再構築%d接続/%dB 欠落%d 取得エラー%d\r\n開始%d フレーム%d 抽出%d 解析失敗%d 集計%d\r\nTLS形式%d / ドライバー欠落%d（ポート別詳細は診断JSON）\r\n%s", stageText(s.Stage), tr, rb, ts, sb, empty, ur, urb, us, usb, s.ReassembledStreams, reb, s.ReassemblyGaps, s.CaptureDecodeErrors, s.StartMarkers, s.Frames, s.DecodeSuccess, s.DecodeFailures, s.Aggregated, s.TLSStreams, s.DriverDrops, detail)
}
