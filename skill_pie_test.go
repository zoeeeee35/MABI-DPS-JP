package main

import (
	"math"
	"testing"
)

func TestSkillPieTotalsExcludeAllAndChildren(t *testing.T) {
	rows := []SkillDamageRow{{Key: "all", Damage: 1000}, {Key: "unknown", Name: "未分類", Damage: 300}, {Key: "unknown-child:1", Damage: 300}, {Key: "skill:A", Name: "Very_Long_Internal_Skill_Name", Damage: 700}, {Key: "skill:unused", Damage: 0}}
	slices := skillPieData(rows)
	if len(slices) != 2 || slices[0].Name != "Very_Long_Internal_Skill_Name" || slices[0].Share != 0.7 || slices[1].Share != 0.3 || slices[1].End != 1 {
		t.Fatal(slices)
	}
	if skillPieHit(slices, 1000, 1000, 0, 0, 10) != -1 || len(skillPieData(nil)) != 0 {
		t.Fatal("empty/outside")
	}
	for i, s := range slices {
		a := (s.Start+s.End)*math.Pi - math.Pi/2
		x, y := math.Cos(a)*5, math.Sin(a)*5
		if hit := skillPieHit(slices, x, y, 0, 0, 10); hit != i {
			t.Fatal(i, hit)
		}
	}
}
