package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
)

func TestLocalFireProbe(t *testing.T) {
	path := os.Getenv("MABI_FIRE_PCAP")
	if path == "" {
		t.Skip("local capture")
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	counts := map[string]int{}
	sums := map[string]float64{}
	var a, b uint64
	var value uint32
	s, e := replayPCAPObserved(f, 16000, func(id uint32, p []byte, out bool) {
		if out {
			return
		}
		if id == 0x1d4f8 {
			if sk, ok := decodeJPSkill(p); ok {
				counts[fmt.Sprintf("skill/%X/%s", sk.Actor, sk.Class)]++
			}
		}
		if id == 0x5243 && len(p) == 53 {
			counts[fmt.Sprintf("5243/%X", binary.LittleEndian.Uint64(p))]++
			sums[fmt.Sprintf("5243/%X", binary.LittleEndian.Uint64(p))] += float64(binary.LittleEndian.Uint32(p[16:]))
		}
		if id == 0x4ff7 && len(p) >= 16 {
			actor := binary.LittleEndian.Uint64(p)
			counts[fmt.Sprintf("4ff7/%X/len%d", actor, len(p))]++
			for off := 16; off+8 <= len(p); off++ {
				v := math.Float64frombits(binary.LittleEndian.Uint64(p[off:]))
				if v > 0 && v < 1e9 && math.Trunc(v) == v {
					sums[fmt.Sprintf("4ff7/%X/f64offset%d", actor, off)] += v
				}
			}
			if actor == a && binary.LittleEndian.Uint64(p[8:]) == b && value > 0 {
				counts["paired"]++
				for off := 16; off < len(p); off++ {
					if off+2 <= len(p) && value < 65536 && uint32(binary.LittleEndian.Uint16(p[off:])) == value {
						counts[fmt.Sprintf("match/all-u16/%d", off)]++
					}
					vv, nn := binary.Uvarint(p[off:])
					if nn > 0 && vv == uint64(value) {
						counts[fmt.Sprintf("match/all-varuint/%d", off)]++
					}
				}
				for off := 16; off+4 <= len(p); off++ {
					if off+8 <= len(p) && math.Float64frombits(binary.LittleEndian.Uint64(p[off:])) == float64(value) {
						counts[fmt.Sprintf("match/f64/%d", off)]++
					}
					if math.Float32frombits(binary.LittleEndian.Uint32(p[off:])) == float32(value) {
						counts[fmt.Sprintf("match/f32/%d", off)]++
					}
					if value < 65536 && uint32(binary.LittleEndian.Uint16(p[off:])) == value {
						counts[fmt.Sprintf("match/u16/%d", off)]++
					}
					if value < 16777216 && (binary.LittleEndian.Uint32(p[off:])&0xffffff) == value {
						counts[fmt.Sprintf("match/u24/%d", off)]++
					}
					if binary.LittleEndian.Uint32(p[off:]) == uint32(-int32(value)) {
						counts[fmt.Sprintf("match/negative32/%d", off)]++
					}
					if binary.BigEndian.Uint32(p[off:]) == value {
						counts[fmt.Sprintf("match/be32/%d", off)]++
					}
					v, n := binary.Uvarint(p[off:])
					if n > 0 && v == uint64(value) {
						counts[fmt.Sprintf("match/varuint/%d", off)]++
					}
					if n > 0 && int64(v>>1)^-int64(v&1) == int64(value) {
						counts[fmt.Sprintf("match/zigzag/%d", off)]++
					}
				}
			}
		}
		if id == 0x5243 && len(p) == 53 {
			a = binary.LittleEndian.Uint64(p)
			b = binary.LittleEndian.Uint64(p[8:])
			value = binary.LittleEndian.Uint32(p[16:])
		} else {
			a = 0
			b = 0
			value = 0
		}
	})
	if e != nil {
		t.Fatal(e)
	}
	l := s.provisionalLog
	if l == nil || l.Detected == nil {
		t.Fatal("missing identity")
	}
	l.Party = l.Detected
	result, err := analyze(Request{Log: *l, Until: s.Provisional.Until, Idle: 5000})
	if err != nil {
		t.Fatal(err)
	}
	summary := captureSummary(result.Battles)
	if summary == nil || summary.Damage != 274821 {
		t.Fatal("known damage changed")
	}
	fire, ice := false, false
	for _, row := range summary.Players {
		if row.Name == "火炎術師 546B4E" {
			fire = true
			if row.Hits != 0 || row.DamageStatus != "unobserved" {
				t.Fatal("unobserved fire reported as zero")
			}
		}
		if row.Name == "氷結術師 5473F4" {
			ice = true
			if row.Hits != 134 || row.Damage != 274821 || row.DamageStatus != "candidate" {
				t.Fatal("known actor changed")
			}
		}
	}
	if !fire || !ice {
		t.Fatal("missing player")
	}
	data, _ := json.MarshalIndent(map[string]any{"counts": counts, "sums": sums, "invalidJP": s.JP.Invalid, "fireDamageStatus": "unobserved", "knownDamage": 274821}, "", "  ")
	out := os.Getenv("MABI_FIRE_REPORT")
	if out != "" {
		if e = os.WriteFile(out, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
