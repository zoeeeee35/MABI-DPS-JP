package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPartySeparatesUnknownEnemyOutsiderSummonAndZero(t *testing.T) {
	now := time.Now()
	v, _ := newJPProvisionalInput(now, 5000)
	v.Live.Party = newPartyConfig()
	for i, p := range []struct {
		a, b string
		n    uint32
	}{{"self", "enemy", 100}, {"ally", "enemy", 200}, {"enemy", "self", 900}, {"outside", "enemy", 800}, {"unknown", "enemy", 700}, {"self", "ally", 600}, {"pet", "enemy", 50}, {"self", "self", 500}} {
		_, _ = v.append(JPValue{At: now.Add(time.Duration(i) * 100 * time.Millisecond), ActorToken: p.a, TargetToken: p.b, EventToken: string(rune('a' + i)), Value: p.n})
	}
	c := v.Live.Party
	for _, p := range []struct {
		id   string
		role EntityRole
	}{{"self", RoleSelf}, {"ally", RoleParty}, {"enemy", RoleEnemy}, {"outside", RoleOutside}, {"pet", RoleSummon}, {"zero", RoleParty}} {
		e := PartyEntity{ID: v.Live.alias("actor", p.id), Role: p.role, Name: p.id}
		if p.id == "pet" {
			e.Owner = v.Live.alias("actor", "self")
		}
		if err := c.set(e); err != nil {
			t.Fatal(err)
		}
	}
	if err := v.tick(now.Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	b := v.Live.Session.current()
	if b.Damage != 350 || len(b.Players) != 3 || b.Players[0].Name != "ally" || b.Players[2].Damage != 0 {
		t.Fatalf("%+v", b)
	}
	c.Scope = "self"
	_ = v.tick(now.Add(2 * time.Second))
	if v.Live.Session.current().Damage != 150 {
		t.Fatal(v.Live.Session.current())
	}
	c.Scope = "all"
	_ = v.tick(now.Add(2 * time.Second))
	if v.Live.Session.current().Damage != 1150 {
		t.Fatal(v.Live.Session.current())
	}
	data, _ := json.Marshal(v.Live.Session.Log)
	var log CombatLog
	if json.Unmarshal(data, &log) != nil || log.Party.Session != c.Session {
		t.Fatal("mapping lost")
	}
}
func TestPartyUnknownDefaultsExcludeAndOwnerRequired(t *testing.T) {
	now := time.Now()
	v, _ := newJPProvisionalInput(now, 5000)
	v.Live.Party = newPartyConfig()
	_, _ = v.append(provisionalValue(now, "a", 100))
	if e := v.tick(now.Add(time.Second)); e != nil {
		t.Fatal(e)
	}
	if v.Live.Session.current() != nil {
		t.Fatal("unknown counted as PT")
	}
	c := v.Live.Party
	_ = c.set(PartyEntity{ID: v.Live.alias("actor", "a"), Role: RoleSummon})
	_ = c.set(PartyEntity{ID: v.Live.alias("actor", "b"), Role: RoleEnemy})
	_ = v.tick(now.Add(time.Second))
	if v.Live.Session.current() != nil {
		t.Fatal("ownerless summon counted")
	}
}
