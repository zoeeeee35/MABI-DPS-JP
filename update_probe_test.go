package main

import (
	"encoding/binary"
	"os"
	"testing"
	"time"
)

func TestUpdatedJPReceiveMessages(t *testing.T) {
	now := time.Now()
	d := jp9Decoder{}
	damage, effect := linkHit(1, 2, 123)
	skill := linkSkill(1, 2, 123, "IceMage_One")
	binary.LittleEndian.PutUint32(damage, 0x52cc)
	binary.LittleEndian.PutUint32(effect, 0x502e)
	binary.LittleEndian.PutUint32(skill, 0x29850)
	d.feed(skill, false, now)
	hits := d.feed(damage, false, now)
	if len(hits) != 1 || hits[0].Value != 949 {
		t.Fatal("updated damage not decoded", hits)
	}
	if extra := d.feed(effect, false, now); len(extra) != 0 {
		t.Fatal("effect incorrectly counted as damage")
	}
	link := hits[0].Link.snapshot()
	if !link.Paired || len(link.Names) != 1 || link.Names[0] != "IceMage_One" {
		t.Fatal("updated skill association failed", link)
	}
}

func TestLocalNovemberUpdateReplay(t *testing.T) {
	path := os.Getenv("UPDATE_CAP")
	if path == "" {
		t.Skip("local game capture")
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	stats, err := replayPCAP(file, 16000)
	if err != nil {
		t.Fatal(err)
	}
	if stats.JP.Count != 1 || stats.JP.Sum != 11059 || stats.JP.Paired != 1 || stats.JP.SkillFrames != 5 || stats.JP.HPUpdates != 10 {
		t.Fatal("unexpected update replay counts", stats.JP.Count, stats.JP.Sum, stats.JP.Paired, stats.JP.SkillFrames, stats.JP.HPUpdates)
	}
	if stats.provisionalLog == nil || len(stats.provisionalLog.Events) != 1 || stats.provisionalLog.Events[0].Damage != 11059 {
		t.Fatal("damage did not reach the provisional combat log")
	}
}
