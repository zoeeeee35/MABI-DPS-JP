package main

import "testing"

func TestIdentifierTranslationOverridesExistingDisplayOnly(t *testing.T) {
	identifierNamesOnce.Do(loadIdentifierNames)
	old := identifierNames
	defer func() { identifierNames = old }()
	identifierNames = map[effectNameKey]string{{"*", 123}: "外部辞書の技"}
	h := Hit{Damage: 949, Link: &JPSkillLink{Paired: true, Effect: 123}}
	r := SkillDamageRow{Key: "skill:IceMage_FreezingField", Name: "IceMage_FreezingField", events: []Hit{h}}
	if got := skillRowDisplayName(r); got != "外部辞書の技" {
		t.Fatal(got)
	}
	if got := effectName(CombatLog{}, h, map[effectNameKey]string{{"*", 123}: "既存訳"}); got != "外部辞書の技" {
		t.Fatal(got)
	}
	if r.Name != "IceMage_FreezingField" || r.events[0].Damage != 949 {
		t.Fatal("source data changed")
	}
	r.events[0].Link = &JPSkillLink{Paired: true, Effect: 999}
	if got := skillRowDisplayName(r); got != japaneseSkillName(r.Name) {
		t.Fatal(got)
	}
}

func TestIdentifierRowNameRequiresConsistentPairedEvidence(t *testing.T) {
	names := map[effectNameKey]string{{"*", 123}: "氷の技", {"*", 124}: "別の技"}
	hit := func(id uint32, paired bool) Hit {
		return Hit{Damage: 949, Link: &JPSkillLink{Effect: id, Paired: paired}}
	}
	for _, tc := range []struct {
		hits []Hit
		want string
	}{
		{[]Hit{hit(123, true), hit(123, true)}, "氷の技"},
		{[]Hit{hit(123, true), hit(124, true)}, ""},
		{[]Hit{hit(123, false)}, ""},
		{[]Hit{hit(999, true)}, ""},
		{nil, ""},
	} {
		r := SkillDamageRow{events: tc.hits}
		if got := identifierRowName(r, names); got != tc.want {
			t.Fatalf("got %q want %q", got, tc.want)
		}
		for _, h := range r.events {
			if h.Damage != 949 {
				t.Fatal("damage mutated")
			}
		}
	}
}
