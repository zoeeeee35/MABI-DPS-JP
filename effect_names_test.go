package main

import (
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestEffectCSVValidationAndBlank(t *testing.T) {
	m, e := parseEffectNames([]byte("職業,効果ID,日本語訳\nIceMage,0x240EEC7C,氷結継続\nPriest,0x240EEC7C,\n"))
	if e != nil || len(m) != 1 {
		t.Fatal(m, e)
	}
	for _, s := range []string{"職業,効果ID,日本語訳\nIceMage,0x00000000,誤分類\n", "職業,効果ID,日本語訳\nUnknown,0x12,不明\n", "職業,効果ID,日本語訳\nPriest,0x12,一\nPriest,0x12,二\n"} {
		if m, e := parseEffectNames([]byte(s)); e == nil || m != nil {
			t.Fatal(m, e)
		}
	}
}
func TestUnknownEffectHierarchyTotalsAndScopedNames(t *testing.T) {
	log := CombatLog{SkillUses: []SkillUse{{ActorID: "self", Code: "IceMage_A"}}}
	hits := []Hit{{PlayerID: "self", Damage: 10, Link: &JPSkillLink{Paired: true, Effect: 1}}, {PlayerID: "self", Damage: 20, Link: &JPSkillLink{Paired: true, Effect: 2}}, {PlayerID: "self", Damage: 30, Link: &JPSkillLink{Paired: true, Effect: 0}}, {PlayerID: "self", Damage: 40, Link: &JPSkillLink{}}, {PlayerID: "self", Damage: 50, Link: &JPSkillLink{Paired: true, Effect: 3, Names: []string{"IceMage_A"}}}}
	before := fmt.Sprint(hits)
	rows := linkedSkillRows(&Battle{DamageEvents: hits}, log, "self")
	classified := classifyUnknownEffectRows(rows, log, map[effectNameKey]string{{"IceMage", 1}: "氷結継続", {"Priest", 2}: "他職業"})
	var sum int64
	var parent SkillDamageRow
	var translated bool
	for _, r := range classified {
		sum += r.Damage
		if r.Key == "unknown" {
			parent = r
		}
		if r.Name == "氷結継続" {
			translated = r.Damage == 10
		}
	}
	if sum != 150 || !translated || parent.Damage != 90 || len(parent.Children) != 3 {
		t.Fatal(classified)
	}
	var childTotal int64
	for _, c := range parent.Children {
		childTotal += c.Damage
	}
	if childTotal != parent.Damage {
		t.Fatal("child sum")
	}
	closed := expandUnknownEffectRows(classified, false)
	opened := expandUnknownEffectRows(classified, true)
	if len(opened) != len(closed)+3 {
		t.Fatal("expansion count")
	}
	if !reflect.DeepEqual(classified, expandUnknownEffectRows(classified, false)) || fmt.Sprint(hits) != before {
		t.Fatal("source mutated")
	}
}
func TestLocalEffectTranslationReplay(t *testing.T) {
	root := os.Getenv("MABI_EFFECT_ROOT")
	if root == "" {
		t.Skip("local")
	}
	seed, e := os.ReadFile(effectTranslationCSVName)
	if e != nil {
		t.Fatal(e)
	}
	names, e := parseEffectNames(seed)
	if e != nil {
		t.Fatal(e)
	}
	cases := []struct {
		file    string
		actor   uint64
		effect  uint32
		damage  int64
		unknown int
	}{
		{"ice-mage-effect.pcapng", 0x7a6857, 0x240eec7c, 669775, 271},
		{"priest-effect.pcapng", 0x7af2b9, 0x31bba586, 45978, 35},
	}
	for _, c := range cases {
		f, e := os.Open(root + "/" + c.file)
		if e != nil {
			t.Fatal(e)
		}
		stats, e := replayPCAP(f, 16000)
		f.Close()
		if e != nil {
			t.Fatal(e)
		}
		l := *stats.provisionalLog
		id := stats.replayAliases[c.actor]
		if id == "" {
			t.Fatal("actor missing")
		}
		b := &Battle{DamageEvents: l.Events}
		rows := linkedSkillRows(b, l, id)
		out := classifyUnknownEffectRows(rows, l, names)
		var a, z int64
		var found bool
		for _, r := range rows {
			a += r.Damage
		}
		for _, r := range out {
			z += r.Damage
			if r.Key == "unknown" && r.Hits != c.unknown {
				t.Fatal("unknown", r.Hits)
			}
			if r.Key == fmt.Sprintf("effect:%s:%08X", effectActorClass(l, id), c.effect) {
				found = r.Damage == c.damage
			}
		}
		if a != z || !found {
			t.Fatal(c.file, a, z, found)
		}
		t.Logf("%s translated=%d unchanged total=%d", c.file, c.damage, z)
	}
}
