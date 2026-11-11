package main

import "testing"

func TestStaleCharacterUsesFreshActivityNotOldTotal(t *testing.T) {
	l := CombatLog{Events: []Hit{{PlayerID: "old", At: 0, Damage: 900000}, {PlayerID: "new", At: 21000, Damage: 10}, {PlayerID: "new", At: 22000, Damage: 10}, {PlayerID: "new", At: 23000, Damage: 10}}, SkillUses: []SkillUse{{ActorID: "old", At: 0}, {ActorID: "new", At: 21000}}}
	id, hits := recentSoloCandidate(l, 24000, 0)
	if id != "new" || hits != 3 || !staleSelfCanSwitch(l, "old", id, 24000, hits) {
		t.Fatal(id, hits)
	}
	l.SkillUses = append(l.SkillUses, SkillUse{ActorID: "old", At: 23000})
	if staleSelfCanSwitch(l, "old", id, 24000, hits) {
		t.Fatal("active self must not be replaced")
	}
	if staleSelfCanSwitch(l, "old", "new", 24000, 2) {
		t.Fatal("weak evidence")
	}
	if id, _ := recentSoloCandidate(l, 24000, 24000); id != "" {
		t.Fatal("old data crossed reset boundary")
	}
}
func TestScopeResetKeepsOriginalCapture(t *testing.T) {
	c := newDetectedConfig()
	c.Scope = "self"
	c.StartAt = 20000
	c.Entities["p"] = PartyEntity{ID: "p", Role: RoleSelf}
	l := CombatLog{Players: []Player{{ID: "p"}}, Events: []Hit{{PlayerID: "p", TargetID: "e", At: 10000, Damage: 100}, {PlayerID: "p", TargetID: "e", At: 22000, Damage: 200}}}
	out := c.filtered(l)
	if len(out.Events) != 1 || out.Events[0].Damage != 200 || len(l.Events) != 2 {
		t.Fatal(out)
	}
}
