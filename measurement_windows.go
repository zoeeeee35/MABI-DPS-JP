//go:build native && windows

package main

import (
	"encoding/json"
	"fmt"
	"time"
)

type soloTimedRun struct {
	Carry      CombatLog
	Live       *LiveInput
	Start, End int64
	Party      *PartyConfig
	Preview    *Battle
	LastUpdate time.Time
}

var soloMeasurement *soloTimedRun
var soloMeasurementNotice string

func beginSoloMeasurement() {
	if !captureActive {
		ensureNativeCapture()
		if !captureCtl.running() {
			startOrStopCapture()
		}
	}
	if !captureActive || jpInput == nil || jpInput.Live == nil {
		errorText = "計測を開始できません：" + captureSetupError
		if captureSetupError == "" {
			errorText = "計測を開始できません。日本版の取得設定を確認してください。"
		}
		soloMeasurementNotice = errorText
		nativePage = "設定"
		nativeScroll = 0
		show(mainWindow, true)
		updateDisplay()
		return
	}
	soloMeasurementNotice = ""
	historyIndex = -1
	now := time.Now()
	start := now.Add(soloPreparation).Sub(jpInput.Live.origin).Milliseconds()
	if start < 0 {
		return
	}
	soloMeasurement = &soloTimedRun{Live: jpInput.Live, Start: start, End: start + soloTimerDuration(soloTimerMinutes).Milliseconds(), Preview: &Battle{Start: start, End: start, Players: []Row{}, DamageEvents: []Hit{}}}
	bindSoloTimedIdentity(soloMeasurement)
	soloTimerEnd = jpInput.Live.origin.Add(time.Duration(soloMeasurement.End) * time.Millisecond)
	errorText = fmt.Sprintf("5秒後に%d分計測を開始します。準備中のダメージは対象外です。", soloTimerMinutes)
	updateDisplay()
}
func cancelSoloMeasurement() { soloMeasurement = nil; soloTimerEnd = time.Time{}; updateDisplay() }
func updateSoloMeasurement(now time.Time) {
	run := soloMeasurement
	if run == nil {
		return
	}
	if !captureActive || captureInput != run.Live {
		soloMeasurement = nil
		soloTimerEnd = time.Time{}
		errorText = "時間指定計測は中止されました（取得停止・リセット）。"
		updateDisplay()
		return
	}
	if run.Live.Session == nil {
		if !now.Before(run.Live.origin.Add(time.Duration(run.End) * time.Millisecond)) {
			soloMeasurement = nil
			soloTimerEnd = time.Time{}
			errorText = "計測区間にダメージを受信しませんでした。取得設定を確認してください。"
			soloMeasurementNotice = errorText
			updateDisplay()
		}
		return
	}
	if now.Sub(run.LastUpdate) < 250*time.Millisecond {
		return
	}
	run.LastUpdate = now

	end := min(run.End, now.Sub(run.Live.origin).Milliseconds())
	if end <= run.Start {
		return
	}
	if !bindSoloTimedIdentity(run) {
		run.Preview = &Battle{Start: run.Start, End: end, Duration: float64(end-run.Start) / 1000, Players: []Row{}, DamageEvents: []Hit{}}
		if end >= run.End {
			soloMeasurement = nil
			soloTimerEnd = time.Time{}
			errorText = "自分のIDを判定できなかったため、時間指定の結果は保存していません。設定 → 詳細 → 自分IDを修正から確認してください。"
		} else {
			errorText = "計測区間を記録中：攻撃後の自分ID判定を待っています。"
		}
		updateDisplay()
		return
	}
	if run.Preview != nil && run.Preview.Duration == 0 {
		errorText = fmt.Sprintf("%d分計測中。終了時に区間結果を履歴へ保存します。", soloTimerMinutes)
	}
	merged := mergeTimedLogs(run.Carry, run.Live.Session.Log, run.Party, run.Start, run.End)
	q := Request{Log: merged, Until: end, Idle: idle, Window: &MeasurementWindow{Start: run.Start, End: end}}
	q.Log.Party = run.Party
	r, e := analyze(q)
	if e != nil {
		errorText = e.Error()
		return
	}
	b := r.Battles[0]
	b.Active = end < run.End
	run.Preview = &b
	if end < run.End {
		updateDisplay()
		return
	}
	path, e := saveBattle(historyDir, q, b)
	if e != nil {
		errorText = "時間指定の保存失敗：" + e.Error()
		return
	}
	soloMeasurement = nil
	refreshHistory()
	for i, h := range history {
		if h.Path == path {
			historyIndex = i
			break
		}
	}
	errorText = fmt.Sprintf("%.0f秒の計測結果を履歴へ保存しました。", b.Duration)
	updateDisplay()
}

func historyMeasurementLabel(h SavedBattle) string {
	if h.Archive.Request.Window != nil {
		return fmt.Sprintf("時間指定 %.0f秒    累計 %s    DPS %s", h.Battle.Duration, formatNum(float64(h.Battle.Damage)), formatNum(float64(h.Battle.Damage)/max(1, h.Battle.Duration)))
	}
	return fmt.Sprintf("戦闘%d    累計 %s    %.1f秒", h.Battle.Number, formatNum(float64(h.Battle.Damage)), h.Battle.Duration)
}

func soloPreparationRemaining(now time.Time) int {
	if soloMeasurement == nil || soloMeasurement.Live == nil {
		return 0
	}
	return soloCountdown(soloMeasurement.Live.origin.Add(time.Duration(soloMeasurement.Start)*time.Millisecond), now)
}

// Resolve only after the existing self detector (or manual selection) identifies
// an actor. Keep the original window so earlier captured hits are backfilled.
func bindSoloTimedIdentity(run *soloTimedRun) bool {
	if run.Party != nil {
		return true
	}
	who, ok := soloIdentity()
	if !ok {
		return false
	}
	data, err := json.Marshal(soloConfig())
	if err != nil {
		return false
	}
	var party PartyConfig
	if json.Unmarshal(data, &party) != nil || selectSelf(&party, who.ID) != nil {
		return false
	}
	run.Party = &party
	return true
}
