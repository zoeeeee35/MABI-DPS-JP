package main

import (
	"math"
	"sort"
	"strings"
)

type DamageTarget struct {
	ID, Label string
	Hits      int
	Damage    int64
	Last      int64
}

func damageTargetLabel(log CombatLog, id string) string {
	for _, c := range []*PartyConfig{log.Party, log.Detected} {
		if c != nil {
			if e, ok := c.Entities[id]; ok {
				if e.DisplayID != "" {
					return e.DisplayID
				}
				if e.Name != "" {
					return e.Name
				}
			}
		}
	}
	if len(id) > 12 {
		return id[len(id)-12:]
	}
	if id == "" {
		return "対象ID未取得"
	}
	return id
}
func damageTargets(b *Battle, log CombatLog) []DamageTarget {
	m := map[string]*DamageTarget{}
	if b != nil {
		for _, h := range b.DamageEvents {
			if h.Damage <= 0 || h.TargetID == "" {
				continue
			}
			r := m[h.TargetID]
			if r == nil {
				r = &DamageTarget{ID: h.TargetID, Label: damageTargetLabel(log, h.TargetID)}
				m[h.TargetID] = r
			}
			r.Hits++
			r.Damage += h.Damage
			if h.At > r.Last {
				r.Last = h.At
			}
		}
	}
	out := []DamageTarget{}
	for _, r := range m {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Label == out[j].Label {
			return out[i].ID < out[j].ID
		}
		return out[i].Label < out[j].Label
	})
	return out
}

// Display-only target projection. Retains the original combat/measurement window.
// All raw events and archived battles remain available when the filter is cleared.
func filterBattleTarget(b *Battle, target string) *Battle {
	if b == nil || target == "" {
		return b
	}
	out := *b
	out.Damage = 0
	out.DamageEvents = []Hit{}
	out.Players = []Row{}
	rows := map[string]*Row{}
	skills := map[string]map[string]*Skill{}
	for _, p := range b.Players {
		r := p
		r.Damage = 0
		r.Hits = 0
		r.Crits = 0
		r.DPS = 0
		r.Share = 0
		r.CritRate = 0
		r.Skills = []Skill{}
		rows[p.ID] = &r
		skills[p.ID] = map[string]*Skill{}
	}
	for _, h := range b.DamageEvents {
		if h.TargetID != target || h.Damage <= 0 {
			continue
		}
		out.DamageEvents = append(out.DamageEvents, h)
		out.Damage += h.Damage
		r := rows[h.PlayerID]
		if r == nil {
			r = &Row{Player: Player{ID: h.PlayerID}, Skills: []Skill{}}
			rows[h.PlayerID] = r
			skills[h.PlayerID] = map[string]*Skill{}
		}
		r.Damage += h.Damage
		r.Hits++
		if h.Critical {
			r.Crits++
		}
		s := skills[h.PlayerID][h.SkillID]
		if s == nil {
			s = &Skill{ID: h.SkillID}
			skills[h.PlayerID][h.SkillID] = s
		}
		s.Damage += h.Damage
		s.Hits++
		if h.Critical {
			s.Crits++
		}
	}
	den := math.Max(1, out.Duration)
	for id, r := range rows {
		r.DPS = float64(r.Damage) / den
		if out.Damage > 0 {
			r.Share = float64(r.Damage) / float64(out.Damage)
		}
		if r.Hits > 0 {
			r.CritRate = float64(r.Crits) / float64(r.Hits)
		}
		for _, s := range skills[id] {
			s.DPS = float64(s.Damage) / den
			r.Skills = append(r.Skills, *s)
		}
		sort.Slice(r.Skills, func(i, j int) bool {
			if r.Skills[i].Damage == r.Skills[j].Damage {
				return r.Skills[i].ID < r.Skills[j].ID
			}
			return r.Skills[i].Damage > r.Skills[j].Damage
		})
		out.Players = append(out.Players, *r)
	}
	sort.Slice(out.Players, func(i, j int) bool {
		if out.Players[i].Damage == out.Players[j].Damage {
			return out.Players[i].ID < out.Players[j].ID
		}
		return out.Players[i].Damage > out.Players[j].Damage
	})
	return &out
}
func targetSkillUsageLog(log CombatLog, target string) CombatLog {
	if target == "" {
		return log
	}
	out := log
	out.SkillUses = nil
	label := damageTargetLabel(log, target)
	for _, u := range log.SkillUses {
		if u.HasTail && strings.EqualFold(strings.TrimPrefix(strings.ToUpper(u.TargetCandidate), "0X"), strings.TrimPrefix(strings.ToUpper(label), "0X")) {
			out.SkillUses = append(out.SkillUses, u)
		}
	}
	return out
}
