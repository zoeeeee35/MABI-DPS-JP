//go:build native && windows

package main

import "fmt"

func nativeSoloSelectPanel(dc uintptr, left, right, y int32) {
	ntxt(dc, "自分のIDを指定", rect{left + 12, y + 8, right - 10, y + 36}, 0xDDC4A6, 17)
	ntxt(dc, "自動推定：戦闘内の通知件数・累計とスキル通知で選びます。他人を選ぶ可能性があります。", rect{left + 12, y + 45, right - 10, y + 74}, 0xB5C0D7, 12)
	nbutton(dc, rect{left + 12, y + 78, left + 300, y + 112}, "ダメージ検出のみ / 全ID 切替", "solo-filter", soloShowAll, 0)
	nbutton(dc, rect{left + 310, y + 78, right - 12, y + 112}, "戦闘ごとの自動推定 ON/OFF", "solo-autotoggle", soloAutoEnabled, 0)
	who, selected := soloIdentity()
	for i, e := range soloCandidates() {
		yy := y + 122 + int32(i*47)
		label := e.Class
		if label == "" {
			label = "職業不明"
		}
		label += " " + entityDisplayID(e)
		hits, damage, uses := soloEvidence(soloLog(), e.ID)
		label += fmt.Sprintf(" / %d件・%s / スキル%d回", hits, formatNum(float64(damage)), uses)
		if selected && e.ID == who.ID {
			if soloAutoSelected {
				label += " / 自動推定（所有者未確認）"
			} else {
				label += " / 自分（手動指定）"
			}
		} else {
			label += "  / 自分として指定"
		}
		nbutton(dc, rect{left + 10, yy, right - 10, yy + 38}, label, "solo-select", selected && e.ID == who.ID, i)
	}
}

var unknownEffectsExpanded bool
var skillSortColumn = 3
var skillSortDescending = true

func nativeSoloRows(b *Battle) []SkillDamageRow {
	who, ok := soloIdentity()
	if soloMode && !ok {
		return nil
	}
	effectNamesOnce.Do(loadEffectNames)
	usageLog := targetSkillUsageLog(soloLog(), currentTargetFilter())
	rows := classifyUnknownEffectRows(linkedSkillRows(b, usageLog, who.ID), soloLog(), effectNames)
	for i := range rows {
		rows[i].Name = skillRowDisplayName(rows[i])
	}
	sortLinkedSkills(rows, skillSortColumn, skillSortDescending)
	return append([]SkillDamageRow{allSkillDamageRow(b, usageLog, who.ID)}, expandUnknownEffectRows(rows, unknownEffectsExpanded)...)
}
func nativeSoloSkillPanel(dc uintptr, b *Battle, left, right, y int32) {
	nativeSkillTargetFieldDraw(dc, b, left+10, right-10, y)
	ntxt(dc, "ここで攻撃対象を選択。分析・オーバーレイにも同じ条件を適用します。", rect{left + 16, y + 43, right - 16, y + 67}, 0xAF997D, 12)
	y += 78
	rows := nativeSoloRows(b)
	height := int32(150 + 44*max(1, len(rows)))
	nbox(dc, rect{left, y, right, y + height}, 0x261912)
	ntxt(dc, "スキル別ダメージ・判定率（ALLは全スキル合計／同じ表示区間）", rect{left + 16, y + 12, right - 12, y + 39}, 0xDFC9B0, 14)
	headers := []string{"スキル名／スキル群", "使用", "ヒット", "総ダメージ", "最大", "平均", "最小", "通常※", "クリ率", "強打率", "連打率", "追加率"}
	parts := []int32{0, 230, 270, 310, 400, 470, 540, 610, 680, 760, 840, 920, 1000}
	pos := func(i int) int32 { return left + 14 + (right-left-28)*parts[i]/1000 }
	sortColumns := []int{0, 1, 2, 3, 4, 5, 6, 8, 9, 10, 11, 12}
	for i, label := range headers {
		column := sortColumns[i]
		if column == skillSortColumn {
			if skillSortDescending {
				label += " ↓"
			} else {
				label += " ↑"
			}
		}
		r := rect{pos(i), y + 48, pos(i+1) - 2, y + 80}
		if column >= 0 {
			color := uint32(0x453125)
			if column == skillSortColumn {
				color = 0x68513A
			}
			nbox(dc, r, color)
			ncenter(dc, label, r, 0xD1B294, 14)
			clipped := r
			clipped.Top = max(208, clipped.Top)
			clipped.Bottom = min(nativeBottom, clipped.Bottom)
			if clipped.Bottom > clipped.Top {
				nativeHits = append(nativeHits, nativeHit{clipped, "skill-sort", column})
			}
		} else {
			nbox(dc, r, 0x453125)
			ncenter(dc, label, r, 0xD1B294, 14)
		}
	}
	for i, r := range rows {
		yy := y + 87 + int32(i*44)
		stat := func(n int) string {
			if r.Flags.Observed == 0 {
				return "—"
			}
			return fmt.Sprintf("%.0f%%", 100*float64(n)/float64(r.Flags.Total))
		}
		label := r.Name
		if r.Key == "unknown" {
			label = fmt.Sprintf("▶ 未分類（%d項目）", len(r.Children))
			if unknownEffectsExpanded {
				label = fmt.Sprintf("▼ 未分類（%d項目）", len(r.Children))
			}
			hit := rect{pos(0), yy, pos(1), yy + 34}
			hit.Top = max(208, hit.Top)
			hit.Bottom = min(nativeBottom, hit.Bottom)
			if hit.Bottom > hit.Top {
				nativeHits = append(nativeHits, nativeHit{hit, "unknown-effects", 0})
			}
		}
		if len(r.Key) > 14 && r.Key[:14] == "unknown-child:" {
			label = "　└ " + r.Name
		}
		additional := "—"
		if rate, ok := r.Flags.additionalRate(); ok {
			additional = fmt.Sprintf("%.1f%%", 100*rate)
		}
		values := []string{label, fmt.Sprint(r.Uses), fmt.Sprint(r.Hits), formatNum(float64(r.Damage)), formatNum(float64(r.Max)), formatNum(r.Mean), formatNum(float64(r.Min)), stat(r.Flags.Normal), stat(r.Flags.Critical), stat(r.Flags.Strong), stat(r.Flags.Combo), additional}
		if r.Hits == 0 {
			for k := 2; k <= 6; k++ {
				values[k] = "—"
			}
		}
		if r.Key == "all" {
			nbox(dc, rect{left + 14, yy - 2, right - 14, yy + 36}, 0x453125)
		}
		for col, value := range values {
			ntxt(dc, value, rect{pos(col) + 4, yy, pos(col+1) - 3, yy + 34}, 0xD2BFA8, 15)
		}

	}
	yy := y + 92 + int32(44*max(1, len(rows)))
	stats := SkillLinkStats{}
	if b != nil {
		stats = skillLinkStats(b.DamageEvents)
	}
	unknownHits, effectHits := 0, 0
	for _, r := range rows {
		if r.Key == "unknown" {
			unknownHits = r.Hits
		}
		if len(r.Key) >= 7 && r.Key[:7] == "effect:" {
			effectHits += r.Hits
		}
	}
	ntxt(dc, fmt.Sprintf("結合 %d/%d件・一意 %d件・スキル群 %d件・効果ID訳 %d件・未分類 %d件。分類は実測仮説です。", stats.Paired, stats.Damage, stats.Unique, stats.Group, effectHits, unknownHits), rect{left + 20, yy, right - 16, yy + 25}, 0xADD397, 11)
	ntxt(dc, "※通常は全候補OFF。追加率（暫定）＝追加攻撃 ÷ 追加攻撃以外のヒット。使用回数はスキル通知数です。", rect{left + 20, yy + 26, right - 16, yy + 51}, 0xA99AC8, 11)
}
