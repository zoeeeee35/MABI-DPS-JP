package main

import (
	"encoding/binary"
	"os"
	"testing"
)

func TestLatestDamageAndHPRegression(t *testing.T) {
	path := os.Getenv("MABI_LATEST_PCAP")
	if path == "" {
		t.Skip("local capture")
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	hp := []uint64{}
	damage := map[uint64]int{}
	skills := 0
	_, e = replayPCAPObserved(f, 16000, func(id uint32, p []byte, out bool) {
		if out {
			return
		}
		if id == 0x1d596 && len(p) == 21 && binary.LittleEndian.Uint64(p) == 0x548446 && binary.LittleEndian.Uint32(p[8:]) == 8 {
			hp = append(hp, binary.LittleEndian.Uint64(p[12:]))
		}
		if id == 0x5243 && len(p) == 53 && binary.LittleEndian.Uint64(p[8:]) == 0x548446 {
			damage[binary.LittleEndian.Uint64(p[16:])]++
		}
		if id == 0x1d4f8 {
			if s, ok := decodeJPSkillUse(p); ok && s.Actor == 0x548446 {
				skills++
			}
		}
	})
	if e != nil {
		t.Fatal(e)
	}
	pairs := map[[2]uint64]bool{}
	for i := 1; i < len(hp); i++ {
		pairs[[2]uint64{hp[i-1], hp[i]}] = true
	}
	for _, p := range [][2]uint64{{4426, 3100}, {4305, 2979}, {4050, 2695}} {
		if !pairs[p] {
			t.Fatal("HP transition missing", p)
		}
	}
	if damage[1326] != 2 || damage[1355] != 1 || skills != 65 {
		t.Fatal(damage[1326], damage[1355], skills)
	}
}
