package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Local-only research: aggregate numeric resource candidates, never save packet bodies.
func TestLocalResourceAudit(t *testing.T) {
	root := os.Getenv("MABI_RESOURCE_ROOT")
	if root == "" {
		t.Skip("explicit local capture audit")
	}
	type summary struct {
		Count    int
		Entities map[uint64]bool
		Min, Max uint64
	}
	files := []string{}
	if err := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() && filepath.Ext(p) == ".pcapng" {
			files = append(files, p)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if extra := os.Getenv("MABI_RESOURCE_EXTRA"); extra != "" {
		files = append(files, extra)
	}
	sort.Strings(files)
	report := map[string]any{}
	for _, path := range files {
		counts := map[string]int{}
		updates := map[string]*summary{}
		f, e := os.Open(path)
		if e != nil {
			t.Fatal(e)
		}
		_, e = replayPCAPObserved(f, 16000, func(id uint32, p []byte, out bool) {
			direction := "in"
			if out {
				direction = "out"
			}
			if id == 0x1d595 || id == 0x1d596 || id == 0x4e4f {
				counts[fmt.Sprintf("0x%X/%s/len%d", id, direction, len(p))]++
			}
			if id == 0x1d595 && len(p) == 13 {
				counts[fmt.Sprintf("0x1D595/%s/raw8=%08X/raw12=%d", direction, binary.LittleEndian.Uint32(p[8:]), p[12])]++
			}
			if (id != 0x1d595 && id != 0x1d596) || len(p) < 20 || len(p) > 128 {
				return
			}
			kind := binary.LittleEndian.Uint32(p[8:])
			value := binary.LittleEndian.Uint64(p[12:])
			key := fmt.Sprintf("0x%X/%s/len%d/type%d", id, direction, len(p), kind)
			s := updates[key]
			if s == nil {
				s = &summary{Entities: map[uint64]bool{}, Min: value}
				updates[key] = s
			}
			s.Count++
			s.Entities[binary.LittleEndian.Uint64(p)] = true
			if value < s.Min {
				s.Min = value
			}
			if value > s.Max {
				s.Max = value
			}
		})
		f.Close()
		if e != nil {
			t.Fatal(path, e)
		}
		rows := map[string]any{}
		for k, s := range updates {
			rows[k] = map[string]any{"count": s.Count, "entities": len(s.Entities), "minRawValue": s.Min, "maxRawValue": s.Max, "interpretation": "unconfirmed except 0x1D596 len21 type8 current HP"}
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = filepath.Base(path)
		}
		report[rel] = map[string]any{"messageLengths": counts, "resourceCandidates": rows}
	}
	out := os.Getenv("MABI_RESOURCE_REPORT")
	if out != "" {
		b, _ := json.MarshalIndent(report, "", "  ")
		if e := os.WriteFile(out, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	t.Logf("audited %d capture files", len(files))
}
