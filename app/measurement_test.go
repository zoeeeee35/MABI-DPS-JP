package main

import (
	"fmt"
	"testing"
)

func TestTimedMeasurementExactWindowAndArchive(t *testing.T) {
	for _, seconds := range []int64{60, 120, 180} {
		t.Run(fmt.Sprint(seconds), func(t *testing.T) {
			start := int64(10000)
			end := start + seconds*1000
			l := CombatLog{Version: 1, Region: "jp", Source: "synthetic", Players: []Player{{ID: "p", Name: "自分", ClassID: "unknown", Party: true}}, Events: []Hit{{ID: "before", At: start - 1, Type: "damage", PlayerID: "p", SkillID: "s", TargetID: "enemy", Damage: 9000}, {ID: "first", At: start, Type: "damage", PlayerID: "p", SkillID: "s", TargetID: "enemy", Damage: 600}, {ID: "middle", At: start + 10000, Type: "damage", PlayerID: "p", SkillID: "s", TargetID: "enemy", Damage: 1200}, {ID: "last", At: end - 1, Type: "damage", PlayerID: "p", SkillID: "s", TargetID: "enemy", Damage: 1800}, {ID: "after", At: end, Type: "damage", PlayerID: "p", SkillID: "s", TargetID: "enemy", Damage: 9000}}}
			q := Request{Log: l, Until: end, Idle: 5000, Window: &MeasurementWindow{Start: start, End: end}}
			r, e := analyze(q)
			if e != nil {
				t.Fatal(e)
			}
			b := r.Battles[0]
			if len(r.Battles) != 1 || b.Active || b.Duration != float64(seconds) || b.Damage != 3600 || b.Players[0].DPS != 3600/float64(seconds) || b.Players[0].Hits != 3 {
				t.Fatal(b)
			}
			if len(q.Log.Events) != 5 {
				t.Fatal("live log mutated")
			}
			path, e := saveBattle(t.TempDir(), q, b)
			if e != nil {
				t.Fatal(e)
			}
			saved, e := loadArchive(path)
			if e != nil {
				t.Fatal(e)
			}
			if saved.Battle.Duration != float64(seconds) || saved.Battle.Damage != 3600 || saved.Archive.Request.Window == nil {
				t.Fatal(saved.Battle)
			}
		})
	}
}
func TestTimedMeasurementZeroAndInvalid(t *testing.T) {
	q := Request{Log: CombatLog{Version: 1, Region: "jp", Source: "synthetic", Players: []Player{{ID: "p", Name: "自分", ClassID: "unknown", Party: true}}}, Until: 60000, Idle: 5000, Window: &MeasurementWindow{Start: 0, End: 60000}}
	r, e := analyze(q)
	if e != nil || len(r.Battles) != 1 || r.Battles[0].Damage != 0 || r.Battles[0].Duration != 60 {
		t.Fatal(r, e)
	}
	path, e := saveBattle(t.TempDir(), q, r.Battles[0])
	if e != nil {
		t.Fatal(e)
	}
	if _, e = loadArchive(path); e != nil {
		t.Fatal(e)
	}
	q.Window.End = 0
	if _, e = analyze(q); e == nil {
		t.Fatal("invalid window accepted")
	}
}

func TestPreparationExcludedFromEachTimedDuration(t *testing.T) {
	for _, minutes := range []int{1, 2, 3} {
		start := int64(soloPreparation.Milliseconds())
		end := start + soloTimerDuration(minutes).Milliseconds()
		q := Request{Log: CombatLog{Version: 1, Region: "jp", Source: "synthetic", Players: []Player{{ID: "p", Name: "自分", Party: true}}, Events: []Hit{
			{ID: "prep", At: start - 1, Type: "damage", PlayerID: "p", SkillID: "s", TargetID: "e", Damage: 9999},
			{ID: "start", At: start, Type: "damage", PlayerID: "p", SkillID: "s", TargetID: "e", Damage: 600},
			{ID: "end", At: end, Type: "damage", PlayerID: "p", SkillID: "s", TargetID: "e", Damage: 9999},
		}}, Until: end, Idle: 5000, Window: &MeasurementWindow{Start: start, End: end}}
		r, err := analyze(q)
		if err != nil {
			t.Fatal(err)
		}
		b := r.Battles[0]
		if b.Damage != 600 || b.Duration != float64(minutes*60) {
			t.Fatal(minutes, b)
		}
	}
}
