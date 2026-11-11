package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
	"time"
)

func TestLocalResourcePercentProbe(t *testing.T) {
	path := os.Getenv("MABI_PERCENT_PCAP")
	if path == "" {
		t.Skip("local research")
	}
	type hp struct {
		Value uint64
		At    time.Time
	}
	type stat struct {
		Count    int
		Min, Max float64
		HPCount  int
		First    []float64
		Classes  map[string]int
	}
	values := map[uint64]*stat{}
	health := map[uint64]hp{}
	classes := map[uint64]map[string]int{}
	pairs := []map[string]any{}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	_, e = replayPCAPAudited(f, 16000, nil, func(stream int, index uint64, at time.Time, id uint32, p []byte, out bool) {
		if out {
			return
		}
		if id == 0x1d4f8 {
			if s, ok := decodeJPSkillUse(p); ok {
				if classes[s.Actor] == nil {
					classes[s.Actor] = map[string]int{}
				}
				name := s.Code
				for i, c := range name {
					if c == '_' {
						name = name[:i]
						break
					}
				}
				classes[s.Actor][name]++
			}
		}
		if id == 0x1d596 && len(p) == 21 && binary.LittleEndian.Uint32(p[8:]) == 8 {
			health[binary.LittleEndian.Uint64(p)] = hp{binary.LittleEndian.Uint64(p[12:]), at}
		}
		if id != 0x1d595 || len(p) != 13 {
			return
		}
		entity := binary.LittleEndian.Uint64(p)
		v := float64(math.Float32frombits(binary.LittleEndian.Uint32(p[8:])))
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return
		}
		s := values[entity]
		if s == nil {
			s = &stat{Min: v}
			values[entity] = s
		}
		s.Count++
		if v < s.Min {
			s.Min = v
		}
		if v > s.Max {
			s.Max = v
		}
		if len(s.First) < 15 {
			s.First = append(s.First, v)
		}
		if h, ok := health[entity]; ok {
			s.HPCount++
			if v > 0 && h.Value > 0 && at.Sub(h.At) >= 0 && at.Sub(h.At) < 250*time.Millisecond && len(pairs) < 150 {
				pairs = append(pairs, map[string]any{"entity": fmt.Sprintf("%X", entity), "hp": h.Value, "rawFloat": v, "ifPercentThenMaxHP": float64(h.Value) * 100 / v})
			}
		}
	})
	if e != nil {
		t.Fatal(e)
	}
	rows := map[string]any{}
	for entity, s := range values {
		s.Classes = classes[entity]
		rows[fmt.Sprintf("%X", entity)] = s
	}
	b, _ := json.MarshalIndent(map[string]any{"unconfirmedFloatCandidates": rows, "temporalCoincidencesOnly": pairs}, "", "  ")
	if out := os.Getenv("MABI_PERCENT_REPORT"); out != "" {
		if e = os.WriteFile(out, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
