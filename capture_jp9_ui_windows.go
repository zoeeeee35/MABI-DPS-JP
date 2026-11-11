//go:build native && windows

package main

import (
	"fmt"
	"os"
)

var jpCandidateView bool
var jpFromCapture bool
var jpDisplay JPStats

func loadCandidatePCAP(path string) error {
	if !rawCaptureTools {
		return fmt.Errorf("このビルドでは通信記録の読み込みを利用できません")
	}
	if captureCtl.running() {
		return fmt.Errorf("キャプチャを停止してから候補を読み込んでください")
	}
	f, e := os.Open(path)
	if e != nil {
		return fmt.Errorf("PCAPNGを開けません")
	}
	s, e := replayPCAP(f, 16000)
	f.Close()
	if e != nil {
		return e
	}
	displayCaptureTotal = true
	jpFromCapture = false
	jpDisplay = s.JP
	jpCandidateView = true
	session = nil
	if s.provisionalLog != nil {
		var err error
		session, err = newSession(*s.provisionalLog, idle)
		if err != nil {
			return err
		}
		session.Log.Party = session.Log.Detected
		if session.Log.Party == nil {
			session.Log.Party = newDetectedConfig()
		}
		session.Log.Party.discover(session.Log)
		session.Playing = true
		session.Speed = speed
		_ = session.recompute()
	}
	historyIndex = -1
	soloScope()
	errorText = "実通信再生のDPS。方向/回復等の未分類イベントを含み、確定与ダメージではありません。"
	updateDisplay()
	return nil
}
func candidateSummary(s JPStats) string {
	latest := "未受信"
	if len(s.Recent) > 0 {
		latest = formatNum(float64(s.Recent[len(s.Recent)-1].Value))
	}
	return fmt.Sprintf("ダメージ候補（未照合） | 最新 %s | %d件 | 候補値合計 %s | 確定DPSは未計算", latest, s.Count, formatNum(float64(s.Sum)))
}
func updateCandidateDisplay() {
	callText(controls[4], candidateModeLabel()+" | "+candidateSummary(jpDisplay))
	callText(controls[5], "攻撃者別の候補（敵味方・値の意味は未照合）")
	send(controls[idPlayers], lbReset, 0, 0)
	for _, a := range jpDisplay.Actors {
		addList(controls[idPlayers], fmt.Sprintf("%s | 候補 %d件 | 候補値合計 %s（ダメージ未確定）", a.Actor, a.Count, formatNum(float64(a.Sum))))
	}
	callText(controls[6], "0x5243の候補履歴（直近50件・スキルID未特定）")
	send(controls[idSkills], lbReset, 0, 0)
	for i := len(jpDisplay.Recent) - 1; i >= 0; i-- {
		v := jpDisplay.Recent[i]
		addList(controls[idSkills], fmt.Sprintf("%s | %s → %s | 値 %s", v.At.Local().Format("15:04:05.000"), v.Actor, v.Target, formatNum(float64(v.Value))))
	}
	callText(controls[8], fmt.Sprintf("JP受信フレーム %d / 送信 %d / 次の0x4FF7とペア %d / 不正・欠落 %d / 残り %dB。確定DPS・戦闘履歴へは未入力。", jpDisplay.InFrames, jpDisplay.OutFrames, jpDisplay.Paired, jpDisplay.Invalid, jpDisplay.Remainder))
	lastRows = ""
	lastSkills = ""
	invalidate(overlayWindow)
}
func paintCandidateOverlay(dc uintptr, r rect) {
	draw(dc, "MABI DPS JP / ダメージ候補", rect{16, 12, r.Right - 12, 40}, 0xB5D873, overlayBold, 0x20|0x8000)
	draw(dc, candidateModeLabel()+"・画面未照合・DPS未計算", rect{16, 43, r.Right - 12, 66}, 0x91BDE2, overlayFont, 0x20|0x8000)
	draw(dc, fmt.Sprintf("候補 %d件 / 合計 %s（値）", jpDisplay.Count, formatNum(float64(jpDisplay.Sum))), rect{16, 75, r.Right - 12, 101}, 0xE9E5E0, overlayFont, 0x20|0x8000)
	if len(jpDisplay.Recent) == 0 {
		draw(dc, "0x5243候補の受信待ち", rect{16, 115, r.Right - 12, 145}, 0xE9E5E0, overlayFont, 0x20|0x8000)
	}
	for i := 0; i < 4 && i < len(jpDisplay.Recent); i++ {
		v := jpDisplay.Recent[len(jpDisplay.Recent)-1-i]
		y := int32(112 + 35*i)
		draw(dc, fmt.Sprintf("%s  %s", v.At.Local().Format("15:04:05"), v.Actor), rect{16, y, r.Right - 140, y + 28}, 0xE9E5E0, overlayFont, 0x20|0x8000)
		draw(dc, formatNum(float64(v.Value)), rect{r.Right - 135, y, r.Right - 16, y + 28}, 0xB5D873, overlayBold, 0x20|0x2|0x8000)
	}
}

func candidateModeLabel() string {
	if !jpFromCapture {
		return "保存データ"
	}
	if captureCtl.running() {
		return "実受信中"
	}
	return "実受信・停止"
}

func isProvisionalDisplay() bool {
	if historyIndex >= 0 && historyIndex < len(history) {
		return history[historyIndex].Archive.Request.Log.Source == jpProvisionalSource
	}
	return session != nil && session.Log.Source == jpProvisionalSource
}

func overlayResizeHit(x, y, w, h int32) uintptr {
	left, right, top, bottom := x < 8, x >= w-8, y < 8, y >= h-8
	if top && left {
		return 13
	}
	if top && right {
		return 14
	}
	if bottom && left {
		return 16
	}
	if bottom && right {
		return 17
	}
	if left {
		return 10
	}
	if right {
		return 11
	}
	if top {
		return 12
	}
	if bottom {
		return 15
	}
	return 1
}
func sessionResultForTest() any {
	if session == nil {
		return nil
	}
	return session.Result
}
