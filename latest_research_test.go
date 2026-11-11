package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestLocalLatestResearch(t *testing.T) {
	path := os.Getenv("MABI_LATEST_PCAP")
	if path == "" {
		t.Skip("local")
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	type hp struct {
		Frame  int
		Entity string
		Type   uint32
		Value  uint64
	}
	type hit struct {
		Frame         int
		Actor, Target string
		Damage        uint64
	}
	type candidate struct {
		Frame          int
		ID             string
		Length, Offset int
		Value          uint64
		EntityOffsets  []int
	}
	updates := []hp{}
	hits := []hit{}
	candidates := []candidate{}
	classes := map[string]map[string]int{}
	index := 0
	stats, e := replayPCAPObserved(f, 16000, func(id uint32, p []byte, out bool) {
		if out {
			return
		}
		index++
		if id == 0x1d596 && len(p) == 21 {
			updates = append(updates, hp{index, fmt.Sprintf("%X", binary.LittleEndian.Uint64(p)), binary.LittleEndian.Uint32(p[8:]), binary.LittleEndian.Uint64(p[12:])})
		}
		if id == 0x5243 && len(p) == 53 {
			hits = append(hits, hit{index, fmt.Sprintf("%X", binary.LittleEndian.Uint64(p)), fmt.Sprintf("%X", binary.LittleEndian.Uint64(p[8:])), binary.LittleEndian.Uint64(p[16:])})
		}
		if id == 0x1d4f8 {
			if s, ok := decodeJPSkillUse(p); ok {
				k := fmt.Sprintf("%X", s.Actor)
				if classes[k] == nil {
					classes[k] = map[string]int{}
				}
				classes[k][s.Code]++
			}
		}
		for off := 0; off+4 <= len(p); off++ {
			v := uint64(binary.LittleEndian.Uint32(p[off:]))
			if v == 5919 || v == 5174 || v == 6939 || v == 2695 {
				candidates = append(candidates, candidate{Frame: index, ID: fmt.Sprintf("%X", id), Length: len(p), Offset: off, Value: v})
			}
		}
	})
	if e != nil {
		t.Fatal(e)
	}
	data, _ := json.MarshalIndent(map[string]any{"hp": updates, "damage": hits, "knownValueFields": candidates, "skillUses": classes, "hpMatches": stats.JP.HPMatches}, "", "  ")
	if out := os.Getenv("MABI_LATEST_REPORT"); out != "" {
		if e = os.WriteFile(out, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
	t.Logf("HP=%d damage=%d candidates=%d matches=%d", len(updates), len(hits), len(candidates), stats.JP.HPMatches)
}
