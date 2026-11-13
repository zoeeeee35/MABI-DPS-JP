package main

import (
	"fmt"
	"sort"
)

type SkillUsageRow struct {
	Code        string `json:"code"`
	Uses        int    `json:"uses"`
	First, Last int64
}

func ownSkillUsage(log CombatLog, id string, until int64) []SkillUsageRow {
	m := map[string]*SkillUsageRow{}
	for _, u := range log.SkillUses {
		if u.ActorID != id || u.At > until {
			continue
		}
		r := m[u.Code]
		if r == nil {
			r = &SkillUsageRow{Code: u.Code, First: u.At}
			m[u.Code] = r
		}
		r.Uses++
		r.Last = u.At
	}
	out := []SkillUsageRow{}
	for _, r := range m {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Uses == out[j].Uses {
			return out[i].Code < out[j].Code
		}
		return out[i].Uses > out[j].Uses
	})
	return out
}
func selectSelf(c *PartyConfig, id string) error {
	if c == nil {
		return fmt.Errorf("自分のIDを選択してください")
	}
	e, ok := c.Entities[id]
	if !ok {
		return fmt.Errorf("自分のIDを選択してください")
	}
	for key, p := range c.Entities {
		if p.Role == RoleSelf {
			p.Role = RoleOutside
			c.Entities[key] = p
		}
	}
	e.Role = RoleSelf
	e.Hidden = false
	e.Evidence = "manual"
	c.Entities[id] = e
	c.Scope = "self"
	return nil
}

func soloBattleView(b *Battle, id string) *Battle {
	if b == nil {
		return nil
	}
	copy := *b
	copy.Players = []Row{}
	copy.DamageEvents = []Hit{}
	copy.Damage = 0
	for _, p := range b.Players {
		if p.ID == id {
			copy.Damage = p.Damage
			if p.Damage > 0 {
				p.Share = 1
			}
			copy.Players = append(copy.Players, p)
		}
	}
	for _, e := range b.DamageEvents {
		if e.PlayerID == id {
			copy.DamageEvents = append(copy.DamageEvents, e)
		}
	}
	if len(copy.Players) == 0 {
		return nil
	}
	return &copy
}

// Evidence is counted before the display scope filter; selecting self must not
// erase the evidence needed to select a different identity.
func soloEvidence(log CombatLog, id string) (hits int, damage int64, uses int) {
	for _, e := range log.Events {
		if e.PlayerID == id && e.Damage > 0 {
			hits++
			damage += e.Damage
		}
	}
	for _, u := range log.SkillUses {
		if u.ActorID == id {
			uses++
		}
	}
	return
}

// Auto selection is a traffic heuristic, never proof of ownership.
func soloAutoCandidate(log CombatLog, until, idle int64) string {
	start := int64(0)
	last := int64(-1)
	for _, e := range log.Events {
		if e.At > until || e.Damage <= 0 {
			continue
		}
		if last >= 0 && e.At-last >= idle {
			start = e.At
		}
		last = e.At
	}
	if last < 0 || until-last >= idle {
		return ""
	}
	counts := map[string]int{}
	sums := map[string]int64{}
	skills := map[string]bool{}
	for _, u := range log.SkillUses {
		if u.At <= until {
			skills[u.ActorID] = true
		}
	}
	for _, e := range log.Events {
		if e.At >= start && e.At <= until && e.Damage > 0 && skills[e.PlayerID] {
			counts[e.PlayerID]++
			sums[e.PlayerID] += e.Damage
		}
	}
	best := ""
	tie := false
	for id, n := range counts {
		if best == "" || n > counts[best] || (n == counts[best] && sums[id] > sums[best]) {
			best = id
			tie = false
		} else if n == counts[best] && sums[id] == sums[best] {
			tie = true
		}
	}
	if tie {
		return ""
	}
	return best
}
func soloSkillRange(b *Battle, id string) (int64, float64, int64, bool) {
	if b == nil {
		return 0, 0, 0, false
	}
	var lo, hi, sum int64
	n := 0
	for _, e := range b.DamageEvents {
		if e.SkillID != id || e.Damage <= 0 {
			continue
		}
		if n == 0 || e.Damage < lo {
			lo = e.Damage
		}
		if e.Damage > hi {
			hi = e.Damage
		}
		sum += e.Damage
		n++
	}
	if n == 0 {
		return 0, 0, 0, false
	}
	return hi, float64(sum) / float64(n), lo, true
}
