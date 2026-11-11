package main

import (
	"encoding/binary"
	"os"
	"testing"
	"time"
)

func TestHPUpdatesDoNotBecomeDamage(t *testing.T) {
	h := jpHealthTracker{}
	now := time.Now()
	p := make([]byte, 21)
	binary.LittleEndian.PutUint64(p, 123)
	binary.LittleEndian.PutUint32(p[8:], 8)
	binary.LittleEndian.PutUint64(p[12:], 14366)
	h.observe(0x1d596, p, now)
	d := make([]byte, 53)
	binary.LittleEndian.PutUint64(d[8:], 123)
	binary.LittleEndian.PutUint64(d[16:], 919)
	h.observe(0x5243, d, now)
	binary.LittleEndian.PutUint64(p[12:], 13447)
	h.observe(0x1d596, p, now.Add(time.Millisecond))
	if h.Values[123].MatchedDamage != 919 {
		t.Fatal(h.Values)
	}
	h.observe(0x1d596, p, now.Add(2*time.Millisecond))
	if h.Values[123].MatchedDamage != 0 {
		t.Fatal("duplicate match")
	}
}
func TestLocalHPFile(t *testing.T) {
	path := os.Getenv("MABI_HP_PCAP")
	if path == "" {
		t.Skip("local capture")
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	count := 0
	values := map[uint64]bool{}
	stats, e := replayPCAPObserved(f, 16000, func(id uint32, p []byte, out bool) {
		if !out && id == 0x1d596 && len(p) == 21 && binary.LittleEndian.Uint64(p) == 0x563f8b && binary.LittleEndian.Uint32(p[8:]) == 8 {
			count++
			values[binary.LittleEndian.Uint64(p[12:])] = true
		}
	})
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range []uint64{19842, 19159, 18567, 14366, 13447, 12091} {
		if !values[v] {
			t.Errorf("HP %d missing", v)
		}
	}
	t.Logf("self HP updates %d; all HP updates %d; target/time/value matches %d; brotli %d", count, stats.JP.HPUpdates, stats.JP.HPMatches, stats.JP.BrotliOK)
}
