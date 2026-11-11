package main

// Ownership is not established by traffic. These helpers only detect stale
// selections and rank fresh activity; callers must label automatic choices.
func actorRecentActivity(log CombatLog, id string) (last int64) {
	last = -1
	for _, e := range log.Events {
		if e.PlayerID == id && e.Damage > 0 && e.At > last {
			last = e.At
		}
	}
	for _, u := range log.SkillUses {
		if u.ActorID == id && u.At > last {
			last = u.At
		}
	}
	return
}
func recentSoloCandidate(log CombatLog, now, floor int64) (string, int) {
	start := max(floor, now-15000)
	counts := map[string]int{}
	sums := map[string]int64{}
	skills := map[string]bool{}
	for _, u := range log.SkillUses {
		if u.At >= start && u.At <= now {
			skills[u.ActorID] = true
		}
	}
	for _, e := range log.Events {
		if e.At >= start && e.At <= now && e.Damage > 0 && skills[e.PlayerID] {
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
		return "", 0
	}
	return best, counts[best]
}
func staleSelfCanSwitch(log CombatLog, current, next string, now int64, hits int) bool {
	if current == "" {
		return next != ""
	}
	if next == "" || next == current || hits < 3 {
		return false
	}
	last := actorRecentActivity(log, current)
	return (last < 0 || now-last >= 15000) && now-actorRecentActivity(log, next) <= 5000
}
