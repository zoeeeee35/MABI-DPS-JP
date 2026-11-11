package main

import (
	"encoding/binary"
	"time"
)

type JPHealth struct {
	FirstObserved  HPObservation
	CaptureStart   HPStartEvidence
	BattleStart    HPStartEvidence
	History        []HPObservation
	HistoryDropped uint64
	Entity         uint64
	Current        uint64
	Previous       uint64
	HasPrevious    bool
	At             time.Time
	MatchedDamage  uint64
}
type jpHealthTracker struct {
	Stream int
	Frame  uint64
	Values map[uint64]JPHealth
	Damage []jp9Candidate
}

func (h *jpHealthTracker) observe(id uint32, p []byte, at time.Time) {
	if id == 0x5243 && len(p) == 53 {
		target := binary.LittleEndian.Uint64(p[8:])
		damage := binary.LittleEndian.Uint64(p[16:])
		v, ok := h.Values[target]
		if ok && v.HasPrevious && v.Previous > v.Current && v.MatchedDamage == 0 && v.Previous-v.Current == damage && at.Sub(v.At) >= 0 && at.Sub(v.At) <= time.Second {
			v.MatchedDamage = damage
			h.Values[target] = v
			return
		}
		h.Damage = append(h.Damage, jp9Candidate{Actor: binary.LittleEndian.Uint64(p), Target: binary.LittleEndian.Uint64(p[8:]), Damage64: binary.LittleEndian.Uint64(p[16:]), At: at})
		if len(h.Damage) > 256 {
			h.Damage = h.Damage[1:]
		}
	}
	if id != 0x1d596 || len(p) != 21 || binary.LittleEndian.Uint32(p[8:]) != 8 {
		return
	}
	if h.Values == nil {
		h.Values = map[uint64]JPHealth{}
	}
	entity := binary.LittleEndian.Uint64(p)
	old, exists := h.Values[entity]
	if !exists && len(h.Values) >= 4096 {
		return
	}
	v := JPHealth{Entity: entity, Current: binary.LittleEndian.Uint64(p[12:]), Previous: old.Current, HasPrevious: exists, At: at}
	observation := HPObservation{At: at, Stream: h.Stream, Frame: h.Frame, Value: v.Current, Source: "0x1D596/resource=8/current@12"}
	if exists {
		prev := old.Current
		observation.Previous = &prev
		v.FirstObserved = old.FirstObserved
		v.CaptureStart = old.CaptureStart
		v.HistoryDropped = old.HistoryDropped
		v.History = append([]HPObservation(nil), old.History...)
	} else {
		v.FirstObserved = observation
		v.CaptureStart = unknownHPStart(time.Time{}, "収録開始時のHPは未確認。最初の観測値で代用しない")
	}
	v.History = append(v.History, observation)
	if len(v.History) > 512 {
		v.History = v.History[len(v.History)-512:]
		v.HistoryDropped++
	}
	if exists && v.Current < old.Current {

		for i := len(h.Damage) - 1; i >= 0; i-- {
			d := h.Damage[i]
			dt := at.Sub(d.At)
			if dt >= 0 && dt <= time.Second && d.Target == entity && d.Damage64 == old.Current-v.Current {
				v.MatchedDamage = d.Damage64
				h.Damage = append(h.Damage[:i], h.Damage[i+1:]...)
				break
			}
		}
	}
	h.Values[entity] = v
}
