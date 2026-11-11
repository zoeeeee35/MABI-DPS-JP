package main

import (
	"testing"
	"time"
)

func TestSelfSelectionKeepsPreviousDamageAndSkillUsesSeparate(t *testing.T) {
	now := time.Now()
	v, _ := newJPProvisionalInput(now, 5000)
	v.Live.Party = newDetectedConfig()
	for i, a := range []string{"me", "other"} {
		_, _ = v.append(JPValue{At: now, ActorToken: a, TargetToken: "enemy", EventToken: a, Value: uint32(100 + i*900), ActorShortID: a})
		_, _ = v.append(JPValue{At: now, ActorToken: a, SkillCode: "IceMage_One", ClassCode: "IceMage", ActorShortID: a})
		_, _ = v.append(JPValue{At: now.Add(time.Second), ActorToken: a, SkillCode: "MeleeDefaultAttack_1", ActorShortID: a})
	}
	id := v.Live.alias("actor", "me")
	if e := selectSelf(v.Live.Party, id); e != nil {
		t.Fatal(e)
	}
	_ = v.tick(now.Add(2 * time.Second))
	b := v.Live.Session.current()
	if b == nil || b.Damage != 100 || len(b.Players) != 1 || b.Players[0].ID != id {
		t.Fatal(b)
	}
	uses := ownSkillUsage(v.Live.Session.Log, id, 2000)
	if len(uses) != 2 || uses[0].Uses != 1 || b.DamageEvents[0].SkillID != "jp5243:unidentified" {
		t.Fatal("skill use was guessed as damage", uses)
	}
	if e := selectSelf(v.Live.Party, v.Live.alias("actor", "other")); e != nil {
		t.Fatal(e)
	}
	_ = v.tick(now.Add(2 * time.Second))
	if v.Live.Session.current().Damage != 1000 {
		t.Fatal("self switch failed")
	}
}
func TestHistoricalSoloViewDoesNotIncludeOthersOrMutateArchive(t *testing.T) {
	b := Battle{Damage: 1000, Players: []Row{{Player: Player{ID: "me"}, Damage: 100, Share: .1}, {Player: Player{ID: "other"}, Damage: 900, Share: .9}}, DamageEvents: []Hit{{PlayerID: "me", Damage: 100}, {PlayerID: "other", Damage: 900}}}
	view := soloBattleView(&b, "me")
	if view.Damage != 100 || len(view.Players) != 1 || len(view.DamageEvents) != 1 || view.Players[0].Share != 1 || b.Players[0].Share != .1 || b.Damage != 1000 {
		t.Fatal(view, b)
	}
	if soloBattleView(&b, "missing") != nil {
		t.Fatal("unselected archive shown")
	}
}
func TestSoloEvidenceUsesUnfilteredActorIdentity(t *testing.T) {
	log := CombatLog{Events: []Hit{{PlayerID: "self", Damage: 673}, {PlayerID: "other", Damage: 1486}, {PlayerID: "enemy", Damage: 919}}, SkillUses: []SkillUse{{ActorID: "self", Code: "IceMage_A"}, {ActorID: "other", Code: "FireMage_A"}}}
	h, d, u := soloEvidence(log, "self")
	if h != 1 || d != 673 || u != 1 {
		t.Fatal(h, d, u)
	}
	h, d, u = soloEvidence(log, "none")
	if h != 0 || d != 0 || u != 0 {
		t.Fatal(h, d, u)
	}
}
func TestAutomaticTrafficSelectionIsPerBattle(t *testing.T) {
	l := CombatLog{Events: []Hit{{At: 0, PlayerID: "a", Damage: 100}, {At: 1, PlayerID: "a", Damage: 100}, {At: 6000, PlayerID: "b", Damage: 25}}, SkillUses: []SkillUse{{ActorID: "a", At: 0}, {ActorID: "b", At: 6000}}}
	if soloAutoCandidate(l, 100, 5000) != "a" {
		t.Fatal("first battle")
	}
	if soloAutoCandidate(l, 6100, 5000) != "b" {
		t.Fatal("new battle")
	}
	if soloAutoCandidate(l, 12000, 5000) != "" {
		t.Fatal("idle selection")
	}
}
func TestSkillDamageRangeRequiresActualSkillIdentity(t *testing.T) {
	b := &Battle{DamageEvents: []Hit{{SkillID: "a", Damage: 10}, {SkillID: "a", Damage: 30}, {SkillID: "b", Damage: 999}}}
	hi, avg, lo, ok := soloSkillRange(b, "a")
	if !ok || hi != 30 || avg != 20 || lo != 10 {
		t.Fatal(hi, avg, lo)
	}
	if _, _, _, ok = soloSkillRange(b, "unknown"); ok {
		t.Fatal("fabricated skill")
	}
}
