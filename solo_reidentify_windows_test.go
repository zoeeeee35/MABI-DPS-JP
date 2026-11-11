//go:build native && windows

package main

import "testing"

func TestNativeAutomaticCharacterChangeAndExplicitOff(t *testing.T) {
	oldSession, oldAuto, oldSelected, oldIndex, oldTotal, oldFloor, oldPlayer, oldError, oldRun := session, soloAutoEnabled, soloAutoSelected, historyIndex, displayCaptureTotal, soloIdentityFloor, selectedPlayer, errorText, soloMeasurement
	defer func() {
		session, soloAutoEnabled, soloAutoSelected, historyIndex, displayCaptureTotal, soloIdentityFloor, selectedPlayer, errorText, soloMeasurement = oldSession, oldAuto, oldSelected, oldIndex, oldTotal, oldFloor, oldPlayer, oldError, oldRun
	}()
	c := newDetectedConfig()
	c.Scope = "self"
	c.Entities["old"] = PartyEntity{ID: "old", Role: RoleSelf, Class: "氷結術師", ClassFamily: "IceMage", Evidence: "manual"}
	c.Entities["new"] = PartyEntity{ID: "new", Role: RoleOutside, Class: "クロスボウ使い", ClassFamily: "Arbalist"}
	l := CombatLog{Version: 1, Region: "jp", Source: "synthetic", Party: c, Players: []Player{{ID: "old", Name: "old", ClassID: "unknown"}, {ID: "new", Name: "new", ClassID: "unknown"}}, SkillUses: []SkillUse{{ActorID: "old", At: 0, Code: "IceMage_A"}, {ActorID: "new", At: 21000, Code: "Arbalist_A"}}}
	for i, at := range []int64{0, 21000, 22000, 23000} {
		id := "new"
		damage := int64(10)
		if i == 0 {
			id = "old"
			damage = 9000
		}
		l.Events = append(l.Events, Hit{ID: string(rune('a' + i)), At: at, Type: "damage", PlayerID: id, TargetID: "enemy", SkillID: "s", Damage: damage})
	}
	s, e := newSession(l, 5000)
	if e != nil {
		t.Fatal(e)
	}
	s.Cursor = 24000
	_ = s.recompute()
	session = s
	soloAutoEnabled = true
	soloAutoSelected = false
	historyIndex = -1
	soloIdentityFloor = 0
	soloMeasurement = nil
	displayCaptureTotal = true
	soloAutomaticSelection()
	if c.Entities["new"].Role != RoleSelf || c.StartAt != 21000 || !soloAutoSelected || displayCaptureTotal || s.current() == nil || s.current().Damage != 30 {
		t.Fatal(c.StartAt, c.Entities, s.Result)
	}
	_ = selectSelf(c, "old")
	soloAutoEnabled = false
	soloAutomaticSelection()
	if c.Entities["old"].Role != RoleSelf {
		t.Fatal("explicit OFF ignored")
	}
}
