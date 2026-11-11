package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func nativeFixture(t *testing.T) CombatLog {
	t.Helper()
	data, e := assets.ReadFile("native_sample.json")
	if e != nil {
		t.Fatal(e)
	}
	var l CombatLog
	if e = decode(bytes.NewReader(data), &l); e != nil {
		t.Fatal(e)
	}
	return l
}
func TestNativeReplayRankingAndBoundaries(t *testing.T) {
	s, e := newSession(nativeFixture(t), 5000)
	if e != nil {
		t.Fatal(e)
	}
	s.Playing = true
	for _, c := range []struct {
		at     int64
		top    string
		total  int64
		active bool
		number int
	}{{0, "sample:a", 1000, true, 1}, {4000, "sample:b", 8800, true, 1}, {6000, "sample:a", 15300, true, 1}, {8000, "sample:b", 17300, true, 1}, {10000, "sample:a", 22300, true, 1}, {12000, "sample:a", 22300, false, 1}, {20000, "sample:c", 3000, true, 2}, {24000, "sample:b", 9500, true, 2}, {29000, "sample:b", 9500, false, 2}} {
		if e = s.advance(c.at - s.Cursor); e != nil {
			t.Fatal(e)
		}
		b := s.current()
		if b == nil || b.Players[0].ID != c.top || b.Damage != c.total || b.Active != c.active || b.Number != c.number {
			t.Fatalf("at %d got %#v", c.at, b)
		}
	}
	if s.Playing {
		t.Fatal("playback did not stop")
	}
	if len(s.Result.Battles) != 2 {
		t.Fatal(s.Result)
	}
}
func TestPauseSpeedAndResetSemantics(t *testing.T) {
	s, _ := newSession(nativeFixture(t), 5000)
	_ = s.advance(4000)
	if s.Cursor != 0 {
		t.Fatal("advanced while paused")
	}
	s.Speed = 4
	s.Playing = true
	_ = s.advance(1000)
	if s.Cursor != 4000 || s.current().Players[0].ID != "sample:b" {
		t.Fatal(s)
	}
	if e := s.advance(-1); e == nil {
		t.Fatal("accepted backwards clock")
	}
}
func TestArchiveRoundTripAndDedup(t *testing.T) {
	dir := t.TempDir()
	s, _ := newSession(nativeFixture(t), 5000)
	s.Playing = true
	_ = s.advance(29000)
	q := Request{Log: s.Log, Until: s.Cursor, Idle: s.Idle}
	for _, b := range s.Result.Battles {
		path, e := saveBattle(dir, q, b)
		if e != nil {
			t.Fatal(e)
		}
		path2, e := saveBattle(dir, q, b)
		if e != nil || path2 != path {
			t.Fatal("not deduplicated")
		}
		entry, e := loadArchive(path)
		if e != nil || entry.Battle.Damage != b.Damage || entry.Archive.LiveVerified {
			t.Fatal(entry, e)
		}
	}
	records, bad := loadHistory(dir)
	if len(records) != 2 || bad != 0 {
		t.Fatal(len(records), bad)
	}
	_ = os.WriteFile(filepath.Join(dir, "bad.json"), []byte(`{"password":"should never load"}`), 0600)
	records, bad = loadHistory(dir)
	if len(records) != 2 || bad != 1 {
		t.Fatal(len(records), bad)
	}
}
func TestArchiveRejectsForgedVerification(t *testing.T) {
	s, _ := newSession(nativeFixture(t), 5000)
	s.Playing = true
	_ = s.advance(29000)
	a := Archive{Version: 1, CreatedAt: "2026-10-25T00:00:00Z", Request: Request{Log: s.Log, Until: s.Cursor, Idle: s.Idle}, BattleNumber: 1, LiveVerified: true}
	data, _ := json.Marshal(a)
	path := filepath.Join(t.TempDir(), "fake.json")
	_ = os.WriteFile(path, data, 0600)
	if _, e := loadArchive(path); e == nil {
		t.Fatal("accepted a forged live verified claim")
	}
}
func TestActiveBattleCannotAutoSave(t *testing.T) {
	s, _ := newSession(nativeFixture(t), 5000)
	if _, e := saveBattle(t.TempDir(), Request{Log: s.Log, Until: 0, Idle: 5000}, *s.current()); e == nil {
		t.Fatal("saved unfinished battle")
	}
}
func TestNormalizedImportAndSensitiveFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "日本語.json")
	data, _ := assets.ReadFile("native_sample.json")
	_ = os.WriteFile(path, append([]byte{0xef, 0xbb, 0xbf}, data...), 0600)
	l, e := readLog(path)
	if e != nil || l.Players[0].Name != "アオイ（サンプル）" {
		t.Fatal(l, e)
	}
	_ = os.WriteFile(path, []byte(`{"version":1,"password":"secret"}`), 0600)
	if _, e = readLog(path); e == nil {
		t.Fatal("accepted noncombat data")
	}
}
