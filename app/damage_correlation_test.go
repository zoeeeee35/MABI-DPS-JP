package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

func TestLocalDamageCorrelation(t *testing.T) {
	path := os.Getenv("MABI_FIRE_PCAP")
	if path == "" {
		t.Skip("local")
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	type frame struct {
		id    uint32
		p     []byte
		index int
	}
	recent := []frame{}
	known := []frame{}
	matches := map[string]int{}
	seen := map[string]bool{}
	index := 0
	compare := func(k, x frame) {
		if len(k.p) != 53 || x.id == 0x5243 {
			return
		}
		value := binary.LittleEndian.Uint32(k.p[16:])
		if value < 100 {
			return
		}
		actor := k.p[:8]
		target := k.p[8:16]
		if !bytes.Contains(x.p, actor) && !bytes.Contains(x.p, target) {
			return
		}
		for off := 0; off+4 <= len(x.p); off++ {
			kind := ""
			if binary.LittleEndian.Uint32(x.p[off:]) == value {
				kind = "u32"
			} else if math.Float32frombits(binary.LittleEndian.Uint32(x.p[off:])) == float32(value) {
				kind = "f32"
			} else if off+8 <= len(x.p) && math.Float64frombits(binary.LittleEndian.Uint64(x.p[off:])) == float64(value) {
				kind = "f64"
			}
			if kind != "" {
				key := fmt.Sprintf("%d/%X/%d", k.index, x.id, off)
				if !seen[key] {
					seen[key] = true
					matches[fmt.Sprintf("message%X/length%d/offset%d/"+kind, x.id, len(x.p), off)]++
				}
			}
		}
	}
	_, e = replayPCAPObserved(f, 16000, func(id uint32, p []byte, out bool) {
		if out || len(p) > 4096 {
			return
		}
		index++
		x := frame{id, append([]byte(nil), p...), index}
		for _, k := range known {
			if index-k.index <= 32 {
				compare(k, x)
			}
		}
		if id == 0x5243 && len(p) == 53 {
			for _, prev := range recent {
				compare(x, prev)
			}
			known = append(known, x)
			if len(known) > 32 {
				known = known[1:]
			}
		}
		recent = append(recent, x)
		if len(recent) > 32 {
			recent = recent[1:]
		}
	})
	if e != nil {
		t.Fatal(e)
	}
	data, _ := json.MarshalIndent(matches, "", "  ")
	if out := os.Getenv("MABI_CORRELATION_REPORT"); out != "" {
		if e = os.WriteFile(out, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
