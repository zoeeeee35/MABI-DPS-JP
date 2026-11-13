package main

import (
	"os"
	"testing"
)

func TestLocalDictionaryUnknownReclassification(t *testing.T) {
	path := os.Getenv("MABI_NAME_DICTIONARY")
	if path == "" {
		t.Skip("local dictionary not specified")
	}
	data, err := readBoundedFile(path, 4<<20)
	if err != nil {
		t.Fatal(err)
	}
	names, err := parseEffectNames(data)
	if err != nil {
		t.Fatal(err)
	}
	identifierNamesOnce.Do(loadIdentifierNames)
	old := identifierNames
	identifierNames = names
	defer func() { identifierNames = old }()
	count := 0
	for key, want := range names {
		if key.Class != "*" {
			continue
		}
		hit := Hit{PlayerID: "self", Damage: 123, Link: &JPSkillLink{Paired: true, Effect: key.ID}}
		rows := classifyUnknownEffectRows([]SkillDamageRow{{Key: "unknown", Name: "未分類", Damage: 123, Hits: 1, events: []Hit{hit}}}, CombatLog{}, names)
		if len(rows) != 1 || rows[0].Name != want || rows[0].Key == "unknown" || rows[0].Damage != 123 {
			t.Fatalf("ID %08X classification failed", key.ID)
		}
		count++
	}
	for id, want := range map[uint32]string{0x240EEC7C: "アイシクルマーク", 0x5BC5DB35: "身に染みる冷気"} {
		if names[effectNameKey{"*", id}] != want {
			t.Fatalf("missing %08X", id)
		}
	}
	t.Logf("verified %d dictionary IDs, damage unchanged", count)
}
