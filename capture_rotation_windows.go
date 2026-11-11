//go:build native && windows

package main

import "fmt"

func enableLiveRotation(live *LiveInput) {
	live.OnRotate = func(log CombatLog, end int64, reason string) error {
		q := Request{Log: log, Until: end, Idle: live.idle}
		r, e := analyze(q)
		if e != nil {
			return e
		}
		for _, b := range r.Battles {
			if b.Active {
				return fmt.Errorf("自動保存する区間が未終了です")
			}
			key := archiveKey(q, b)
			if remembered[key] {
				continue
			}
			if _, e = saveBattleSegment(historyDir, q, b, reason); e != nil {
				return e
			}
			remembered[key] = true
		}
		if soloMeasurement != nil && soloMeasurement.Live == live {
			run := soloMeasurement
			run.Carry = mergeTimedLogs(run.Carry, log, run.Party, run.Start, run.End)
		}
		refreshHistory()
		errorText = "取得を継続しています。集計上限のため直前の区間を履歴に自動保存しました。"
		return nil
	}
}
