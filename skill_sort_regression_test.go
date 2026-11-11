package main

import (
	"reflect"
	"testing"
)

func TestSkillPercentageSortUsesRateNotCounts(t *testing.T) {
	for _, col := range []int{8, 9, 10, 11} {
		for _, desc := range []bool{false, true} {
			rows := []SkillDamageRow{{Key: "many", Hits: 100, Flags: FlagStatistics{Total: 100, Observed: 100, Normal: 20, Critical: 20, Strong: 20, Combo: 20}}, {Key: "high-rate", Hits: 10, Flags: FlagStatistics{Total: 10, Observed: 10, Normal: 9, Critical: 9, Strong: 9, Combo: 9}}, {Key: "unknown", Hits: 1}}
			sortLinkedSkills(rows, col, desc)
			want := "many"
			if desc {
				want = "high-rate"
			}
			if rows[0].Key != want || rows[2].Key != "unknown" {
				t.Fatal(col, desc, rows)
			}
		}
	}
}
func TestSkillSortMissingLastAndNameTiesStable(t *testing.T) {
	for _, col := range []int{2, 3, 4, 5, 6, 7} {
		for _, desc := range []bool{false, true} {
			rows := []SkillDamageRow{{Key: "missing"}, {Key: "known", Hits: 1, Damage: 10, Min: 10, Max: 10, Mean: 10, Share: 1}}
			sortLinkedSkills(rows, col, desc)
			if rows[0].Key != "known" {
				t.Fatal(col, desc)
			}
		}
	}
	for _, desc := range []bool{false, true} {
		a := []SkillDamageRow{{Key: "b", Name: "通常攻撃"}, {Key: "a", Name: "通常攻撃"}}
		b := []SkillDamageRow{a[1], a[0]}
		sortLinkedSkills(a, 0, desc)
		sortLinkedSkills(b, 0, desc)
		if !reflect.DeepEqual(a, b) {
			t.Fatal(a, b)
		}
	}
}
func TestTimedLogCarryAcrossRotation(t *testing.T) {
	party := newDetectedConfig()
	party.Scope = "self"
	party.Entities["self"] = PartyEntity{ID: "self", Role: RoleSelf}
	old := CombatLog{Version: 1, Region: "jp", Source: jpProvisionalSource, Players: []Player{{ID: "self", Name: "self", Party: true}}, Events: []Hit{{ID: "1", PlayerID: "self", Type: "damage", SkillID: "s", TargetID: "enemy", At: 1000, Damage: 100}}, SkillUses: []SkillUse{{At: 999, ActorID: "self", Code: "skill"}}}
	current := old
	current.Events = []Hit{{ID: "2", PlayerID: "self", Type: "damage", SkillID: "s", TargetID: "enemy", At: 61000, Damage: 200}}
	merged := mergeTimedLogs(old, current, party, 0, 120000)
	if len(merged.Events) != 2 || len(merged.SkillUses) != 1 {
		t.Fatal(merged)
	}
	r, e := analyze(Request{Log: merged, Until: 120000, Idle: 5000, Window: &MeasurementWindow{Start: 0, End: 120000}})
	if e != nil {
		t.Fatal(e)
	}
	if r.Battles[0].Damage != 300 || r.Battles[0].Duration != 120 || r.Battles[0].Players[0].DPS != 2.5 {
		t.Fatal(r)
	}
}
