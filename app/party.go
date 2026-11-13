package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

type EntityRole string

const (
	RoleUnknown EntityRole = "unknown"
	RoleSelf    EntityRole = "self"
	RoleParty   EntityRole = "party"
	RoleOutside EntityRole = "outside"
	RoleEnemy   EntityRole = "enemy_npc"
	RoleSummon  EntityRole = "summon"
)

type PartyEntity struct {
	ClassFamily  string     `json:"classFamily,omitempty"`
	DisplayID    string     `json:"displayId,omitempty"`
	Evidence     string     `json:"evidence,omitempty"`
	Observations int        `json:"classObservations,omitempty"`
	Hidden       bool       `json:"hidden,omitempty"`
	ID           string     `json:"id"`
	Name         string     `json:"name,omitempty"`
	Class        string     `json:"class,omitempty"`
	Role         EntityRole `json:"role"`
	Owner        string     `json:"owner,omitempty"`
}
type PartyConfig struct {
	StartAt             int64                  `json:"scopeStartMs,omitempty"`
	AllowUnknownTargets bool                   `json:"allowUnknownTargets,omitempty"`
	Session             string                 `json:"session"`
	Scope               string                 `json:"scope"`
	Entities            map[string]PartyEntity `json:"entities"`
}

func newPartyConfig() *PartyConfig {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return &PartyConfig{Session: hex.EncodeToString(b[:]), Scope: "party", Entities: map[string]PartyEntity{}}
}
func (c *PartyConfig) set(e PartyEntity) error {
	if c.Entities == nil {
		c.Entities = map[string]PartyEntity{}
	}
	if e.ID == "" || len(e.Name) > 128 || len(e.Class) > 128 {
		return fmt.Errorf("ID・名前・職業を確認してください")
	}
	switch e.Role {
	case RoleUnknown, RoleSelf, RoleParty, RoleOutside, RoleEnemy, RoleSummon:
	default:
		return fmt.Errorf("分類が不正です")
	}
	if e.Role == RoleSelf {
		for id, p := range c.Entities {
			if id != e.ID && p.Role == RoleSelf {
				return fmt.Errorf("自分は1人だけ指定してください")
			}
		}
	}
	if e.Owner == e.ID {
		return fmt.Errorf("所有者に自分自身は指定できません")
	}
	c.Entities[e.ID] = e
	return nil
}
func (c *PartyConfig) selected(e PartyEntity) bool {
	if e.Hidden {
		return false
	}
	switch c.Scope {
	case "self":
		return e.Role == RoleSelf
	case "members":
		return e.Role == RoleParty
	case "all":
		return e.Role == RoleSelf || e.Role == RoleParty || e.Role == RoleOutside
	default:
		return e.Role == RoleSelf || e.Role == RoleParty
	}
}
func (c *PartyConfig) discover(l CombatLog) {
	if c.Entities == nil {
		c.Entities = map[string]PartyEntity{}
	}
	for _, p := range l.Players {
		if _, ok := c.Entities[p.ID]; !ok {
			c.Entities[p.ID] = PartyEntity{ID: p.ID, Role: RoleUnknown}
		}
	}
	for _, h := range l.Events {
		if h.TargetID != "" {
			if _, ok := c.Entities[h.TargetID]; !ok {
				c.Entities[h.TargetID] = PartyEntity{ID: h.TargetID, Role: RoleUnknown}
			}
		}
	}
}
func (c *PartyConfig) filtered(l CombatLog) CombatLog {
	c.discover(l)
	l.Players = []Player{}
	out := []Hit{}
	ids := []string{}
	for id := range c.Entities {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		e := c.Entities[id]
		if !c.selected(e) {
			continue
		}
		name := strings.TrimSpace(e.Name)
		if name == "" {
			name = "ID " + entityDisplayID(e)
		}
		class := strings.TrimSpace(e.Class)
		if class == "" {
			class = "職業不明"
		}
		l.Players = append(l.Players, Player{ID: id, Name: name, ClassID: class, Party: true})
	}
	for _, h := range l.Events {
		if h.At < c.StartAt {
			continue
		}
		if h.Type == "end" {
			out = append(out, h)
			continue
		}
		a := c.Entities[h.PlayerID]
		t := c.Entities[h.TargetID]
		if a.Role == RoleSummon {
			a = c.Entities[a.Owner]
		}
		if !c.selected(a) || (t.Role != RoleEnemy && !(c.AllowUnknownTargets && t.Role == RoleUnknown)) || h.PlayerID == h.TargetID {
			continue
		}
		h.PlayerID = a.ID
		out = append(out, h)
	}
	if len(l.Players) == 0 {
		l.Players = []Player{{ID: "pt:empty", Name: "PT未設定", ClassID: "職業不明"}}
	}
	l.Events = out
	l.Party = nil
	return l
}

func newDetectedConfig() *PartyConfig {
	c := newPartyConfig()
	c.Scope = "all"
	c.AllowUnknownTargets = true
	return c
}

func entityDisplayID(e PartyEntity) string {
	if e.DisplayID != "" {
		return e.DisplayID
	}
	id := e.ID
	return strings.ToUpper(id[max(0, len(id)-6):])
}
