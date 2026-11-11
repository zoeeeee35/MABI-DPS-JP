package main

import (
	"fmt"
	"sort"
)

// Merge only the selected player's timed window. Capture segments may rotate,
// but event IDs and the capture clock remain stable across those segments.
func mergeTimedLogs(old, current CombatLog, party *PartyConfig, start, end int64) CombatLog {
	out := current
	out.Party = party
	out.Events = nil
	out.SkillUses = nil
	out.Players = nil
	self := ""
	if party != nil {
		for id, e := range party.Entities {
			if e.Role == RoleSelf {
				self = id
				break
			}
		}
	}
	players := map[string]Player{}
	for _, l := range []CombatLog{old, current} {
		for _, p := range l.Players {
			if self == "" || p.ID == self {
				players[p.ID] = p
			}
		}
	}
	for _, p := range players {
		out.Players = append(out.Players, p)
	}
	sort.Slice(out.Players, func(i, j int) bool { return out.Players[i].ID < out.Players[j].ID })
	seen := map[string]bool{}
	uses := map[string]int{}
	for n, l := range []CombatLog{old, current} {
		local := map[string]int{}
		for _, h := range l.Events {
			if h.At < start || h.At >= end || self != "" && h.PlayerID != self || seen[h.ID] {
				continue
			}
			seen[h.ID] = true
			out.Events = append(out.Events, h)
		}
		for _, u := range l.SkillUses {
			if u.At < start || u.At >= end || self != "" && u.ActorID != self {
				continue
			}
			key := fmt.Sprintf("%s/%d/%s/%d/%s/%t", u.ActorID, u.At, u.Code, u.EffectCandidate, u.TargetCandidate, u.HasTail)
			local[key]++
			if n == 0 || local[key] > uses[key] {
				out.SkillUses = append(out.SkillUses, u)
			}
		}
		for k, v := range local {
			if v > uses[k] {
				uses[k] = v
			}
		}
	}
	sort.SliceStable(out.Events, func(i, j int) bool {
		if out.Events[i].At == out.Events[j].At {
			return out.Events[i].ID < out.Events[j].ID
		}
		return out.Events[i].At < out.Events[j].At
	})
	return out
}
