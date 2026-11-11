//go:build native && windows

package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestLocalDungeonTransitionFiles(t *testing.T) {
	root := os.Getenv("MABI_DUNGEON_ROOT")
	if root == "" {
		t.Skip("local")
	}
	reports := []any{}
	var live *JPProvisionalInput
	oldSession, oldAuto, oldSelected, oldFloor := session, soloAutoEnabled, soloAutoSelected, soloIdentityFloor
	defer func() {
		session, soloAutoEnabled, soloAutoSelected, soloIdentityFloor = oldSession, oldAuto, oldSelected, oldFloor
	}()
	soloAutoEnabled = true
	soloIdentityFloor = 0
	selectedIDs := []string{}
	var eventNumber int
	for _, name := range []string{"before-transition.pcapng", "after-transition.pcapng"} {
		f, e := os.Open(root + "/" + name)
		if e != nil {
			t.Fatal(e)
		}
		actors := map[string]map[string]uint64{}
		conn := map[int]uint64{}
		var first, last time.Time
		stats, e := replayPCAPAudited(f, 16000, nil, func(c int, i uint64, at time.Time, id uint32, p []byte, out bool) {
			if out {
				return
			}
			if first.IsZero() {
				first = at
			}
			last = at
			conn[c]++
			if live == nil {
				live, e = newJPProvisionalInput(at, 5000)
				if e != nil {
					t.Fatal(e)
				}
			}
			if live.Live.Party == nil {
				live.Live.Party = newDetectedConfig()
				live.Live.Party.Scope = "self"
			}
			value := JPValue{At: at}
			if id == 0x5243 && len(p) == 53 {
				eventNumber++
				a, b := binary.LittleEndian.Uint64(p), binary.LittleEndian.Uint64(p[8:])
				value.ActorToken = fmt.Sprint(a)
				value.TargetToken = fmt.Sprint(b)
				value.ActorShortID = fmt.Sprintf("%X", a)
				value.TargetShortID = fmt.Sprintf("%X", b)
				value.EventToken = fmt.Sprintf("transition:%d", eventNumber)
				value.Damage64 = binary.LittleEndian.Uint64(p[16:])
			}
			if id == 0x1d4f8 {
				if skill, ok := decodeJPSkillUse(p); ok {
					value.ActorToken = fmt.Sprint(skill.Actor)
					value.ActorShortID = fmt.Sprintf("%X", skill.Actor)
					value.SkillCode = skill.Code
					value.ClassCode = skill.Class
				}
			}
			if value.SkillCode != "" || value.Damage64 > 0 {
				if _, err := live.append(value); err != nil {
					t.Fatal(err)
				}
			}
			if err := live.tick(at); err != nil {
				t.Fatal(err)
			}
			session = live.Live.Session
			if session != nil {
				soloAutomaticSelection()
			}
			if id == 0x5243 && len(p) == 53 {
				k := fmt.Sprintf("%X", binary.LittleEndian.Uint64(p))
				if actors[k] == nil {
					actors[k] = map[string]uint64{}
				}
				actors[k]["hits"]++
				actors[k]["damage"] += binary.LittleEndian.Uint64(p[16:])
			}
			if id == 0x1d4f8 {
				if s, ok := decodeJPSkillUse(p); ok {
					k := fmt.Sprintf("%X", s.Actor)
					if actors[k] == nil {
						actors[k] = map[string]uint64{}
					}
					actors[k][s.Code]++
				}
			}
		})
		f.Close()
		if e != nil {
			t.Fatal(e)
		}
		if who, ok := soloIdentity(); ok {
			selectedIDs = append(selectedIDs, who.DisplayID)
		} else {
			selectedIDs = append(selectedIDs, "")
		}
		reports = append(reports, map[string]any{"file": name, "first": first, "last": last, "frames": conn, "actors": actors, "gaps": stats.ReassemblyGaps, "drops": stats.ResourceDrops, "jp": stats.JP})
	}
	if len(selectedIDs) != 2 || selectedIDs[0] != "7B4FF0" || selectedIDs[1] != "88D575" {
		t.Fatalf("real capture self reselect: %v", selectedIDs)
	}
	t.Logf("real capture self selection: %v", selectedIDs)
	b, _ := json.MarshalIndent(reports, "", "  ")
	if e := os.WriteFile(os.Getenv("MABI_DUNGEON_REPORT"), b, 0600); e != nil {
		t.Fatal(e)
	}
}
