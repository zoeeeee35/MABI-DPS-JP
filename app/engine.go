package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
)

const maxTime = int64(7 * 24 * 60 * 60 * 1000)

type Player struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	ClassID string `json:"classId"`
	Party   bool   `json:"party"`
}
type Hit struct {
	Link     *JPSkillLink `json:"skillLink,omitempty"`
	Flags    *DamageFlags `json:"flagCandidates,omitempty"`
	ID       string       `json:"id"`
	At       int64        `json:"atMs"`
	Type     string       `json:"type"`
	PlayerID string       `json:"playerId,omitempty"`
	SkillID  string       `json:"skillId,omitempty"`
	TargetID string       `json:"targetId,omitempty"`
	Damage   int64        `json:"damage,omitempty"`
	Critical bool         `json:"critical,omitempty"`
}
type SkillUse struct {
	TargetCandidate string `json:"targetCandidate,omitempty"`
	EffectCandidate uint32 `json:"effectCandidate,omitempty"`
	HasTail         bool   `json:"hasTail,omitempty"`
	At              int64  `json:"atMs"`
	ActorID         string `json:"actorId"`
	Code            string `json:"code"`
}
type CombatLog struct {
	SkillUses []SkillUse   `json:"skillUses,omitempty"`
	Detected  *PartyConfig `json:"detectedEntities,omitempty"`
	Party     *PartyConfig `json:"partyConfig,omitempty"`
	Version   int          `json:"version"`
	Region    string       `json:"region"`
	Source    string       `json:"source"`
	Players   []Player     `json:"players"`
	Events    []Hit        `json:"events"`
}
type Request struct {
	Window *MeasurementWindow `json:"measurementWindow,omitempty"`
	Log    CombatLog          `json:"log"`
	Until  int64              `json:"untilMs"`
	Idle   int64              `json:"idleMs"`
}
type Skill struct {
	ID     string  `json:"id"`
	Damage int64   `json:"damage"`
	Hits   int     `json:"hits"`
	Crits  int     `json:"crits"`
	DPS    float64 `json:"dps"`
}
type Row struct {
	DamageStatus string `json:"damageStatus,omitempty"`
	Player
	Damage   int64   `json:"damage"`
	Hits     int     `json:"hits"`
	Crits    int     `json:"crits"`
	DPS      float64 `json:"dps"`
	Share    float64 `json:"share"`
	CritRate float64 `json:"critRate"`
	Skills   []Skill `json:"skills"`
}
type Battle struct {
	Number       int     `json:"number"`
	Start        int64   `json:"startMs"`
	End          int64   `json:"endMs"`
	Duration     float64 `json:"durationSec"`
	Active       bool    `json:"active"`
	Reason       string  `json:"reason"`
	Damage       int64   `json:"damage"`
	Players      []Row   `json:"players"`
	DamageEvents []Hit   `json:"damageEvents"`
}
type Result struct {
	Version      int      `json:"version"`
	Region       string   `json:"region"`
	Source       string   `json:"source"`
	Until        int64    `json:"untilMs"`
	Idle         int64    `json:"idleMs"`
	Battles      []Battle `json:"battles"`
	LiveVerified bool     `json:"liveVerified"`
}

func decode(r io.Reader, out any) error {
	d := json.NewDecoder(r)
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return fmt.Errorf("JSONの形式が正しくありません: %w", err)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("JSONは1件だけ指定してください")
	}
	return nil
}
func validID(s string) bool { return len(s) > 0 && len(s) <= 128 }
func validate(q Request) error {
	l := q.Log
	if l.Version != 1 || (l.Region != "jp" && l.Region != "tw" && l.Region != "kr") || (l.Source != "synthetic" && l.Source != "normalized-offline" && l.Source != "capture-experimental" && l.Source != jpProvisionalSource) {
		return fmt.Errorf("対応していないバージョン・地域・データ種別です")
	}
	if q.Until < 0 || q.Until > maxTime || q.Idle < 1000 || q.Idle > 60000 {
		return fmt.Errorf("再生時刻または無操作時間の範囲が正しくありません")
	}
	if len(l.Players) == 0 || len(l.Players) > 64 || len(l.Events) > 20000 {
		return fmt.Errorf("プレイヤーは1〜64人、イベントは20000件以下にしてください")
	}
	players := map[string]bool{}
	ids := map[string]bool{}
	last := int64(-1)
	for _, p := range l.Players {
		if !validID(p.ID) || len(p.Name) == 0 || len(p.Name) > 128 || len(p.ClassID) > 128 || players[p.ID] {
			return fmt.Errorf("プレイヤーのID・表示名が不正または重複しています")
		}
		players[p.ID] = true
	}
	for _, e := range l.Events {
		if !validID(e.ID) || ids[e.ID] || e.At < last || e.At < 0 || e.At > maxTime {
			return fmt.Errorf("イベントIDが重複しているか、時刻が昇順ではありません")
		}
		ids[e.ID] = true
		last = e.At
		if e.Type == "damage" {
			if !players[e.PlayerID] || !validID(e.SkillID) || !validID(e.TargetID) || e.Damage < 0 || e.Damage > 1000000000 {
				return fmt.Errorf("ダメージイベントの値・参照先が正しくありません")
			}
		} else if e.Type == "end" {
			if e.PlayerID != "" || e.SkillID != "" || e.TargetID != "" || e.Damage != 0 || e.Critical {
				return fmt.Errorf("終了イベントにダメージ情報を含められません")
			}
		} else {
			return fmt.Errorf("未対応のイベント種別です")
		}
	}
	return nil
}
func analyze(q Request) (Result, error) {
	if q.Window != nil {
		return analyzeMeasurement(q)
	}
	partyMode := q.Log.Party != nil
	if partyMode {
		q.Log = q.Log.Party.filtered(q.Log)
	}
	r := Result{Version: 1, Region: q.Log.Region, Source: q.Log.Source, Until: q.Until, Idle: q.Idle, Battles: []Battle{}}
	if err := validate(q); err != nil {
		return r, err
	}
	roster := map[string]Player{}
	for _, p := range q.Log.Players {
		roster[p.ID] = p
	}
	var b *Battle
	var last int64
	finish := func(end int64, reason string) {
		if b != nil {
			b.End = end
			b.Active = false
			b.Reason = reason
			r.Battles = append(r.Battles, *b)
			b = nil
		}
	}
	for _, e := range q.Log.Events {
		if e.At > q.Until {
			break
		}
		if b != nil && e.At-last >= q.Idle {
			finish(last, "idle")
		}
		if e.Type == "end" {
			finish(e.At, "explicit")
			continue
		}
		if !roster[e.PlayerID].Party {
			continue
		}
		if b == nil {
			b = &Battle{Number: len(r.Battles) + 1, Start: e.At, Active: true, Reason: "active", Players: []Row{}, DamageEvents: []Hit{}}
		}
		b.DamageEvents = append(b.DamageEvents, e)
		b.Damage += e.Damage
		last = e.At
	}
	if b != nil {
		if q.Until-last >= q.Idle {
			finish(last, "idle")
		} else {
			// Keep the battle active during the grace period without charging it to DPS.
			b.End = last
			r.Battles = append(r.Battles, *b)
		}
	}
	for i := range r.Battles {
		b := &r.Battles[i]
		b.Duration = float64(b.End-b.Start) / 1000
		denominator := math.Max(1, b.Duration)
		rows := map[string]*Row{}
		skills := map[string]map[string]*Skill{}
		for _, e := range b.DamageEvents {
			row := rows[e.PlayerID]
			if row == nil {
				row = &Row{Player: roster[e.PlayerID], Skills: []Skill{}}
				rows[e.PlayerID] = row
				skills[e.PlayerID] = map[string]*Skill{}
			}
			row.Damage += e.Damage
			row.Hits++
			if e.Critical {
				row.Crits++
			}
			s := skills[e.PlayerID][e.SkillID]
			if s == nil {
				s = &Skill{ID: e.SkillID}
				skills[e.PlayerID][e.SkillID] = s
			}
			s.Damage += e.Damage
			s.Hits++
			if e.Critical {
				s.Crits++
			}
		}
		if partyMode {
			for id, p := range roster {
				if p.Party && rows[id] == nil {
					rows[id] = &Row{Player: p, Skills: []Skill{}}
					skills[id] = map[string]*Skill{}
				}
			}
		}
		for id, row := range rows {
			if q.Log.Source == jpProvisionalSource {
				row.DamageStatus = "candidate"
				if row.Hits == 0 {
					row.DamageStatus = "unobserved"
				}
			}
			row.DPS = float64(row.Damage) / denominator
			if b.Damage > 0 {
				row.Share = float64(row.Damage) / float64(b.Damage)
			}
			row.CritRate = float64(row.Crits) / math.Max(1, float64(row.Hits))
			for _, s := range skills[id] {
				s.DPS = float64(s.Damage) / denominator
				row.Skills = append(row.Skills, *s)
			}
			sort.Slice(row.Skills, func(i, j int) bool {
				a, c := row.Skills[i], row.Skills[j]
				if a.Damage == c.Damage {
					return a.ID < c.ID
				}
				return a.Damage > c.Damage
			})
			b.Players = append(b.Players, *row)
		}
		sort.Slice(b.Players, func(i, j int) bool {
			a, c := b.Players[i], b.Players[j]
			if a.Damage == c.Damage {
				return a.ID < c.ID
			}
			return a.Damage > c.Damage
		})
	}
	return r, nil
}
