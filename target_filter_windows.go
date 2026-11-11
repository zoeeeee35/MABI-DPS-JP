//go:build native && windows

package main

import "fmt"

var selectedDamageTarget string
var selectedDamageTargetContext string
var nativeTargetRows []DamageTarget

func currentTargetFilter() string {
	who, _ := soloIdentity()
	log := soloLog()
	context := who.ID
	if log.Party != nil {
		context = log.Party.Session + ":" + context
	}
	if context != selectedDamageTargetContext {
		selectedDamageTarget = ""
		selectedDamageTargetContext = context
	}
	return selectedDamageTarget
}
func targetFilterText() string {
	if id := currentTargetFilter(); id != "" {
		return "対象：" + damageTargetLabel(soloLog(), id)
	}
	return "対象：すべて"
}
func nativeTargetPanel(dc uintptr, left, right, y int32) {
	nativeTargetRows = damageTargets(baseDisplayBattle(), soloLog())
	ntxt(dc, "攻撃対象のフィルター", rect{left + 16, y + 8, right - 16, y + 39}, 0xE9E3D9, 17)
	ntxt(dc, "対象IDで選択します。画面上の位置・名前との対応は未解析です。", rect{left + 16, y + 44, right - 16, y + 70}, 0xAF997D, 12)
	ntxt(dc, "1体だけに通常攻撃し、ヒット数や最終命中時刻の変化で対象を確認してください。", rect{left + 16, y + 72, right - 16, y + 97}, 0xAF997D, 12)
	selected := currentTargetFilter()
	nbutton(dc, rect{left + 16, y + 107, left + 240, y + 143}, "すべての対象", "target-all", selected == "", 0)
	ntxt(dc, targetFilterText(), rect{left + 260, y + 109, right - 16, y + 141}, 0xADCBA6, 14)
	ntxt(dc, "DPSの時間は共通。選択対象へのダメージだけを表示。履歴の元データは全対象を保存します。", rect{left + 16, y + 151, right - 16, y + 180}, 0xAF997D, 12)
	found := selected == ""
	for i, t := range nativeTargetRows {
		yy := y + 190 + int32(i*49)
		if selected == t.ID {
			found = true
		}
		label := fmt.Sprintf("対象 %s / %dヒット・%s / 最終命中 %s", t.Label, t.Hits, formatNum(float64(t.Damage)), duration(float64(t.Last)/1000))
		if selected == t.ID {
			label += " / 選択中"
		}
		nbutton(dc, rect{left + 16, yy, right - 16, yy + 40}, label, "target-select", selected == t.ID, i)
	}
	if !found {
		yy := y + 194 + int32(len(nativeTargetRows)*49)
		ntxt(dc, "選択した対象への命中はこの表示区間にありません。必要なら「すべての対象」へ戻してください。", rect{left + 16, yy, right - 16, yy + 40}, 0xA99AC8, 12)
	}
}
