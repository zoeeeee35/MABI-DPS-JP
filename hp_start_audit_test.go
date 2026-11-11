package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestHPStartCaptureAudit(t *testing.T) {
	path := os.Getenv("MABI_HP_START")
	if path == "" {
		t.Skip("local capture")
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	type row struct {
		Stream         int
		Index          uint64
		At             time.Time
		ID             string
		Entity, Target string
		Resource       uint32
		Value          uint64
		Length         int
	}
	rows := []row{}
	counts := map[string]int{}
	skills := map[string]map[string]int{}
	trackers := map[int]*jpHealthTracker{}
	stats, e := replayPCAPAudited(f, 16000, nil, func(stream int, index uint64, at time.Time, id uint32, p []byte, out bool) {
		key := fmt.Sprintf("%X/%t", id, out)
		counts[key]++
		if id == 0x1d4f8 {
			if skill, ok := decodeJPSkillUse(p); ok {
				key := fmt.Sprintf("%X/out=%t", skill.Actor, out)
				if skills[key] == nil {
					skills[key] = map[string]int{}
				}
				skills[key][skill.Code]++
			}
		}
		if out {
			return
		}
		if trackers[stream] == nil {
			trackers[stream] = &jpHealthTracker{Stream: stream}
		}
		trackers[stream].Frame = index
		trackers[stream].observe(id, p, at)
		r := row{Stream: stream, Index: index, At: at, ID: fmt.Sprintf("%X", id), Length: len(p)}
		if len(p) >= 8 {
			r.Entity = fmt.Sprintf("%X", binary.LittleEndian.Uint64(p))
		}
		if id == 0x1d596 && len(p) == 21 {
			r.Resource = binary.LittleEndian.Uint32(p[8:])
			r.Value = binary.LittleEndian.Uint64(p[12:])
			rows = append(rows, r)
		}
		if id==0x4e4f && len(p)>=75 { r.Value=binary.LittleEndian.Uint64(p[67:]); rows=append(rows,r) }
		if id == 0x5243 && len(p) == 53 {
			r.Target = fmt.Sprintf("%X", binary.LittleEndian.Uint64(p[8:]))
			r.Value = binary.LittleEndian.Uint64(p[16:])
			rows = append(rows, r)
		}
	})
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.MarshalIndent(map[string]any{"stats": stats, "counts": counts, "rows": rows, "skills": skills}, "", "  ")
	if p := os.Getenv("MABI_HP_START_REPORT"); p != "" {
		if e = os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	if path := os.Getenv("MABI_HP_CSV"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		values := map[string]JPHealth{}
		for stream, h := range trackers {
			for id, v := range h.Values {
				values[fmt.Sprintf("stream%d/%X", stream, id)] = v
			}
		}
		if err = exportHPHistory(file, values); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("HP/damage rows=%d frames=%d gaps=%d", len(rows), stats.Frames, stats.ReassemblyGaps)
}
