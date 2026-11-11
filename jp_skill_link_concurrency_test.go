package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestSkillLinkSnapshotsRetainLatePairAndNames(t *testing.T) {
	d := jp9Decoder{}
	now := time.Now()
	hit, pair := linkHit(1, 2, 123)
	got := d.feed(hit, false, now)
	published := got[0].Link.snapshot()
	log := CombatLog{Events: []Hit{{Damage: 949, Link: published}}}
	applySkillLinks(&log)
	if log.Events[0].Link.Paired {
		t.Fatal("paired too early")
	}
	d.feed(pair, false, now)
	d.feed(linkSkill(1, 2, 123, "IceMage_A"), false, now)
	applySkillLinks(&log)
	previous := log.Events[0].Link
	if !previous.Paired || len(previous.Names) != 1 {
		t.Fatal(previous)
	}
	d.feed(linkSkill(1, 2, 123, "IceMage_B"), false, now)
	applySkillLinks(&log)
	if len(log.Events[0].Link.Names) != 2 || len(previous.Names) != 1 || published.Paired || log.Events[0].Damage != 949 {
		t.Fatal("snapshot mutated or late linkage lost")
	}
}

func TestConcurrentCaptureSkillRegistryAndUIRecompute(t *testing.T) {
	const n = 1500
	links := make(chan *JPSkillLink, 32)
	go func() {
		defer close(links)
		d := jp9Decoder{}
		now := time.Now()
		hit, pair := linkHit(1, 2, 123)
		for i := 0; i < n; i++ {
			out := d.feed(hit, false, now)
			links <- out[0].Link.snapshot() // publish before pairing, just like split TCP frames
			d.feed(pair, false, now)
			d.feed(linkSkill(1, 2, 123, fmt.Sprintf("IceMage_Concurrent_%02d", i%32)), false, now)
		}
	}()
	log := CombatLog{}
	for link := range links {
		log.Events = append(log.Events, Hit{Damage: 949, Link: link})
		if len(log.Events)%10 == 0 {
			applySkillLinks(&log)
			if _, err := json.Marshal(log); err != nil {
				t.Fatal(err)
			}
		}
	}
	applySkillLinks(&log)
	if len(log.Events) != n {
		t.Fatal(len(log.Events))
	}
	var sum int64
	for _, h := range log.Events {
		sum += h.Damage
		if !h.Link.Paired || len(h.Link.Names) != 32 {
			t.Fatal("lost late pairing/registry entries", h.Link)
		}
	}
	if sum != n*949 {
		t.Fatal("damage changed", sum)
	}
}
