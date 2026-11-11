package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestLocalClassificationCapture(t *testing.T) {
	path := os.Getenv("MABI_CLASSIFICATION_PCAP")
	if path == "" {
		t.Skip("requires local user capture")
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	audit := map[string]map[string]int{}
	for _, id := range []uint64{0x544dc0, 0x5447dd, 0x544d4b, 0x54495d} {
		audit[fmt.Sprintf("%X", id)] = map[string]int{}
	}
	var lastActor, lastTarget uint64
	var lastValue uint32
	s, e := replayPCAPObserved(f, 16000, func(message uint32, body []byte, out bool) {
		if out {
			return
		}
		for _, id := range []uint64{0x544dc0, 0x5447dd, 0x544d4b, 0x54495d} {
			m := audit[fmt.Sprintf("%X", id)]
			if message == 0x4ff7 && len(body) >= 16 && binary.LittleEndian.Uint64(body) == id {
				m[fmt.Sprintf("fourFF7PayloadLength%d", len(body))]++
				if id == lastActor && binary.LittleEndian.Uint64(body[8:]) == lastTarget && lastValue != 0 {
					m["pairedAfter5243"]++
					for off := 16; off+4 <= len(body); off++ {
						if binary.LittleEndian.Uint32(body[off:]) == lastValue {
							m[fmt.Sprintf("paired5243ValueAtOffset%d", off)]++
						}
					}
				}
			}
			for off := 0; off+8 <= len(body); off++ {
				if binary.LittleEndian.Uint64(body[off:]) == id {
					m[fmt.Sprintf("message%X/idOffset%d", message, off)]++
				}
			}
			if message == 0x1d4f8 && len(body) >= 12 && binary.LittleEndian.Uint64(body) == id {
				n := int(binary.LittleEndian.Uint32(body[8:12]))
				if n > 0 && n%2 == 0 && 12+n <= len(body) {
					u := make([]uint16, n/2)
					for i := range u {
						u[i] = binary.LittleEndian.Uint16(body[12+i*2:])
					}
					code := strings.TrimRight(string(utf16.Decode(u)), "\x00")
					prefix := strings.SplitN(code, "_", 2)[0]
					m["skillPrefix:"+prefix]++
					if _, ok := decodeJPSkill(body); !ok {
						for _, r := range code {
							if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
								m[fmt.Sprintf("rejectedRuneU%04X", r)]++
							}
						}
					}
				}
			}
		}
		if message == 0x5243 && len(body) == 53 {
			lastActor = binary.LittleEndian.Uint64(body)
			lastTarget = binary.LittleEndian.Uint64(body[8:])
			lastValue = binary.LittleEndian.Uint32(body[16:])
		} else {
			lastActor = 0
			lastTarget = 0
			lastValue = 0
		}
	})
	f.Close()
	if e != nil {
		t.Fatal(e)
	}
	l := s.provisionalLog
	if l == nil || l.Detected == nil {
		t.Fatal("missing detected data")
	}
	l.Party = l.Detected
	result, e := analyze(Request{Log: *l, Until: s.Provisional.Until, Idle: 5000})
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("combat segments=%d", len(result.Battles))
	if len(result.Battles) > 0 {
		b := result.Battles[len(result.Battles)-1]
		for _, p := range b.Players {
			t.Logf("last segment %s damage=%d", p.Name, p.Damage)
		}
	}
	summary := captureSummary(result.Battles)
	if summary != nil && summary.Damage != 6230451 {
		t.Fatalf("unexpected actor or duplicate event counted: %d", summary.Damage)
	}
	if summary == nil {
		t.Fatal("missing summary")
	}
	report := map[string]any{}
	for _, raw := range []uint64{0x544dc0, 0x5447dd, 0x544d4b, 0x54495d} {
		id := s.replayAliases[raw]
		count, sum, selectedCount, selectedSum := 0, int64(0), 0, int64(0)
		for _, h := range l.Events {
			if h.PlayerID == id {
				count++
				sum += h.Damage
			}
		}
		for _, b := range result.Battles {
			for _, p := range b.Players {
				if p.ID == id {
					selectedCount += p.Hits
					selectedSum += p.Damage
				}
			}
		}
		if raw == 0x544dc0 || raw == 0x5447dd {
			wantN, wantSum := 318, int64(6143222)
			if raw == 0x5447dd {
				wantN, wantSum = 48, 87229
			}
			if count != wantN || sum != wantSum || selectedCount != wantN || selectedSum != wantSum {
				t.Fatalf("%X unexpected aggregation", raw)
			}
			found := false
			for _, p := range summary.Players {
				if p.ID == id {
					found = true
					if p.Damage != wantSum || !strings.HasSuffix(p.Name, fmt.Sprintf("%X", raw)) {
						t.Fatalf("%X display mapping mismatch", raw)
					}
				}
			}
			if !found {
				t.Fatalf("%X missing summary row", raw)
			}
		}
		if raw == 0x544d4b || raw == 0x54495d {
			found := false
			for _, p := range summary.Players {
				if p.ID == id {
					found = true
					if p.Damage != 0 || p.Hits != 0 {
						t.Fatalf("%X unrelated damage assigned", raw)
					}
				}
			}
			if !found {
				t.Fatalf("%X zero-damage player lost", raw)
			}
		}
		expectedSkills := map[uint64]int{0x544dc0: 69, 0x5447dd: 88, 0x544d4b: 59, 0x54495d: 52}
		if audit[fmt.Sprintf("%X", raw)]["message1D4F8/idOffset0"] != expectedSkills[raw] {
			t.Fatalf("%X skill frames not fully recovered", raw)
		}
		entity := l.Detected.Entities[id]
		report[fmt.Sprintf("%X", raw)] = map[string]any{"frameAudit": audit[fmt.Sprintf("%X", raw)], "rawEvents": count, "rawSum": sum, "rankedEvents": selectedCount, "rankedSum": selectedSum, "class": entity.Class, "evidence": entity.Evidence, "observations": entity.Observations}
		t.Logf("%X raw=%d/%d ranked=%d/%d class=%s evidence=%s", raw, count, sum, selectedCount, selectedSum, entity.Class, entity.Evidence)
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	if out := os.Getenv("MABI_CLASSIFICATION_REPORT"); out != "" {
		_ = os.MkdirAll(filepath.Dir(out), 0700)
		if e = os.WriteFile(out, data, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
