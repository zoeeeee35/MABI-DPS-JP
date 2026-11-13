package main

import "testing"

func TestMergeSkillDisplayRows(t *testing.T) {
	a := Hit{Damage: 100, Flags: &DamageFlags{Critical: true}}
	c := Hit{Damage: 300, Flags: &DamageFlags{Strong: true}}
	rows := []SkillDamageRow{
		{Key: "skill:a", Name: "クリスタルエッジ", Uses: 1, Hits: 1, Damage: 100, Min: 100, Max: 100, events: []Hit{a}, names: []string{"a"}},
		{Key: "skill:b", Name: "クリスタルエッジ", Uses: 1, Hits: 1, Damage: 300, Min: 300, Max: 300, events: []Hit{c}, names: []string{"b"}},
		{Key: "unknown", Name: "未分類", Damage: 50, Children: []SkillDamageRow{{Key: "unknown-child:x"}}},
	}
	log := CombatLog{SkillUses: []SkillUse{{ActorID: "self", Code: "a"}, {ActorID: "self", Code: "b"}, {ActorID: "other", Code: "a"}}}
	// Usage-only rows and multiple matching aliases must not multiply uses.
	got := mergeSkillDisplayRows(rows, log, "self", nil)
	if got[0].Uses != 2 { t.Fatal(got[0].Uses) }; if len(got) != 2 || got[0].Damage != 400 || got[0].Hits != 2 || got[0].Mean != 200 || got[0].Min != 100 || got[0].Max != 300 {
		t.Fatalf("bad merge: %+v", got)
	}
	if got[0].Flags.Critical != 1 || got[0].Flags.Strong != 1 || got[0].Flags.Total != 2 {
		t.Fatal(got[0].Flags)
	}
	if len(got[1].Children) != 1 || rows[0].Damage != 100 || rows[0].Key != "skill:a" {
		t.Fatal("source or unknown hierarchy changed")
	}
	if got[0].Share != float64(400)/450 {
		t.Fatal(got[0].Share)
	}
}

