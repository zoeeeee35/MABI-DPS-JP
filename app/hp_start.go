package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"
)

type HPStartEvidence struct {
	State        string
	Value        *uint64 `json:",omitempty"`
	At           time.Time
	Reason       string
	LastObserved *uint64 `json:",omitempty"`
	ObservedAt   time.Time
}
type HPObservation struct {
	At       time.Time
	Stream   int
	Frame    uint64
	Value    uint64
	Previous *uint64 `json:",omitempty"`
	Source   string
}

func unknownHPStart(at time.Time, reason string) HPStartEvidence {
	return HPStartEvidence{State: "Unknown", At: at, Reason: reason}
}

// Prior observations cannot prove a boundary value without complete HP adjustments.
func (h JPHealth) boundary(at time.Time) HPStartEvidence {
	r := unknownHPStart(at, "境界時刻のHP通知・全増減の完全性が未確認")
	for _, v := range h.History {
		if v.At.Before(at) {
			n := v.Value
			r.LastObserved = &n
			r.ObservedAt = v.At
		}
	}
	return r
}
func exportHPHistory(w io.Writer, values map[string]JPHealth) error {
	c := csv.NewWriter(w)
	_ = c.Write([]string{"entity_id", "timestamp", "stream", "frame", "current_hp", "previous_hp", "delta", "source", "first_observed_hp", "capture_start_state", "capture_start_reason", "history_dropped", "battle_start_at", "battle_start_state", "battle_start_reason"})
	for id, h := range values {
		for _, v := range h.History {
			prev, delta := "", ""
			if v.Previous != nil {
				prev = strconv.FormatUint(*v.Previous, 10)
				if v.Value >= *v.Previous {
					delta = "+" + strconv.FormatUint(v.Value-*v.Previous, 10)
				} else {
					delta = "-" + strconv.FormatUint(*v.Previous-v.Value, 10)
				}
			}
			if e := c.Write([]string{id, v.At.Format(time.RFC3339Nano), strconv.Itoa(v.Stream), strconv.FormatUint(v.Frame, 10), strconv.FormatUint(v.Value, 10), prev, delta, v.Source, fmt.Sprint(h.FirstObserved.Value), h.CaptureStart.State, h.CaptureStart.Reason, strconv.FormatUint(h.HistoryDropped, 10), h.BattleStart.At.Format(time.RFC3339Nano), h.BattleStart.State, h.BattleStart.Reason}); e != nil {
				return e
			}
		}
	}
	c.Flush()
	return c.Error()
}
