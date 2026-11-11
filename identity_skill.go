package main

import (
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

type jpSkillUse struct {
	Target      uint64
	Effect      uint32
	HasTail     bool
	Actor       uint64
	Code, Class string
}

func decodeJPSkill(body []byte) (jpSkillUse, bool) {
	s, ok := decodeJPSkillUse(body)
	return s, ok && s.Class != ""
}
func decodeJPSkillUse(body []byte) (jpSkillUse, bool) {
	if len(body) < 16 {
		return jpSkillUse{}, false
	}
	n := int(binary.LittleEndian.Uint32(body[8:12]))
	if n < 4 || n > 512 || n%2 != 0 || 12+n > len(body) {
		return jpSkillUse{}, false
	}
	u := make([]uint16, n/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(body[12+i*2:])
	}
	if u[len(u)-1] == 0 {
		u = u[:len(u)-1]
	}
	code := string(utf16.Decode(u))
	for _, ch := range code {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '_') {
			return jpSkillUse{}, false
		}
	}
	class := ""
	for prefix := range jpClassNames {
		if strings.HasPrefix(code, prefix+"_") {
			if class != "" {
				return jpSkillUse{}, false
			}
			class = prefix
		}
	}
	actor := binary.LittleEndian.Uint64(body)
	if actor == 0 {
		return jpSkillUse{}, false
	}
	s := jpSkillUse{Actor: actor, Code: code, Class: class}
	if len(body)-12-n == 29 {
		tail := body[12+n:]
		s.Target = binary.LittleEndian.Uint64(tail[17:25])
		s.Effect = binary.LittleEndian.Uint32(tail[25:29])
		s.HasTail = true
	}
	return s, true
}
func (v *JPProvisionalInput) observeSkill(e JPValue) error {
	id := v.Live.alias("actor", e.ActorToken)
	// Skill-only entities remain in the identity registry; they do not occupy damage-player slots.
	if v.Live.Session != nil {
		v.Live.Session.Log.Party = v.Live.Party
	}
	if len(v.Live.Uses) >= 20000 {
		v.Live.Uses = append([]SkillUse(nil), v.Live.Uses[len(v.Live.Uses)-10000:]...)
	}
	{
		v.Live.Uses = append(v.Live.Uses, SkillUse{At: e.At.Sub(v.Live.origin).Milliseconds(), ActorID: v.Live.alias("actor", e.ActorToken), Code: e.SkillCode, TargetCandidate: e.SkillTarget, EffectCandidate: e.SkillEffect, HasTail: e.SkillHasTail})
		if v.Live.Session != nil {
			v.Live.Session.Log.SkillUses = v.Live.Uses
		}
	}
	if e.ClassCode == "" || jpClassNames[e.ClassCode] == "" {
		return nil
	}

	family := e.ClassCode
	if family == "NoviceWarrior" {
		family = "ExpertWarrior"
	}
	label := jpClassNames[family]
	if label == "" {
		label = jpClassNames[e.ClassCode]
	}
	c := v.Live.Party
	if c == nil {
		if v.Live.Detected == nil {
			v.Live.Detected = newDetectedConfig()
		}
		c = v.Live.Detected
	}
	if v.Live.Session != nil {
		v.Live.Session.Log.Detected = c
	}
	id = v.Live.alias("actor", e.ActorToken)
	entity := c.Entities[id]
	entity.ID = id
	if e.ActorShortID != "" {
		entity.DisplayID = e.ActorShortID
	}
	if entity.Evidence == "manual" {
		if (entity.Role == RoleSelf || entity.Role == RoleParty || entity.Role == RoleOutside) && (entity.Class == "" || entity.Class == "職業不明") {
			entity.Class = label
			entity.ClassFamily = family
			entity.Observations++
			if entity.Name == "" {
				entity.Name = entity.Class + " " + entityDisplayID(entity)
			}
			c.Entities[id] = entity
		}
		return nil
	}
	if (entity.ClassFamily != "" && entity.ClassFamily != family) || (entity.ClassFamily == "" && entity.Class != "" && entity.Class != label) {
		entity.Evidence = "conflicting-skill-prefixes"
		entity.Role = RoleUnknown
		entity.Class = "職業不明"
		c.Entities[id] = entity
		return nil
	}
	if entity.Evidence == "conflicting-skill-prefixes" {
		return nil
	}
	entity.Class = label
	entity.ClassFamily = family
	entity.Observations++
	entity.Evidence = "skill-prefix-inference"
	if entity.Observations >= 2 && entity.Role != RoleSelf {
		entity.Role = RoleOutside
	}
	entity.Name = entity.Class + " " + entityDisplayID(entity)
	c.Entities[id] = entity
	return nil
}
