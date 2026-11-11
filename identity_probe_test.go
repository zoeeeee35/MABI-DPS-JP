package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalIdentityResearch(t *testing.T) {
	root := os.Getenv("MABI_LOCAL_RESEARCH")
	if root == "" {
		t.Skip("local captures opt in")
	}
	out := map[string]any{}
	paths, _ := filepath.Glob(filepath.Join(root, "*", "captures", "*", "*.pcapng"))
	for _, path := range paths {
		f, e := os.Open(path)
		if e != nil {
			t.Fatal(e)
		}
		s, e := replayPCAP(f, 16000)
		f.Close()
		if e != nil {
			t.Fatal(e)
		}
		rel, _ := filepath.Rel(root, path)
		entry := map[string]any{"identity": s.Identity}
		if s.provisionalLog != nil && s.provisionalLog.Detected != nil {
			log := *s.provisionalLog
			log.Party = log.Detected
			result, err := analyze(Request{Log: log, Until: s.Provisional.Until, Idle: 5000})
			if err != nil {
				t.Fatal(err)
			}
			entry["detectedPlayers"] = result
		}
		out[rel] = entry
	}
	data, _ := json.MarshalIndent(out, "", "  ")
	if e := os.WriteFile(filepath.Join(root, "identity-research.json"), data, 0600); e != nil {
		t.Fatal(e)
	}
}
