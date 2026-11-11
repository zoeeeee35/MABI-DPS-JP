package main

import (
	"fmt"
	"math"
)

type MeasurementWindow struct {
	Start int64 `json:"startMs"`
	End   int64 `json:"endMs"`
}

func analyzeMeasurement(q Request) (Result, error) {
	w := q.Window
	if w == nil || w.Start < 0 || w.End <= w.Start || w.End > maxTime {
		return Result{}, fmt.Errorf("計測区間が不正です")
	}
	base := trimMeasurementRequest(q)
	base.Window = nil
	r, err := analyze(base)
	if err != nil {
		return r, err
	}
	b := captureSummary(r.Battles)
	if b == nil {
		b = &Battle{Players: []Row{}, DamageEvents: []Hit{}}
	}
	b.Number = 1
	b.Start = w.Start
	b.End = w.End
	b.Duration = float64(w.End-w.Start) / 1000
	b.Active = false
	b.Reason = "timed-measurement"
	for i := range b.Players {
		p := &b.Players[i]
		p.DPS = float64(p.Damage) / math.Max(1, b.Duration)
		for j := range p.Skills {
			p.Skills[j].DPS = float64(p.Skills[j].Damage) / math.Max(1, b.Duration)
		}
	}
	r.Battles = []Battle{*b}
	return r, nil
}

func trimMeasurementRequest(q Request) Request {
	if q.Window == nil {
		return q
	}
	w := q.Window
	base := q
	base.Until = w.End
	base.Log.Events = nil
	base.Log.SkillUses = nil
	for _, e := range q.Log.Events {
		if e.At >= w.Start && e.At < w.End {
			base.Log.Events = append(base.Log.Events, e)
		}
	}
	for _, u := range q.Log.SkillUses {
		if u.At >= w.Start && u.At < w.End {
			base.Log.SkillUses = append(base.Log.SkillUses, u)
		}
	}
	return base
}
