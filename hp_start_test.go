package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"strings"
	"testing"
	"time"
)

func hpBody(id, value uint64) []byte {
	p := make([]byte, 21)
	binary.LittleEndian.PutUint64(p, id)
	binary.LittleEndian.PutUint32(p[8:], 8)
	binary.LittleEndian.PutUint64(p[12:], value)
	return p
}
func TestHPFirstObservedIsNotStartingHP(t *testing.T) {
	h := jpHealthTracker{}
	at := time.Unix(100, 0)
	h.observe(0x1d596, hpBody(12, 0), at)
	v := h.Values[12]
	if v.FirstObserved.Value != 0 || v.CaptureStart.State != "Unknown" || v.CaptureStart.Value != nil {
		t.Fatal(v)
	}
	b := v.boundary(at.Add(time.Second))
	if b.Value != nil || b.State != "Unknown" || b.LastObserved == nil || *b.LastObserved != 0 {
		t.Fatal(b)
	}
	h.observe(0x1d596, hpBody(12, 500), at.Add(2*time.Second))
	h.observe(0x1d596, hpBody(13, 999), at)
	if h.Values[12].FirstObserved.Value != 0 || h.Values[13].FirstObserved.Value != 999 || len(h.Values[12].History) != 2 {
		t.Fatal(h.Values)
	}
	var csv bytes.Buffer
	if e := exportHPHistory(&csv, map[string]JPHealth{"C": h.Values[12]}); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(csv.String(), "+500") || !strings.Contains(csv.String(), "Unknown") {
		t.Fatal(csv.String())
	}
}
func TestHPBoundedHistoryAndSnapshot(t *testing.T) {
	h := jpHealthTracker{}
	at := time.Unix(100, 0)
	h.observe(0x1d596, hpBody(1, 10), at)
	old := h.Values[1]
	for i := 0; i < 600; i++ {
		h.observe(0x1d596, hpBody(1, uint64(i)), at.Add(time.Duration(i+1)*time.Second))
	}
	v := h.Values[1]
	if len(v.History) != 512 || v.HistoryDropped != 89 || v.FirstObserved.Value != 10 || len(old.History) != 1 || old.History[0].Value != 10 {
		t.Fatal(v.HistoryDropped)
	}
}
func TestHPDamageDoesNotProveInitialHP(t *testing.T) {
	h := jpHealthTracker{}
	at := time.Unix(100, 0)
	p := make([]byte, 53)
	binary.LittleEndian.PutUint64(p[8:], 9)
	binary.LittleEndian.PutUint64(p[16:], 919)
	h.observe(0x5243, p, at)
	h.observe(0x1d596, hpBody(9, 13447), at)
	if h.Values[9].CaptureStart.Value != nil {
		t.Fatal("damage alone reconstructed initial HP")
	}
}
func TestOldHP919And1356Matches(t *testing.T) {
	path := os.Getenv("MABI_HP_PCAP")
	if path == "" {
		t.Skip("local capture")
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	trackers := map[int]*jpHealthTracker{}
	matches := map[uint64]bool{}
	_, e = replayPCAPAudited(f, 16000, nil, func(stream int, index uint64, at time.Time, id uint32, p []byte, out bool) {
		if out {
			return
		}
		if trackers[stream] == nil {
			trackers[stream] = &jpHealthTracker{}
		}
		h := trackers[stream]
		h.observe(id, p, at)
		if v, ok := h.Values[0x563f8b]; ok && v.MatchedDamage != 0 {
			matches[v.MatchedDamage] = true
		}
		if v, ok := h.Values[0x563f8b]; ok && v.CaptureStart.Value != nil {
			t.Error("old capture inferred start")
		}
	})
	if e != nil {
		t.Fatal(e)
	}
	if !matches[919] || !matches[1356] {
		t.Fatal(matches)
	}
}
