package main

// mergeSkillDisplayRows combines presentation rows only. Source events and their
// protocol IDs remain intact; unknown rows keep their expandable structure.
func mergeSkillDisplayRows(rows []SkillDamageRow, usage CombatLog, player string, b *Battle) []SkillDamageRow {
	out := []SkillDamageRow{}
	byName := map[string]int{}
	total := int64(0)
	for _, r := range rows {
		total += r.Damage
	}
	for _, r := range rows {
		if r.Key == "unknown" || r.Name == "" {
			out = append(out, r)
			continue
		}
		if i, ok := byName[r.Name]; ok {
			dst := &out[i]
			dst.events = append(dst.events, r.events...)
			dst.names = append(dst.names, r.names...)
			dst.Uses += r.Uses
			if r.Hits > 0 && (dst.Hits == 0 || r.Min < dst.Min) {
				dst.Min = r.Min
			}
			if r.Max > dst.Max {
				dst.Max = r.Max
			}
			dst.Hits += r.Hits
			dst.Damage += r.Damage
			dst.Key = "display:" + dst.Name
			dst.Mean = 0
			if dst.Hits > 0 {
				dst.Mean = float64(dst.Damage) / float64(dst.Hits)
			}
			dst.Flags = damageFlagStatistics(dst.events, "")
		} else {
			byName[r.Name] = len(out)
			r.events = append([]Hit(nil), r.events...)
			r.names = append([]string(nil), r.names...)
			out = append(out, r)
		}
	}
	for i := range out {
		r := &out[i]
		if total > 0 {
			r.Share = float64(r.Damage) / float64(total)
		}
		if len(r.Key) < 8 || r.Key[:8] != "display:" {
			continue
		}
		// A use notification can match several original rows. Count it once after
		// combining those rows rather than summing duplicated per-row use counts.
		codes := map[string]bool{}
		for _, code := range r.names {
			codes[code] = true
		}
		r.Uses = 0
		for _, u := range usage.SkillUses {
			if player != "" && u.ActorID != player {
				continue
			}
			if b != nil && (u.At < b.Start || u.At > b.End) {
				continue
			}
			if codes[u.Code] {
				r.Uses++
			}
		}
	}
	return out
}
