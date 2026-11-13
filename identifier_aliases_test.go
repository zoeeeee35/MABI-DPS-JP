package main

import "testing"

func TestIdentifierAliasesUseExactActorAndEffect(t *testing.T) {
	names := map[effectNameKey]string{{"*", 1}: "同じ名前", {"*", 2}: "別の名前"}
	log := CombatLog{SkillUses: []SkillUse{
		{ActorID: "self", Code: "IceMage_IceBarrier", HasTail: true, EffectCandidate: 1},
		{ActorID: "other", Code: "IceMage_IceBarrier", HasTail: true, EffectCandidate: 2},
		{ActorID: "self", Code: "IceMage_Unknown", HasTail: false, EffectCandidate: 1},
		{ActorID: "self", Code: "IceMage_Conflict", HasTail: true, EffectCandidate: 1},
		{ActorID: "self", Code: "IceMage_Conflict", HasTail: true, EffectCandidate: 2},
	}}
	a := identifierSkillAliases(log, "self", names)
	if a["IceMage_IceBarrier"] != "同じ名前" || a["IceMage_Unknown"] != "" || a["IceMage_Conflict"] != "" {
		t.Fatal(a)
	}
	if len(identifierSkillAliases(log, "", names)) != 0 {
		t.Fatal("mixed actors")
	}
	row := SkillDamageRow{Key: "skill:IceMage_IceBarrier", Name: "IceMage_IceBarrier", Uses: 1}
	if got := skillRowDisplayNameWithAliases(row, a); got != "同じ名前" {
		t.Fatal(got)
	}
	if row.Name != "IceMage_IceBarrier" || row.Damage != 0 || row.Uses != 1 {
		t.Fatal("source statistics changed")
	}
}
func TestIdentifierAliasesDoNotRenameUnknownDamage(t *testing.T) {
	a := map[string]string{"MeleeDefaultAttack_1": "通常攻撃", "MeleeDefaultAttack_2": "通常攻撃"}
	r := SkillDamageRow{Key: "group:effect", names: []string{"MeleeDefaultAttack_1", "MeleeDefaultAttack_2"}}
	if got := skillRowDisplayNameWithAliases(r, a); got != "通常攻撃" {
		t.Fatal(got)
	}
	r = SkillDamageRow{Key: "unknown", Name: "未分類", Damage: 123}
	if got := skillRowDisplayNameWithAliases(r, a); got != "未分類" {
		t.Fatal("unknown damage assigned", got)
	}
}
