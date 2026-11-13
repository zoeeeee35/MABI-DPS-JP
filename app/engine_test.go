package main

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func fixture(t *testing.T) Request {
	t.Helper()
	data, e := assets.ReadFile("sample.json")
	if e != nil {
		t.Fatal(e)
	}
	var l CombatLog
	if e = json.Unmarshal(data, &l); e != nil {
		t.Fatal(e)
	}
	return Request{Log: l, Until: 27000, Idle: 5000}
}
func TestSampleTotals(t *testing.T) {
	q := fixture(t)
	r, e := analyze(q)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Battles) != 2 {
		t.Fatal(r)
	}
	b := r.Battles[0]
	if b.Damage != 9000 || b.Duration != 12 || b.Active || b.Reason != "explicit" {
		t.Fatal(b)
	}
	p := b.Players[0]
	if p.Damage != 6000 || p.DPS != 500 || p.Hits != 6 || p.Crits != 2 || math.Abs(p.Share-2.0/3) > 1e-9 || math.Abs(p.CritRate-1.0/3) > 1e-9 {
		t.Fatal(p)
	}
	if len(p.Skills) != 2 || p.Skills[0].Damage != 3000 {
		t.Fatal(p.Skills)
	}
	if b.Players[1].DPS != 250 {
		t.Fatal(b.Players[1])
	}
	b = r.Battles[1]
	if b.Damage != 2000 || b.Duration != 2 || b.Active || b.Reason != "idle" {
		t.Fatal(b)
	}
	if r.LiveVerified {
		t.Fatal("must never claim live verification")
	}
}
func TestReplayAndBoundary(t *testing.T) {
	q := fixture(t)
	q.Until = 3000
	r, e := analyze(q)
	if e != nil {
		t.Fatal(e)
	}
	b := r.Battles[0]
	if !b.Active || b.Damage != 3000 || b.Duration != 3 {
		t.Fatal(b)
	}
	q.Until = 26999
	r, err := analyze(q)
	if err != nil {
		t.Fatal(err)
	}
	pending := r.Battles[1]
	if !pending.Active || pending.End != 22000 || pending.Duration != 2 {
		t.Fatal("grace period must not extend measured duration", pending)
	}
	q.Until = 27000
	r, err = analyze(q)
	if err != nil {
		t.Fatal(err)
	}
	ended := r.Battles[1]
	if ended.Active || ended.End != pending.End || ended.Duration != pending.Duration {
		t.Fatal("ending at the idle boundary must not rewind the display", ended)
	}
}

func TestHitWithinGraceKeepsBattle(t *testing.T) {
	q := fixture(t)
	hit := q.Log.Events[len(q.Log.Events)-1]
	hit.ID = "grace-hit"
	hit.At = 26999
	q.Log.Events = append(q.Log.Events, hit)
	q.Until = 27000
	r, err := analyze(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Battles) != 2 || !r.Battles[1].Active || r.Battles[1].End != hit.At {
		t.Fatal("hit before the deadline must extend the same battle", r.Battles)
	}
}

func TestSingleHitAndZero(t *testing.T) {
	q := fixture(t)
	q.Log.Events = q.Log.Events[:1]
	q.Until = 0
	r, _ := analyze(q)
	if r.Battles[0].Players[0].DPS != 1000 {
		t.Fatal(r)
	}
	q.Log.Events[0].Damage = 0
	r, _ = analyze(q)
	if r.Battles[0].Players[0].Share != 0 || r.Battles[0].Players[0].DPS != 0 {
		t.Fatal(r)
	}
}
func TestNonPartyExcluded(t *testing.T) {
	q := fixture(t)
	q.Log.Players[0].Party = false
	r, e := analyze(q)
	if e != nil {
		t.Fatal(e)
	}
	if r.Battles[0].Damage != 3000 || len(r.Battles[0].Players) != 1 || r.Battles[0].Players[0].Share != 1 {
		t.Fatal(r)
	}
}
func TestInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Request)
	}{
		{"duplicate event", func(q *Request) { q.Log.Events[1].ID = q.Log.Events[0].ID }},
		{"out of order", func(q *Request) { q.Log.Events[1].At = -1 }},
		{"negative damage", func(q *Request) { q.Log.Events[0].Damage = -1 }},
		{"missing actor", func(q *Request) { q.Log.Events[0].PlayerID = "none" }},
		{"unknown region", func(q *Request) { q.Log.Region = "invalid" }},
		{"unsafe number", func(q *Request) { q.Log.Events[0].Damage = 1000000001 }},
		{"invalid timeout", func(q *Request) { q.Idle = 0 }},
		{"duplicate roster", func(q *Request) { q.Log.Players[1].ID = q.Log.Players[0].ID }},
		{"unknown event", func(q *Request) { q.Log.Events[0].Type = "raw" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := fixture(t)
			tc.mutate(&q)
			if _, e := analyze(q); e == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}
func TestStrictJSON(t *testing.T) {
	for _, input := range []string{`{"password":"secret"}`, `{} {}`, `{"untilMs":1.5}`, `{"untilMs":1e400}`} {
		var q Request
		if e := decode(strings.NewReader(input), &q); e == nil {
			t.Fatal("accepted invalid JSON", input)
		}
	}
}
func TestSplitOnGap(t *testing.T) {
	q := fixture(t)
	q.Log.Events = []Hit{q.Log.Events[0], q.Log.Events[1]}
	q.Log.Events[1].At = 5000
	q.Until = 5000
	r, e := analyze(q)
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Battles) != 2 || r.Battles[0].Active || !r.Battles[1].Active {
		t.Fatal(r)
	}
}
