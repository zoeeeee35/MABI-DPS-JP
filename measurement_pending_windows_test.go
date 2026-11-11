//go:build native && windows

package main

import (
	"testing"
	"time"
)

func TestTimedMeasurementLateIdentityBackfillsOwnDamage(t *testing.T) {
	oldSession, oldRun, oldAuto, oldSelected, oldIndex, oldFloor, oldPlayer := session, soloMeasurement, soloAutoEnabled, soloAutoSelected, historyIndex, soloIdentityFloor, selectedPlayer
	defer func() {
		session, soloMeasurement, soloAutoEnabled, soloAutoSelected, historyIndex, soloIdentityFloor, selectedPlayer = oldSession, oldRun, oldAuto, oldSelected, oldIndex, oldFloor, oldPlayer
	}()
	c := newDetectedConfig()
	c.Scope = "self"
	c.Entities["p"] = PartyEntity{ID: "p", Role: RoleOutside, Class: "氷結術師", ClassFamily: "IceMage"}
	c.Entities["other"] = PartyEntity{ID: "other", Role: RoleEnemy}
	log := CombatLog{Version: 1, Region: "jp", Source: "synthetic", Party: c, Players: []Player{{ID: "p", Name: "p", ClassID: "unknown"}, {ID: "other", Name: "other", ClassID: "unknown"}}, SkillUses: []SkillUse{{ActorID: "p", At: 5000, Code: "IceMage_Test"}}, Events: []Hit{
		{ID: "pre", At: 4999, Type: "damage", PlayerID: "p", SkillID: "s", TargetID: "enemy", Damage: 9999},
		{ID: "first", At: 5000, Type: "damage", PlayerID: "p", SkillID: "s", TargetID: "enemy", Damage: 600},
		{ID: "second", At: 6000, Type: "damage", PlayerID: "p", SkillID: "s", TargetID: "enemy", Damage: 1200},
		{ID: "other", At: 6000, Type: "damage", PlayerID: "other", SkillID: "s", TargetID: "p", Damage: 8888},
	}}
	s, err := newSession(log, 5000)
	if err != nil {
		t.Fatal(err)
	}
	s.Cursor = 7000
	session = s
	historyIndex = -1
	soloAutoEnabled = true
	soloAutoSelected = false
	soloIdentityFloor = 0
	run := &soloTimedRun{Start: 5000, End: 65000}
	soloMeasurement = run
	if bindSoloTimedIdentity(run) {
		t.Fatal("identity guessed before detector")
	}
	soloAutomaticSelection()
	if soloMeasurement != run || !bindSoloTimedIdentity(run) {
		t.Fatal("pending run blocked auto identification")
	}
	merged := mergeTimedLogs(CombatLog{}, log, run.Party, run.Start, run.End)
	r, err := analyze(Request{Log: merged, Until: 65000, Idle: 5000, Window: &MeasurementWindow{Start: run.Start, End: run.End}})
	if err != nil {
		t.Fatal(err)
	}
	b := r.Battles[0]
	if b.Damage != 1800 || b.Duration != 60 || len(b.Players) != 1 || b.Players[0].ID != "p" {
		t.Fatal(b)
	}
	if run.Start != 5000 || run.End != 65000 {
		t.Fatal("window shifted")
	}
}

func TestTimedMeasurementStartsBeforeFirstDamageSession(t *testing.T) {
	oldActive, oldInput, oldJP, oldSession, oldRun, oldEnd, oldMinutes, oldModern, oldHistory, oldError, oldNotice := captureActive, captureInput, jpInput, session, soloMeasurement, soloTimerEnd, soloTimerMinutes, nativeModern, historyIndex, errorText, soloMeasurementNotice
	defer func() {
		captureActive, captureInput, jpInput, session, soloMeasurement, soloTimerEnd, soloTimerMinutes, nativeModern, historyIndex, errorText, soloMeasurementNotice = oldActive, oldInput, oldJP, oldSession, oldRun, oldEnd, oldMinutes, oldModern, oldHistory, oldError, oldNotice
	}()
	live, err := newLiveInput("jp", time.Now(), 5000)
	if err != nil {
		t.Fatal(err)
	}
	if live.Session != nil {
		t.Fatal("test must start without damage session")
	}
	captureActive = true
	captureInput = live
	jpInput = &JPProvisionalInput{Live: live}
	session = nil
	soloMeasurement = nil
	soloTimerMinutes = 1
	nativeModern = false
	historyIndex = -1
	beginSoloMeasurement()
	run := soloMeasurement
	if run == nil || run.Party != nil || soloTimerEnd.IsZero() {
		t.Fatal("active capture rejected before first damage")
	}
	if n := soloPreparationRemaining(time.Now()); n < 4 || n > 5 {
		t.Fatal("no preparation", n)
	}
	start := run.Start
	updateSoloMeasurement(live.origin.Add(time.Duration(start+1000) * time.Millisecond))
	if soloMeasurement != run || run.Start != start {
		t.Fatal("session wait cancelled or shifted window")
	}
	updateSoloMeasurement(live.origin.Add(time.Duration(run.End) * time.Millisecond))
	if soloMeasurement != nil || !soloTimerEnd.IsZero() {
		t.Fatal("empty run did not finish")
	}
}
