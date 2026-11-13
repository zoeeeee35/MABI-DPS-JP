package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestTargetFilterSingleAndClearKeepsWindowAndSource(t *testing.T) {
	b := &Battle{Start: 1000, End: 11000, Duration: 10, Active: true, Damage: 600, Players: []Row{{Player: Player{ID: "self"}, Damage: 600, Hits: 3, DPS: 60}}, DamageEvents: []Hit{{PlayerID: "self", TargetID: "a", Damage: 100, SkillID: "s"}, {PlayerID: "self", TargetID: "b", Damage: 200, SkillID: "s"}, {PlayerID: "self", TargetID: "a", Damage: 300, SkillID: "t", Critical: true}}}
	data, _ := json.Marshal(b)
	one := filterBattleTarget(b, "a")
	if one.Damage != 400 || len(one.DamageEvents) != 2 || one.Duration != 10 || one.Start != 1000 || one.End != 11000 || one.Players[0].DPS != 40 || one.Players[0].Hits != 2 || one.Players[0].Crits != 1 {
		t.Fatal(one)
	}
	empty := filterBattleTarget(b, "absent")
	if empty.Damage != 0 || empty.Players[0].Hits != 0 || empty.Players[0].DPS != 0 {
		t.Fatal(empty)
	}
	if filterBattleTarget(b, "") != b || filterBattleTarget(nil, "a") != nil {
		t.Fatal("clear")
	}
	after, _ := json.Marshal(b)
	if !reflect.DeepEqual(data, after) {
		t.Fatal("raw battle changed")
	}
	var sum int64
	for _, target := range damageTargets(b, CombatLog{}) {
		sum += filterBattleTarget(b, target.ID).Damage
	}
	if sum != b.Damage {
		t.Fatal(sum)
	}
}
func TestTargetFilteredUsageOnlyKnownTarget(t *testing.T) {
	log := CombatLog{Detected: newDetectedConfig(), SkillUses: []SkillUse{{HasTail: true, TargetCandidate: "000ABC", Code: "S"}, {HasTail: true, TargetCandidate: "DEF", Code: "Other"}, {Code: "Unknown"}}}
	log.Detected.Entities["a"] = PartyEntity{ID: "a", DisplayID: "000ABC"}
	scoped := targetSkillUsageLog(log, "a")
	if len(scoped.SkillUses) != 1 || scoped.SkillUses[0].Code != "S" || len(log.SkillUses) != 3 {
		t.Fatal(scoped)
	}
}
func TestLocalTrainingDummyTargets(t *testing.T) {
	path := os.Getenv("MABI_TARGET_PCAP")
	if path == "" {
		t.Skip("local")
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	stats, e := replayPCAP(f, 16000)
	if e != nil {
		t.Fatal(e)
	}
	if stats.provisionalLog == nil {
		t.Fatal("no log")
	}
	log := *stats.provisionalLog
	type actorReport struct {
		Actor   string
		Targets []DamageTarget
		Hits    int
		Damage  int64
	}
	result := []actorReport{}
	for _, p := range log.Players {
		b := &Battle{Start: 0, End: 100000, Duration: 100, Players: []Row{{Player: p}}, DamageEvents: []Hit{}}
		for _, h := range log.Events {
			if h.PlayerID == p.ID {
				b.DamageEvents = append(b.DamageEvents, h)
				b.Damage += h.Damage
			}
		}
		if len(b.DamageEvents) == 0 {
			continue
		}
		targets := damageTargets(b, log)
		var sum int64
		hits := 0
		for _, target := range targets {
			view := filterBattleTarget(b, target.ID)
			sum += view.Damage
			hits += len(view.DamageEvents)
			for _, h := range view.DamageEvents {
				if h.TargetID != target.ID {
					t.Fatal("target leak")
				}
			}
		}
		if sum != b.Damage || hits != len(b.DamageEvents) {
			t.Fatal("lost/double counted hits")
		}
		result = append(result, actorReport{Actor: damageTargetLabel(log, p.ID), Targets: targets, Hits: hits, Damage: sum})
		t.Logf("actor %s targets=%d hits=%d damage=%d", damageTargetLabel(log, p.ID), len(targets), hits, sum)
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	if out := os.Getenv("MABI_TARGET_REPORT"); out != "" {
		if e = os.WriteFile(out, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if len(result) == 0 {
		t.Fatal(fmt.Errorf("no target data"))
	}
}
