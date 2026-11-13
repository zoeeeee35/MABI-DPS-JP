package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCaptureManySkillOnlyActorsDoNotConsumeDamageSlots(t *testing.T) {
	now := time.Unix(100, 0)
	v, _ := newJPProvisionalInput(now, 5000)
	for i := 0; i < 200; i++ {
		if _, e := v.append(JPValue{At: now, ActorToken: fmt.Sprint(i), SkillCode: "IceMage_CrystalEdge_01", ClassCode: "IceMage"}); e != nil {
			t.Fatal(e)
		}
	}
	if len(v.Live.actors) != 0 {
		t.Fatal("skill-only actors consumed damage slots")
	}
	if _, e := v.append(JPValue{At: now, ActorToken: "self", TargetToken: "enemy", EventToken: "first", Value: 100}); e != nil {
		t.Fatal(e)
	}
}
func TestCapture65DamageActorsArchivesAndContinues(t *testing.T) {
	now := time.Unix(100, 0)
	v, _ := newLiveInput("jp", now, 5000)
	archived := int64(0)
	v.OnRotate = func(log CombatLog, end int64, reason string) error {
		if reason != "actor-capacity" || len(log.Players) != 64 {
			t.Fatal(reason, len(log.Players))
		}
		for _, h := range log.Events {
			archived += h.Damage
		}
		return nil
	}
	for i := 0; i < 130; i++ {
		if e := v.append(RawDamage{Attacker: fmt.Sprint(i), Target: "enemy", Skill: "skill", Damage: 100, At: now.Add(time.Duration(i) * time.Second)}); e != nil {
			t.Fatal(e)
		}
	}
	for _, h := range v.Session.Log.Events {
		archived += h.Damage
	}
	if archived != 13000 || v.Rotations != 2 {
		t.Fatal(archived, v.Rotations)
	}
}
func TestCapture20001EventsArchivesAndKeepsDPS(t *testing.T) {
	now := time.Unix(100, 0)
	v, _ := newLiveInput("jp", now, 5000)
	total := int64(0)
	v.OnRotate = func(log CombatLog, end int64, reason string) error {
		if reason != "event-capacity" {
			t.Fatal(reason)
		}
		for i := range log.Players {
			log.Players[i].Party = true
		}
		r, e := analyze(Request{Log: log, Until: end, Idle: 5000})
		if e != nil {
			return e
		}
		for _, b := range r.Battles {
			if b.Active {
				t.Fatal("unfinished archive")
			}
			total += b.Damage
		}
		return nil
	}
	ids := map[string]bool{}
	for i := 0; i < 20005; i++ {
		if e := v.append(RawDamage{Attacker: "self", Target: "enemy", Skill: "skill", Damage: 3, At: now.Add(time.Duration(i) * 2 * time.Millisecond)}); e != nil {
			t.Fatal(e)
		}
		h := v.Session.Log.Events[len(v.Session.Log.Events)-1]
		if ids[h.ID] {
			t.Fatal("ID reused")
		}
		ids[h.ID] = true
	}
	for i := range v.Session.Log.Players {
		v.Session.Log.Players[i].Party = true
	}
	if e := v.tick(now.Add(time.Minute)); e != nil {
		t.Fatal(e)
	}
	for _, b := range v.Session.Result.Battles {
		total += b.Damage
	}
	if total != 60015 || v.Rotations != 1 {
		t.Fatal(total, v.Rotations)
	}
}
func TestCaptureFailedRotationRetainsEvents(t *testing.T) {
	v, _ := newLiveInput("jp", time.Unix(100, 0), 5000)
	v.Session = &Session{Log: CombatLog{Events: make([]Hit, 20000)}}
	v.OnRotate = func(CombatLog, int64, string) error { return errors.New("disk full") }
	e := v.append(RawDamage{Attacker: "self", Target: "enemy", Skill: "skill", Damage: 3})
	if e == nil || !strings.Contains(e.Error(), "disk full") || len(v.Session.Log.Events) != 20000 {
		t.Fatal(e)
	}
}
func TestCaptureSeenIDsBoundedAndRecentDuplicatesRejected(t *testing.T) {
	v, _ := newJPProvisionalInput(time.Unix(100, 0), 5000)
	for i := 0; i < 30000; i++ {
		v.rememberEvent(fmt.Sprint(i))
	}
	if len(v.Seen) > 4096 || !v.Seen["29999"] {
		t.Fatal(len(v.Seen))
	}
	if ok, e := v.append(JPValue{At: time.Unix(101, 0), ActorToken: "a", TargetToken: "b", EventToken: "29999", Value: 100}); e != nil || ok {
		t.Fatal(ok, e)
	}
}

func TestCaptureFifteenMinutesDoesNotHitTimeLimit(t *testing.T) {
	now := time.Unix(100, 0)
	v, _ := newLiveInput("jp", now, 5000)
	for i := 0; i <= 900; i++ {
		if e := v.append(RawDamage{Attacker: "self", Target: "enemy", Skill: "s", At: now.Add(time.Duration(i) * time.Second), Damage: 10}); e != nil {
			t.Fatal(i, e)
		}
	}
	if v.Rotations != 0 || len(v.Session.Log.Events) != 901 {
		t.Fatal(v.Rotations)
	}
}
func TestRotationSavesReloadableArchives(t *testing.T) {
	now := time.Unix(100, 0)
	v, _ := newLiveInput("jp", now, 5000)
	dir := t.TempDir()
	saved := []string{}
	v.OnRotate = func(log CombatLog, end int64, reason string) error {
		for i := range log.Players {
			log.Players[i].Party = true
		}
		q := Request{Log: log, Until: end, Idle: 5000}
		r, e := analyze(q)
		if e != nil {
			return e
		}
		for _, b := range r.Battles {
			p, e := saveBattleSegment(dir, q, b, reason)
			if e != nil {
				return e
			}
			saved = append(saved, p)
		}
		return nil
	}
	for i := 0; i < 65; i++ {
		if e := v.append(RawDamage{Attacker: fmt.Sprint(i), Target: "enemy", Skill: "s", At: now.Add(time.Duration(i) * time.Millisecond), Damage: 100}); e != nil {
			t.Fatal(e)
		}
	}
	if len(saved) != 1 {
		t.Fatal(saved)
	}
	arch, e := loadArchive(saved[0])
	if e != nil {
		t.Fatal(e)
	}
	if arch.Archive.SegmentReason != "actor-capacity" || arch.Battle.Damage != 6400 || v.Session.Log.Events[0].Damage != 100 {
		t.Fatal(arch.Battle)
	}
}
