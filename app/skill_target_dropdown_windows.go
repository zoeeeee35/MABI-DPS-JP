//go:build native && windows

package main

import "fmt"

var nativeSkillTargetsOpen bool
var nativeSkillTargetField, nativeSkillTargetBounds rect
var nativeSkillTargetOffset, nativeSkillTargetVisible int

func nativeSkillTargetFieldDraw(dc uintptr, b *Battle, left, right, y int32) {
	nativeTargetRows = damageTargets(baseDisplayBattle(), soloLog())
	nativeSkillTargetField = rect{left, y, right, y + 39}
	nbutton(dc, nativeSkillTargetField, targetFilterText()+"　▼", "skill-target-toggle", currentTargetFilter() != "", 0)
}
func nativeSkillTargetDropdown(dc uintptr, bottom int32) {
	rows := len(nativeTargetRows) + 1
	top := nativeSkillTargetField.Bottom + 4
	room := bottom - top - 32
	above := nativeSkillTargetField.Top - nativeBodyTop - 4 - 32
	upward := room < 42 && above > room
	if upward {
		room = above
	}
	nativeSkillTargetVisible = max(1, min(rows, int(room/42)))
	if upward {
		top = max(nativeBodyTop, nativeSkillTargetField.Top-4-28-int32(nativeSkillTargetVisible*42))
	}
	nativeSkillTargetOffset = max(0, min(max(0, rows-nativeSkillTargetVisible), nativeSkillTargetOffset))
	nativeSkillTargetBounds = rect{nativeSkillTargetField.Left, top, nativeSkillTargetField.Right, top + 28 + int32(nativeSkillTargetVisible*42)}
	nbox(dc, nativeSkillTargetBounds, 0x302219)
	nativeHits = append(nativeHits, nativeHit{nativeSkillTargetBounds, "skill-target-noop", 0})
	ntxt(dc, "攻撃対象を選択（スクロール可）", rect{nativeSkillTargetBounds.Left + 12, top + 3, nativeSkillTargetBounds.Right - 10, top + 27}, 0xDDD4C4, 12)
	for j := 0; j < nativeSkillTargetVisible; j++ {
		index := j + nativeSkillTargetOffset
		yy := top + 26 + int32(j*42)
		r := rect{nativeSkillTargetBounds.Left + 6, yy, nativeSkillTargetBounds.Right - 6, yy + 38}
		label := "すべての対象"
		selected := currentTargetFilter() == ""
		action := "target-all"
		idx := 0
		if index > 0 {
			t := nativeTargetRows[index-1]
			label = fmt.Sprintf("対象 %s / %dヒット・%s", t.Label, t.Hits, formatNum(float64(t.Damage)))
			selected = currentTargetFilter() == t.ID
			action = "target-select"
			idx = index - 1
		}
		nbutton(dc, r, label, action, selected, idx)
	}
}
