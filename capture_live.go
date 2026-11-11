package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// LiveInput is the sole bridge into the existing Session/analyze engine.
// Unknown actors are excluded until the user explicitly selects them.
type LiveInput struct {
	OnRotate                       func(CombatLog, int64, string) error
	Rotations                      uint64
	LastRotation                   string
	Uses                           []SkillUse
	Detected                       *PartyConfig
	Party                          *PartyConfig
	Session                        *Session
	origin                         time.Time
	key                            [32]byte
	actors                         map[string]string
	targets                        map[string]string
	Events, Aggregated, LateEvents uint64
	region                         string
	idle                           int64
}

func newLiveInput(region string, now time.Time, idle int64) (*LiveInput, error) {
	if _, e := profileFor(region); e != nil {
		return nil, e
	}
	v := &LiveInput{origin: now, actors: map[string]string{}, targets: map[string]string{}, region: region, idle: idle}
	_, e := rand.Read(v.key[:])
	return v, e
}
func (v *LiveInput) alias(kind, raw string) string {
	h := hmac.New(sha256.New, v.key[:])
	h.Write([]byte(kind + ":" + raw))
	return kind + ":" + hex.EncodeToString(h.Sum(nil)[:8])
}
func (v *LiveInput) append(e RawDamage) error {
	if e.Damage < 0 || e.Damage > 1000000000 || e.Attacker == "" || e.Target == "" || e.Skill == "" {
		return fmt.Errorf("抽出イベントが集計仕様に合いません")
	}
	if err := v.rotateIfNeeded(e); err != nil {
		return err
	}
	actor, ok := v.actors[e.Attacker]
	if !ok {
		if len(v.actors) >= 64 {
			return fmt.Errorf("攻撃者上限64件に達しました")
		}
		actor = v.alias("actor", e.Attacker)
		v.actors[e.Attacker] = actor
		p := Player{ID: actor, Name: fmt.Sprintf("未確認の攻撃者%d", len(v.actors)), ClassID: "unknown", Party: false}
		if v.Session == nil {
			v.Session = &Session{Log: CombatLog{Version: 1, Region: v.region, Source: "capture-experimental", Players: []Player{p}, Events: []Hit{}}, Idle: v.idle, Speed: 1}
		} else {
			v.Session.Log.Players = append(v.Session.Log.Players, p)
		}
	}
	target := v.alias("target", e.Target)
	at := e.At.Sub(v.origin).Milliseconds()
	if at < 0 {
		at = 0
	}
	if n := len(v.Session.Log.Events); n > 0 && at < v.Session.Log.Events[n-1].At {
		at = v.Session.Log.Events[n-1].At
		v.LateEvents++
	}
	if at > maxTime {
		return fmt.Errorf("取得時間の上限に達しました")
	}
	v.Events++
	v.Session.Log.Events = append(v.Session.Log.Events, Hit{ID: fmt.Sprintf("capture:%d", v.Events), At: at, Type: "damage", PlayerID: actor, SkillID: e.Skill, TargetID: target, Damage: e.Damage, Critical: e.Critical})
	return nil
}
func (v *LiveInput) tick(now time.Time) error {
	if v.Session == nil {
		return nil
	}
	v.Session.Cursor = now.Sub(v.origin).Milliseconds()
	if v.Session.Cursor < 0 {
		v.Session.Cursor = 0
	}
	if v.Session.Cursor > maxTime {
		v.Session.Cursor = maxTime
	}
	if n := len(v.Session.Log.Events); n > 0 && v.Session.Cursor < v.Session.Log.Events[n-1].At {
		v.Session.Cursor = v.Session.Log.Events[n-1].At
	}
	if e := v.Session.recompute(); e != nil {
		return e
	}
	var count uint64
	for _, b := range v.Session.Result.Battles {
		for _, p := range b.Players {
			count += uint64(p.Hits)
		}
	}
	v.Aggregated = count
	return nil
}
func (v *LiveInput) selectActor(index int, party bool) error {
	if v.Session == nil || index < 0 || index >= len(v.Session.Log.Players) {
		return fmt.Errorf("攻撃者を選択してください")
	}
	v.Session.Log.Players[index].Party = party
	return v.Session.recompute()
}
