package main

import (
	"fmt"
	"sort"
	"strings"
)

type SkillDamageRow struct {
	Children          []SkillDamageRow
	Key, Name, Detail string
	Uses, Hits        int
	Damage, Max, Min  int64
	Mean, Share       float64
	Flags             FlagStatistics
	events            []Hit
	names             []string
}

func linkedSkillRows(b *Battle, log CombatLog, player string) []SkillDamageRow {
	rows := map[string]*SkillDamageRow{}
	var total int64
	ensure := func(key, name string) *SkillDamageRow {
		r := rows[key]
		if r == nil {
			r = &SkillDamageRow{Key: key, Name: name}
			rows[key] = r
		}
		return r
	}
	if b != nil {
		for _, h := range b.DamageEvents {
			if h.Damage <= 0 || (player != "" && h.PlayerID != player) {
				continue
			}
			key, name, detail := "unknown", "スキル未特定", "結合なし／識別子未対応"
			names := []string{}
			if h.Link != nil && h.Link.Paired {
				names = h.Link.Names
				if len(names) == 1 {
					key = "skill:" + names[0]
					name = names[0]
					detail = fmt.Sprintf("%s / 0x%08X / 一意対応（仮説）", name, h.Link.Effect)
				} else if len(names) > 1 {
					key = fmt.Sprintf("group:%08X:", h.Link.Effect) + strings.Join(names, "|")
					name = fmt.Sprintf("0x%08X スキル群", h.Link.Effect)
					detail = strings.Join(names, " / ")
				}
			} else if h.Link == nil && h.SkillID != "jp5243:unidentified" {
				key = "skill:" + h.SkillID
				name = h.SkillID
				names = []string{h.SkillID}
				detail = "入力イベント"
			}
			r := ensure(key, name)
			r.Detail = detail
			r.names = names
			r.Hits++
			r.Damage += h.Damage
			r.events = append(r.events, h)
			if r.Hits == 1 || h.Damage < r.Min {
				r.Min = h.Damage
			}
			if h.Damage > r.Max {
				r.Max = h.Damage
			}
			total += h.Damage
		}
	}
	for _, u := range log.SkillUses {
		if player != "" && u.ActorID != player {
			continue
		}
		if b != nil && (u.At < b.Start || u.At > b.End) {
			continue
		}
		matched := false
		for _, r := range rows {
			for _, name := range r.names {
				if name == u.Code {
					r.Uses++
					matched = true
					break
				}
			}
		}
		if !matched {
			r := ensure("skill:"+u.Code, u.Code)
			r.Uses++
			r.Detail = "使用通知のみ／対応ダメージ未取得"
		}
	}
	out := []SkillDamageRow{}
	for _, r := range rows {
		if r.Hits > 0 {
			r.Mean = float64(r.Damage) / float64(r.Hits)
		}
		if total > 0 {
			r.Share = float64(r.Damage) / float64(total)
		}
		r.Flags = damageFlagStatistics(r.events, "")
		out = append(out, *r)
	}
	return out
}
func sortLinkedSkills(rows []SkillDamageRow, column int, desc bool) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		missing := func(r SkillDamageRow) bool {
			if column == 12 {
				_, ok := r.Flags.additionalRate()
				return !ok
			}
			if column >= 8 {
				return r.Flags.Observed == 0 || r.Flags.Total == 0
			}
			return column >= 2 && column <= 7 && r.Hits == 0
		}
		if missing(a) != missing(b) {
			return !missing(a)
		}
		rate := func(n int, total int) float64 {
			if total == 0 {
				return 0
			}
			return float64(n) / float64(total)
		}
		x, y := float64(0), float64(0)
		switch column {
		case 0:
			if a.Name == b.Name {
				return a.Key < b.Key
			}
			if desc {
				return a.Name > b.Name
			}
			return a.Name < b.Name
		case 1:
			x, y = float64(a.Uses), float64(b.Uses)
		case 2:
			x, y = float64(a.Hits), float64(b.Hits)
		case 3:
			x, y = float64(a.Damage), float64(b.Damage)
		case 4:
			x, y = float64(a.Max), float64(b.Max)
		case 5:
			x, y = a.Mean, b.Mean
		case 6:
			x, y = float64(a.Min), float64(b.Min)
		case 7:
			x, y = a.Share, b.Share
		case 8:
			x, y = rate(a.Flags.Normal, a.Flags.Total), rate(b.Flags.Normal, b.Flags.Total)
		case 9:
			x, y = rate(a.Flags.Critical, a.Flags.Total), rate(b.Flags.Critical, b.Flags.Total)
		case 10:
			x, y = rate(a.Flags.Strong, a.Flags.Total), rate(b.Flags.Strong, b.Flags.Total)
		case 11:
			x, y = rate(a.Flags.Combo, a.Flags.Total), rate(b.Flags.Combo, b.Flags.Total)
		case 12:
			x, _ = a.Flags.additionalRate()
			y, _ = b.Flags.additionalRate()
		}
		if x == y {
			return a.Key < b.Key
		}
		if desc {
			return x > y
		}
		return x < y
	})
}

// Aggregate original hits once, including unclassified skills. Never average per-skill rates.
func allSkillDamageRow(b *Battle, log CombatLog, player string) SkillDamageRow {
	r := SkillDamageRow{Key: "all", Name: "ALL（全スキル）"}
	if b != nil {
		for _, h := range b.DamageEvents {
			if h.Damage <= 0 || (player != "" && h.PlayerID != player) {
				continue
			}
			r.events = append(r.events, h)
			r.Hits++
			r.Damage += h.Damage
			if r.Hits == 1 || h.Damage < r.Min {
				r.Min = h.Damage
			}
			if h.Damage > r.Max {
				r.Max = h.Damage
			}
		}
	}
	for _, u := range log.SkillUses {
		if player != "" && u.ActorID != player {
			continue
		}
		if b != nil && (u.At < b.Start || u.At > b.End) {
			continue
		}
		r.Uses++
	}
	if r.Hits > 0 {
		r.Mean = float64(r.Damage) / float64(r.Hits)
		r.Share = 1
	}
	r.Flags = damageFlagStatistics(r.events, "")
	return r
}
