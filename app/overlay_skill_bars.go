package main

import (
	"sort"
	"strings"
)

const overlaySkillTableTop int32 = 305
const overlaySkillTableHeight int32 = 184
const overlaySkillVisibleRows = 6

type overlaySkillBar struct {
	Key, Name    string
	Hits         int
	Damage       int64
	Share, Width float64
}

func overlaySkillBars(rows []SkillDamageRow) []overlaySkillBar {
	out := []overlaySkillBar{}
	var total int64
	for _, r := range rows {
		if r.Key == "all" || strings.HasPrefix(r.Key, "unknown-child:") || r.Damage <= 0 {
			continue
		}
		out = append(out, overlaySkillBar{Key: r.Key, Name: r.Name, Hits: r.Hits, Damage: r.Damage})
		total += r.Damage
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Damage == out[j].Damage {
			return out[i].Key < out[j].Key
		}
		return out[i].Damage > out[j].Damage
	})
	for i := range out {
		out[i].Share = float64(out[i].Damage) / float64(total)
		out[i].Width = float64(out[i].Damage) / float64(out[0].Damage)
	}
	return out
}

// Keep existing button hit coordinates after inserting the table above them.
func overlayControlY(y int32) int32 {
	if y >= overlaySkillTableTop && y < overlaySkillTableTop+overlaySkillTableHeight {
		return -1
	}
	if y >= overlaySkillTableTop+overlaySkillTableHeight {
		return y - overlaySkillTableHeight
	}
	return y
}
