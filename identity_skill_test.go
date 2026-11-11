package main

import (
	"encoding/binary"
	"testing"
	"time"
	"unicode/utf16"
)

func skillFixture(actor uint64, name string) []byte {
	u := append(utf16.Encode([]rune(name)), 0)
	b := make([]byte, 12+len(u)*2)
	binary.LittleEndian.PutUint64(b, actor)
	binary.LittleEndian.PutUint32(b[8:], uint32(len(u)*2))
	for i, n := range u {
		binary.LittleEndian.PutUint16(b[12+i*2:], n)
	}
	return b
}
func TestJPSkillUsesEightByteIdentityAndBoundedUTF16(t *testing.T) {
	b := skillFixture(0x12345678aabbccdd, "IceMage_CrystalEdge_01")
	v, ok := decodeJPSkill(b)
	if !ok || v.Actor != 0x12345678aabbccdd || v.Class != "IceMage" {
		t.Fatal(v)
	}
	binary.LittleEndian.PutUint32(b[8:], 5000)
	if _, ok := decodeJPSkill(b); ok {
		t.Fatal("bad length accepted")
	}
	if _, ok := decodeJPSkill(skillFixture(1, "NPC_IceMage_Fake")); ok {
		t.Fatal("substring matched")
	}
}
func TestDetectedPlayersSameClassSeparateAndEnemyExcluded(t *testing.T) {
	now := time.Now()
	v, _ := newJPProvisionalInput(now, 5000)
	v.Live.Party = newDetectedConfig()
	for _, actor := range []string{"a", "b"} {
		for i := 0; i < 2; i++ {
			_, _ = v.append(JPValue{At: now, ActorToken: actor, SkillCode: "IceMage_CrystalEdge_01", ClassCode: "IceMage"})
		}
	}
	for i, p := range []struct {
		a, b string
		val  uint32
	}{{"a", "enemy", 100}, {"b", "enemy", 200}, {"enemy", "a", 900}, {"a", "b", 400}} {
		_, _ = v.append(JPValue{At: now, ActorToken: p.a, TargetToken: p.b, Value: p.val, EventToken: string(rune('1' + i))})
	}
	_ = v.tick(now.Add(time.Second))
	b := v.Live.Session.current()
	if b.Damage != 300 || len(b.Players) != 2 || b.Players[0].ID == b.Players[1].ID || b.Players[0].ClassID != "氷結術師" {
		t.Fatal(b)
	}
	e := v.Live.Party.Entities[b.Players[0].ID]
	e.Hidden = true
	v.Live.Party.Entities[e.ID] = e
	_ = v.tick(now.Add(time.Second))
	if v.Live.Session.current().Damage != 100 {
		t.Fatal("hide failed")
	}
}
func TestClassConflictAndManualFallback(t *testing.T) {
	now := time.Now()
	v, _ := newJPProvisionalInput(now, 5000)
	v.Live.Party = newDetectedConfig()
	for _, class := range []string{"IceMage", "IceMage", "SwordMaster"} {
		_, _ = v.append(JPValue{At: now, ActorToken: "a", SkillCode: class + "_Test", ClassCode: class})
	}
	e := v.Live.Party.Entities[v.Live.alias("actor", "a")]
	if e.Role != RoleUnknown || e.Evidence != "conflicting-skill-prefixes" {
		t.Fatal(e)
	}
	e.Role = RoleOutside
	e.Evidence = "manual"
	e.Class = "職業不明"
	_ = v.Live.Party.set(e)
	_, _ = v.append(provisionalValue(now, "hit", 100))
	_ = v.tick(now.Add(time.Second))
	if v.Live.Session.current() == nil {
		t.Fatal("manual unknown class excluded")
	}
}
func TestWarriorProgressionAndDamageBeforeClass(t *testing.T) {
	now := time.Now()
	v, _ := newJPProvisionalInput(now, 5000)
	v.Live.Party = newDetectedConfig()
	_, e := v.append(JPValue{At: now, ActorToken: "war", ActorShortID: "5447DD", TargetToken: "enemy", EventToken: "first", Value: 87229})
	if e != nil {
		t.Fatal(e)
	}
	_ = v.tick(now.Add(time.Second))
	if v.Live.Session.current() != nil {
		t.Fatal("unknown prematurely ranked")
	}
	for _, class := range []string{"NoviceWarrior", "ExpertWarrior"} {
		_, _ = v.append(JPValue{At: now.Add(time.Second), ActorToken: "war", ActorShortID: "5447DD", SkillCode: class + "_Attack", ClassCode: class})
	}
	_ = v.tick(now.Add(2 * time.Second))
	b := v.Live.Session.current()
	if b == nil || b.Damage != 87229 || b.Players[0].Name != "戦士 5447DD" || b.Players[0].ClassID != "戦士" {
		t.Fatal(b)
	}
	var original []byte = skillFixture(0x5447dd, "NoviceWarrior_Attack")
	if _, ok := decodeJPSkill(original); !ok {
		t.Fatal("novice unsupported")
	}
}
func TestCaptureSummaryPreservesSeparatePlayersAndIdle(t *testing.T) {
	b := []Battle{{Start: 1000, End: 2000, Damage: 100, Players: []Row{{Player: Player{ID: "a", Name: "a"}, Damage: 100, Hits: 1}}}, {Start: 10000, End: 11000, Damage: 300, Players: []Row{{Player: Player{ID: "b", Name: "b"}, Damage: 300, Hits: 1}}}}
	s := captureSummary(b)
	if s.Damage != 400 || s.Duration != 10 || len(s.Players) != 2 || s.Players[0].ID != "b" || s.Players[0].DPS != 30 || s.Players[1].Damage != 100 || b[0].Players[0].DPS != 0 {
		t.Fatal(s)
	}
	if captureSummary(nil) != nil {
		t.Fatal("empty summary")
	}
}

func TestSkillNotificationPreservesSelectedSelf(t *testing.T) {
	v, e := newJPProvisionalInput(time.Now(), 5000)
	if e != nil {
		t.Fatal(e)
	}
	v.Live.Party = newDetectedConfig()
	value := JPValue{At: time.Now(), ActorToken: "self", ActorShortID: "7B4FF0", SkillCode: "IceMage_CrystalEdge_01", ClassCode: "IceMage"}
	if e := v.observeSkill(value); e != nil {
		t.Fatal(e)
	}
	id := v.Live.alias("actor", "self")
	if e := selectSelf(v.Live.Party, id); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 4; i++ {
		if e := v.observeSkill(value); e != nil {
			t.Fatal(e)
		}
	}
	if v.Live.Party.Entities[id].Role != RoleSelf {
		t.Fatal("skill notification erased self selection")
	}
}
