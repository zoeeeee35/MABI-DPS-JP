package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestHistoryTitleUsesDateClassDurationNotCounter(t *testing.T) {
	c := newDetectedConfig()
	c.Entities["p"] = PartyEntity{ID: "p", Role: RoleSelf, ClassFamily: "IceMage"}
	h := SavedBattle{Archive: Archive{CreatedAt: "2026-10-25T05:30:00Z", Request: Request{Log: CombatLog{Party: c}, Window: &MeasurementWindow{Start: 0, End: 60000}}}, Battle: Battle{Number: 99, Duration: 60, Damage: 600}}
	title := historyDisplayTitle(h)
	if !strings.Contains(title, "2026/10/25 14:30:00") || !strings.Contains(title, "氷結術師") || !strings.Contains(title, "60.0秒") || strings.Contains(title, "戦闘99") {
		t.Fatal(title)
	}
	if !strings.Contains(historyDisplayDetail(h), "場所未取得") {
		t.Fatal(h)
	}
}
func TestHistoryLocationRoundTripDoesNotChangeCombat(t *testing.T) {
	q := Request{Log: CombatLog{Version: 1, Region: "jp", Source: "synthetic", Players: []Player{{ID: "p", Name: "自分", ClassID: "unknown", Party: true}}, Events: []Hit{{ID: "d", At: 0, Type: "damage", PlayerID: "p", TargetID: "e", SkillID: "MeleeDefaultAttack_1", Damage: 100}}}, Until: 60000, Idle: 5000, Window: &MeasurementWindow{Start: 0, End: 60000}}
	r, e := analyze(q)
	if e != nil {
		t.Fatal(e)
	}
	path, e := saveBattle(t.TempDir(), q, r.Battles[0])
	if e != nil {
		t.Fatal(e)
	}
	before, e := loadArchive(path)
	if e != nil {
		t.Fatal(e)
	}
	if e = updateHistoryLocation(path, "カカシ・訓練場"); e != nil {
		t.Fatal(e)
	}
	after, e := loadArchive(path)
	if e != nil {
		t.Fatal(e)
	}
	if after.Archive.LocationNote != "カカシ・訓練場" || after.Archive.CreatedAt != before.Archive.CreatedAt || !reflect.DeepEqual(before.Battle, after.Battle) || !reflect.DeepEqual(before.Archive.Request, after.Archive.Request) {
		t.Fatal(after)
	}
	if e = updateHistoryLocation(path, strings.Repeat("あ", 81)); e == nil {
		t.Fatal("overlong note accepted")
	}
	hist, bad := loadHistory(filepath.Dir(path))
	if bad != 0 || len(hist) != 1 || hist[0].Archive.LocationNote != "カカシ・訓練場" {
		t.Fatal(hist, bad)
	}
}

func TestHistoryUsesRecordedTimestampWhenAvailable(t *testing.T) {
	stamp, _ := time.Parse(time.RFC3339, "2026-10-25T05:30:01Z")
	h := SavedBattle{Archive: Archive{CreatedAt: "2026-10-25T08:00:00Z"}, Battle: Battle{Start: 1000, DamageEvents: []Hit{{At: 2000, Flags: &DamageFlags{Timestamp: stamp}}}}}
	got, captured := historyRecordTime(h)
	if !captured || got.Format(time.RFC3339) != "2026-10-25T05:30:00Z" || !strings.Contains(historyDisplayTitle(h), "14:30:00") {
		t.Fatal(got, captured)
	}
}
