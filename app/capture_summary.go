package main

import (
	"math"
	"sort"
)

// captureSummary is a display-only summary; it never changes combat boundaries or archives.
func captureSummary(battles []Battle) *Battle {
	if len(battles) == 0 {
		return nil
	}
	out := Battle{Number: 0, Start: battles[0].Start, End: battles[len(battles)-1].End, Active: battles[len(battles)-1].Active, Reason: "capture-summary", Players: []Row{}, DamageEvents: []Hit{}}
	rows := map[string]*Row{}
	skills := map[string]map[string]*Skill{}
	for _, b := range battles {
		out.Damage += b.Damage
		out.DamageEvents = append(out.DamageEvents, b.DamageEvents...)
		for _, p := range b.Players {
			r := rows[p.ID]
			if r == nil {
				r = &Row{Player: p.Player, DamageStatus: p.DamageStatus, Skills: []Skill{}}
				rows[p.ID] = r
				skills[p.ID] = map[string]*Skill{}
			}
			r.Damage += p.Damage
			r.Hits += p.Hits
			r.Crits += p.Crits
			for _, s := range p.Skills {
				sk := skills[p.ID][s.ID]
				if sk == nil {
					sk = &Skill{ID: s.ID}
					skills[p.ID][s.ID] = sk
				}
				sk.Damage += s.Damage
				sk.Hits += s.Hits
				sk.Crits += s.Crits
			}
		}
	}
	out.Duration = float64(out.End-out.Start) / 1000
	den := math.Max(1, out.Duration)
	for id, r := range rows {
		if r.DamageStatus != "" {
			r.DamageStatus = "candidate"
			if r.Hits == 0 {
				r.DamageStatus = "unobserved"
			}
		}
		r.DPS = float64(r.Damage) / den
		r.CritRate = float64(r.Crits) / math.Max(1, float64(r.Hits))
		if out.Damage > 0 {
			r.Share = float64(r.Damage) / float64(out.Damage)
		}
		for _, s := range skills[id] {
			s.DPS = float64(s.Damage) / den
			r.Skills = append(r.Skills, *s)
		}
		sort.Slice(r.Skills, func(i, j int) bool { return r.Skills[i].ID < r.Skills[j].ID })
		out.Players = append(out.Players, *r)
	}
	sort.Slice(out.Players, func(i, j int) bool {
		a, b := out.Players[i], out.Players[j]
		if a.Damage == b.Damage {
			return a.ID < b.ID
		}
		return a.Damage > b.Damage
	})
	return &out
}
