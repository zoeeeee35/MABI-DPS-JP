package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"
)

// Local capture audit exports semantic aggregates only, never packet payloads.
func TestLocalCompanionUnknownDamage(t *testing.T) {
	path := os.Getenv("MABI_COMPANION_PCAP")
	if path == "" {
		t.Skip("local capture")
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	type pending struct {
		index uint64
		p     []byte
	}
	previous := map[int]pending{}
	recent := map[int][]pending{}
	laterMatches := map[string]map[string]uint64{}
	mismatches := map[string]map[string]uint64{}
	stats, e := replayPCAPAudited(f, 16000, nil, func(conn int, index uint64, at time.Time, id uint32, p []byte, out bool) {
		if out {
			return
		}
		if id == 0x4ff7 && len(p) == 35 {
			matches := []pending{}
			for _, d := range recent[conn] {
				if index > d.index && index-d.index <= 128 && bytes.Equal(d.p[:16], p[:16]) && bytes.Equal(d.p[32:40], p[24:32]) {
					matches = append(matches, d)
				}
			}
			if len(matches) == 1 && index-matches[0].index > 1 {
				d := matches[0]
				key := fmt.Sprintf("actor=%X effect=%08X", binary.LittleEndian.Uint64(p), binary.LittleEndian.Uint32(p[16:]))
				if laterMatches[key] == nil {
					laterMatches[key] = map[string]uint64{}
				}
				laterMatches[key]["hits"]++
				laterMatches[key]["damage"] += binary.LittleEndian.Uint64(d.p[16:])
				if lag := index - d.index; lag > laterMatches[key]["maxFrameDistance"] {
					laterMatches[key]["maxFrameDistance"] = lag
				}
			}
		}
		if id == 0x5243 && len(p) == 53 {
			recent[conn] = append(recent[conn], pending{index, append([]byte(nil), p...)})
			if len(recent[conn]) > 128 {
				recent[conn] = recent[conn][len(recent[conn])-128:]
			}
		}
		last := previous[conn]
		if last.p != nil && last.index+1 == index {
			actor, target := binary.LittleEndian.Uint64(last.p), binary.LittleEndian.Uint64(last.p[8:])
			key := fmt.Sprintf("damageActor=%X target=%X next=%X len=%d", actor, target, id, len(p))
			if id == 0x4ff7 && len(p) == 35 {
				ma, mt := binary.LittleEndian.Uint64(p), binary.LittleEndian.Uint64(p[8:])
				field := bytes.Equal(last.p[32:40], p[24:32])
				if actor == ma && target == mt && field {
					key = "matched"
				} else {
					key += fmt.Sprintf(" metadataActor=%X target=%X fieldEqual=%t effect=%08X", ma, mt, field, binary.LittleEndian.Uint32(p[16:]))
				}
			}
			if key != "matched" {
				if mismatches[key] == nil {
					mismatches[key] = map[string]uint64{}
				}
				mismatches[key]["hits"]++
				mismatches[key]["damage"] += binary.LittleEndian.Uint64(last.p[16:])
			}
		}
		if id == 0x5243 && len(p) == 53 {
			previous[conn] = pending{index, append([]byte(nil), p...)}
		} else {
			delete(previous, conn)
		}
	})
	if e != nil {
		t.Fatal(e)
	}
	if stats.provisionalLog == nil {
		t.Fatal("no combat log")
	}
	l := stats.provisionalLog
	type effect struct {
		Hits   int
		Damage int64
		Skills []string
	}
	type actor struct {
		ID                      string
		Hits, Unknown, Unpaired int
		Damage, UnknownDamage   int64
		Skills                  map[string]int
		Effects                 map[string]*effect
	}
	ids := map[string]string{}
	for raw, id := range stats.replayAliases {
		ids[id] = fmt.Sprintf("%X", raw)
	}
	actors := map[string]*actor{}
	ensure := func(id string) *actor {
		a := actors[id]
		if a == nil {
			a = &actor{ID: ids[id], Skills: map[string]int{}, Effects: map[string]*effect{}}
			actors[id] = a
		}
		return a
	}
	for _, u := range l.SkillUses {
		ensure(u.ActorID).Skills[u.Code]++
	}
	for _, h := range l.Events {
		if h.Damage <= 0 {
			continue
		}
		a := ensure(h.PlayerID)
		a.Hits++
		a.Damage += h.Damage
		key := "unpaired"
		var names []string
		if h.Link != nil && h.Link.Paired {
			key = fmt.Sprintf("0x%08X", h.Link.Effect)
			names = h.Link.Names
		} else {
			a.Unpaired++
		}
		ef := a.Effects[key]
		if ef == nil {
			ef = &effect{Skills: names}
			a.Effects[key] = ef
		}
		ef.Hits++
		ef.Damage += h.Damage
		if len(names) == 0 {
			a.Unknown++
			a.UnknownDamage += h.Damage
		}
	}
	out := []*actor{}
	for _, a := range actors {
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Damage > out[j].Damage })
	report := map[string]any{"nonAdjacentMatches": laterMatches, "mismatches": mismatches, "actors": out, "frames": stats.JP.InFrames, "damageEvents": stats.JP.Count, "gaps": stats.ReassemblyGaps, "recoveries": stats.JP.Recovered, "invalid": stats.JP.Invalid}
	data, _ := json.MarshalIndent(report, "", "  ")
	if e = os.WriteFile(os.Getenv("MABI_COMPANION_REPORT"), data, 0600); e != nil {
		t.Fatal(e)
	}
	t.Logf("damage=%d actors=%d", stats.JP.Count, len(out))
}
