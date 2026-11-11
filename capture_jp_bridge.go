package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const jpProvisionalSource = "capture-jp-provisional"

func jpToken(key [32]byte, kind string, data []byte) string {
	h := hmac.New(sha256.New, key[:])
	h.Write([]byte(kind + ":"))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil)[:16])
}

type JPRole string

const (
	JPUnknown JPRole = "unknown"
	JPPlayer  JPRole = "player"
	JPEnemy   JPRole = "enemy"
)

type JPProvisionalInput struct {
	Health    map[string]JPHealth
	Live      *LiveInput
	seenOrder []string
	Seen      map[string]bool
	Roles     map[string]JPRole
	Excluded  uint64
}

func newJPProvisionalInput(at time.Time, idle int64) (*JPProvisionalInput, error) {
	live, e := newLiveInput("jp", at, idle)
	if e != nil {
		return nil, e
	}
	return &JPProvisionalInput{Live: live, Seen: map[string]bool{}, Roles: map[string]JPRole{}}, nil
}
func (v *JPProvisionalInput) classify(e JPValue) string {
	if e.ActorToken == "" || e.TargetToken == "" || e.EventToken == "" {
		return "invalid_identity"
	}
	if e.ActorToken == e.TargetToken {
		return "self"
	}
	if v.Roles[e.ActorToken] == JPEnemy {
		return "incoming_or_enemy"
	}
	if v.Roles[e.ActorToken] == JPPlayer && v.Roles[e.TargetToken] == JPPlayer {
		return "friendly_or_heal"
	}
	if v.Roles[e.ActorToken] == JPPlayer && v.Roles[e.TargetToken] == JPEnemy {
		return "outgoing_provisional"
	}
	return "unclassified_provisional"
}
func (v *JPProvisionalInput) append(e JPValue) (bool, error) {
	if e.At.Before(v.Live.origin) {
		return false, nil
	}
	if e.HP != nil {
		if v.Health == nil {
			v.Health = map[string]JPHealth{}
		}
		copyHP := *e.HP
		copyHP.BattleStart = v.hpBattleBoundary(copyHP)
		v.Health[e.ActorShortID] = copyHP
		return false, nil
	}
	if e.Damage64 != 0 {
		if e.Damage64 > 1000000000 {
			v.Excluded++
			return false, nil
		}
		e.Value = uint32(e.Damage64)
	}
	if e.SkillCode != "" {
		return false, v.observeSkill(e)
	}
	class := v.classify(e)
	if class != "outgoing_provisional" && class != "unclassified_provisional" {
		v.Excluded++
		return false, nil
	}
	if e.Value == 0 || e.Value > 1000000000 {
		v.Excluded++
		return false, nil
	}
	if v.Seen[e.EventToken] {
		return false, nil
	}

	if err := v.Live.append(RawDamage{Attacker: e.ActorToken, Target: e.TargetToken, Skill: "jp5243:unidentified", Damage: int64(e.Value), At: e.At}); err != nil {
		return false, err
	}
	v.rememberEvent(e.EventToken)
	for id, hp := range v.Health {
		hp.BattleStart = v.hpBattleBoundary(hp)
		v.Health[id] = hp
	}
	s := v.Live.Session
	s.Log.Source = jpProvisionalSource
	s.Log.Events[len(s.Log.Events)-1].Flags = e.Flags
	s.Log.Events[len(s.Log.Events)-1].Link = e.Link.snapshot()
	s.Log.SkillUses = v.Live.Uses
	s.Log.Detected = v.Live.Detected
	if v.Live.Party != nil {
		s.Log.Party = v.Live.Party
	}
	s.Log.Events[len(s.Log.Events)-1].TargetID = v.Live.alias("actor", e.TargetToken)
	latest := s.Log
	latest.Events = latest.Events[len(latest.Events)-1:]
	for _, config := range []*PartyConfig{s.Log.Party, s.Log.Detected} {
		if config != nil {
			config.discover(latest)
			id := v.Live.alias("actor", e.ActorToken)
			entity := config.Entities[id]
			if e.ActorShortID != "" {
				entity.DisplayID = e.ActorShortID
			}
			config.Entities[id] = entity
			id = v.Live.alias("actor", e.TargetToken)
			entity = config.Entities[id]
			if e.TargetShortID != "" {
				entity.DisplayID = e.TargetShortID
			}
			config.Entities[id] = entity
		}
	}
	// This roster means included provisional identifiers, not verified party members.
	for i := range s.Log.Players {
		if s.Log.Players[i].Name == fmt.Sprintf("未確認の攻撃者%d", i+1) {
			s.Log.Players[i].Name = fmt.Sprintf("識別子%d（）", i+1)
			s.Log.Players[i].Party = true
		}
	}
	return true, nil
}
func (v *JPProvisionalInput) tick(at time.Time) error { return v.Live.tick(at) }

func (v *JPProvisionalInput) hpBattleBoundary(hp JPHealth) HPStartEvidence {
	if v.Live.Session != nil {
		battles := v.Live.Session.Result.Battles
		if len(battles) > 0 {
			return hp.boundary(v.Live.origin.Add(time.Duration(battles[len(battles)-1].Start) * time.Millisecond))
		}
	}
	return unknownHPStart(time.Time{}, "戦闘開始イベント未観測")
}
