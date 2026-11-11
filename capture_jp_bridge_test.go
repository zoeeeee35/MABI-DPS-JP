package main

import (
	"github.com/gopacket/gopacket/layers"
	"math"
	"path/filepath"
	"testing"
	"time"
)

func provisionalValue(at time.Time, id string, value uint32) JPValue {
	return JPValue{At: at, ActorToken: "a", TargetToken: "b", EventToken: id, Value: value}
}
func TestJPProvisionalDPSMultiHitDuplicateIdleAndArchive(t *testing.T) {
	now := time.Now()
	v, e := newJPProvisionalInput(now, 5000)
	if e != nil {
		t.Fatal(e)
	}
	for _, x := range []JPValue{provisionalValue(now.Add(10*time.Second), "1", 1000), provisionalValue(now.Add(12*time.Second), "2", 1000), provisionalValue(now.Add(12*time.Second), "2", 1000)} {
		if _, e = v.append(x); e != nil {
			t.Fatal(e)
		}
	}
	if e = v.tick(now.Add(13 * time.Second)); e != nil {
		t.Fatal(e)
	}
	b := v.Live.Session.current()
	if b.Start != 10000 || b.Damage != 2000 || b.Players[0].Hits != 2 || !b.Active || b.Duration != 2 || math.Abs(b.Players[0].DPS-1000) > 0.01 || v.Live.Session.Log.Source != jpProvisionalSource || v.Live.Session.Result.LiveVerified {
		t.Fatal(b)
	}
	_ = v.tick(now.Add(18 * time.Second))
	b = v.Live.Session.current()
	if b.Active || b.Duration != 2 || b.Players[0].DPS != 1000 {
		t.Fatal(b)
	}
	q := Request{Log: v.Live.Session.Log, Until: v.Live.Session.Cursor, Idle: 5000}
	dir := t.TempDir()
	path, e := saveBattle(dir, q, *b)
	if e != nil {
		t.Fatal(e)
	}
	loaded, e := loadArchive(filepath.Clean(path))
	if e != nil || loaded.Battle.Damage != 2000 || loaded.Archive.LiveVerified || loaded.Archive.Request.Log.Source != jpProvisionalSource {
		t.Fatal(e, loaded)
	}
}
func TestJPProvisionalRolesAndUnknownNotFormal(t *testing.T) {
	now := time.Now()
	v, _ := newJPProvisionalInput(now, 5000)
	v.Roles["self"] = JPPlayer
	v.Roles["ally"] = JPPlayer
	v.Roles["enemy"] = JPEnemy
	tests := []struct {
		a, b   string
		accept bool
	}{{"self", "enemy", true}, {"enemy", "self", false}, {"self", "ally", false}, {"self", "self", false}, {"unknown", "target", true}, {"", "target", false}}
	for i, tc := range tests {
		e := JPValue{At: now, ActorToken: tc.a, TargetToken: tc.b, EventToken: string(rune('a' + i)), Value: 100}
		accepted, err := v.append(e)
		if err != nil || accepted != tc.accept {
			t.Fatal(tc, accepted, err)
		}
	}
	_ = v.tick(now.Add(time.Second))
	if v.Live.Session.current().Damage != 200 || v.Live.Session.Log.Source != jpProvisionalSource || v.Live.Session.Result.LiveVerified {
		t.Fatal(v.Live.Session.Result)
	}
}
func TestJP5243PairRetransmissionToProvisionalEngine(t *testing.T) {
	p := pipelineFixture()
	p.Stats.Profile = "jp-kr-legacy-probe"
	now := time.Now()
	v, _ := newJPProvisionalInput(now, 5000)
	pair := jpFrame(0x4ff7, jpHit(1, 2, 730)[9:25], false)
	payload := append(jpHit(1, 2, 730), pair...)
	payload = append(payload, jpHit(1, 2, 730)...)
	payload = append(payload, pair...)
	for _, seq := range []uint32{100, 100} {
		p.feed(ethernetTCP(t, seq, payload, 40000), layers.LinkTypeEthernet, now)
		for _, e := range p.jpQueue {
			if _, err := v.append(e); err != nil {
				t.Fatal(err)
			}
		}
		p.jpQueue = nil
	}
	_ = v.tick(now.Add(time.Second))
	if p.Stats.JP.Paired != 2 || p.Stats.JP.Count != 2 || v.Live.Session.current().Damage != 1460 || v.Live.Session.current().Players[0].Hits != 2 {
		t.Fatal(p.Stats.JP, v.Live.Session.current())
	}
}
func TestJPProvisionalGroupShareAndResetOrigin(t *testing.T) {
	now := time.Now()
	v, _ := newJPProvisionalInput(now, 5000)
	_, _ = v.append(provisionalValue(now, "1", 1000))
	e := provisionalValue(now.Add(2*time.Second), "2", 3000)
	e.ActorToken = "party2"
	_, _ = v.append(e)
	_ = v.tick(now.Add(2 * time.Second))
	b := v.Live.Session.current()
	if len(b.Players) != 2 || b.Players[0].Damage != 3000 || b.Players[0].DPS != 1500 || b.Players[0].Share != 0.75 || b.Damage != 4000 {
		t.Fatal(b)
	}
	reset, _ := newJPProvisionalInput(now.Add(3*time.Second), 5000)
	accepted, err := reset.append(e)
	if err != nil || accepted || reset.Live.Session != nil {
		t.Fatal("pre-reset event included")
	}
}
