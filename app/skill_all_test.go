package main

import "testing"

func TestAllSkillDamageRowIncludesUnknownAndOverlappingFlags(t *testing.T) {
	b := &Battle{Start: 10, End: 100, DamageEvents: []Hit{
		{PlayerID: "self", Damage: 100, SkillID: "known", Flags: &DamageFlags{Critical: true, Strong: true}},
		{PlayerID: "self", Damage: 200, SkillID: "jp5243:unidentified", Flags: &DamageFlags{Combo: true, ComboAlt: true}},
		{PlayerID: "self", Damage: 300, Flags: &DamageFlags{}},
		{PlayerID: "other", Damage: 999, Flags: &DamageFlags{Critical: true}},
	}}
	r := allSkillDamageRow(b, CombatLog{}, "self")
	if r.Key != "all" || r.Hits != 3 || r.Damage != 600 || r.Min != 100 || r.Max != 300 || r.Mean != 200 {
		t.Fatal(r)
	}
	if r.Flags.Total != 3 || r.Flags.Critical != 1 || r.Flags.Strong != 1 || r.Flags.Combo != 1 || r.Flags.Normal != 1 {
		t.Fatal(r.Flags)
	}
	if b.DamageEvents[0].Damage != 100 || len(b.DamageEvents) != 4 {
		t.Fatal("input changed")
	}
}
func TestAllSkillDamageRowMissingFlagsAndEmptyBattle(t *testing.T) {
	r := allSkillDamageRow(&Battle{DamageEvents: []Hit{{Damage: 10}, {Damage: 20, Flags: &DamageFlags{Critical: true}}}}, CombatLog{}, "")
	if r.Flags.Total != 2 || r.Flags.Observed != 1 || r.Flags.Critical != 1 {
		t.Fatal(r)
	}
	empty := allSkillDamageRow(nil, CombatLog{}, "")
	if empty.Hits != 0 || empty.Mean != 0 || empty.Flags.Total != 0 {
		t.Fatal(empty)
	}
}
