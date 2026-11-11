package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestJapaneseSkillDictionaryUsesKnownAliasesOnly(t *testing.T) {
	for _, tc := range [][2]string{{"IceMage_FreezingField", "フリージングフィールド"}, {"MeleeDefaultAttack_1", "通常攻撃（近接1段目）"}, {"MeleeDefaultAttack_2", "通常攻撃（近接2段目）"}, {"MeleeDefaultAttack_3", "通常攻撃（近接3段目）"}, {"Arbalist_MountingShock_Enhanced", "Arbalist_MountingShock_Enhanced"}, {"Unknown_NewSkill", "Unknown_NewSkill"}} {
		if got := japaneseSkillName(tc[0]); got != tc[1] {
			t.Fatal(tc, got)
		}
	}
}
func TestJapaneseSkillGroupDoesNotClassifyByToken(t *testing.T) {
	r := SkillDamageRow{Key: "group:772815F3", Name: "0x772815F3 スキル群", names: nil}
	if skillRowDisplayName(r) != r.Name {
		t.Fatal("token classified as normal attack")
	}
	r.names = []string{"MeleeDefaultAttack_1", "MeleeDefaultAttack_2"}
	before := r
	if got := skillRowDisplayName(r); got != "通常攻撃（近接・スキル群）" {
		t.Fatal(got)
	}
	if !reflect.DeepEqual(before, r) {
		t.Fatal("internal row changed")
	}
	r.names = append(r.names, "Unknown_NewSkill")
	if got := skillRowDisplayName(r); !strings.Contains(got, "Unknown_NewSkill") {
		t.Fatal(got)
	}
}

func TestBlankCSVTranslationOverridesLegacyJapanese(t *testing.T) {
	skillNamesOnce.Do(loadSkillDisplayNames)
	old := skillNamesJA
	defer func() { skillNamesJA = old }()
	skillNamesJA = skillNameDictionary{KnownAliases: map[string]string{"MeleeDefaultAttack_1": "通常攻撃（近接1段目）", "MeleeDefaultAttack_2": "通常攻撃（近接2段目）"}}
	aliases, _, err := parseSkillTranslationCSV([]byte("内部名,日本語訳\nMeleeDefaultAttack_1,\nMeleeDefaultAttack_2,\nIceMage_Example,氷の技\n"))
	if err != nil {
		t.Fatal(err)
	}
	for code, label := range aliases {
		skillNamesJA.KnownAliases[code] = label
	}
	if got := japaneseSkillName("MeleeDefaultAttack_1"); got != "MeleeDefaultAttack_1" {
		t.Fatal(got)
	}
	if got := japaneseSkillName("IceMage_Example"); got != "氷の技" {
		t.Fatal(got)
	}
	r := SkillDamageRow{Key: "group:example", names: []string{"MeleeDefaultAttack_1", "MeleeDefaultAttack_2"}}
	if got := skillRowDisplayName(r); !strings.Contains(got, "MeleeDefaultAttack_1 / MeleeDefaultAttack_2") {
		t.Fatal(got)
	}
}
