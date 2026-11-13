//go:build native && windows

package main

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

var soloMode = true
var soloShowAll bool

func soloLog() CombatLog {
	if historyIndex >= 0 && historyIndex < len(history) {
		return history[historyIndex].Archive.Request.Log
	}
	if session != nil {
		return session.Log
	}
	return CombatLog{}
}

var soloTimerEnd time.Time
var soloTimerMinutes = 2

func soloConfig() *PartyConfig {
	if historyIndex >= 0 && historyIndex < len(history) {
		return history[historyIndex].Archive.Request.Log.Party
	}
	if session != nil {
		return session.Log.Party
	}
	if captureInput != nil {
		return captureInput.Party
	}
	return nil
}
func soloIdentity() (PartyEntity, bool) {
	c := soloConfig()
	if c != nil {
		for _, e := range c.Entities {
			if e.Role == RoleSelf {
				return e, true
			}
		}
	}
	return PartyEntity{}, false
}
func soloCandidates() []PartyEntity {
	c := soloConfig()
	out := []PartyEntity{}
	if c != nil {
		for _, e := range c.Entities {
			hits, _, _ := soloEvidence(soloLog(), e.ID)
			if e.Role != RoleEnemy && e.Role != RoleSummon && (soloShowAll || hits > 0) {
				out = append(out, e)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		hi, _, ui := soloEvidence(soloLog(), out[i].ID)
		hj, _, uj := soloEvidence(soloLog(), out[j].ID)
		if (ui > 0) != (uj > 0) {
			return ui > 0
		}
		if hi != hj {
			return hi > hj
		}
		return entityDisplayID(out[i]) < entityDisplayID(out[j])
	})
	return out
}
func setSoloDisplayID(display string) error {
	c := soloConfig()
	if c == nil {
		return fmt.Errorf("候補を受信してから自分のIDを指定してください")
	}
	for id, e := range c.Entities {
		if strings.EqualFold(entityDisplayID(e), strings.TrimPrefix(strings.ToUpper(display), "0X")) {
			if soloMeasurement != nil && soloMeasurement.Party != nil {
				cancelSoloMeasurement()
			}
			if err := selectSelf(c, id); err != nil {
				return err
			}
			soloAutoSelected = false
			selectedPlayer = id
			if session != nil {
				_ = session.recompute()
			}
			lastRows = ""
			lastSkills = ""
			updateDisplay()
			return nil
		}
	}
	return fmt.Errorf("指定した自分のIDが記録にありません")
}
func soloScope() {
	if !soloMode || !nativeModern || session == nil {
		return
	}
	if session.Log.Party == nil {
		session.Log.Party = newDetectedConfig()
		session.Log.Party.discover(session.Log)
	}
	session.Log.Party.Scope = "self"
	_ = session.recompute()
}
func soloReset() {
	soloMeasurement = nil
	soloTimerEnd = time.Time{}
	if captureActive && jpInput != nil {
		old := jpInput.Live
		next, e := newJPProvisionalInput(time.Now(), idle)
		if e != nil {
			return
		}
		next.Live.key = old.key
		enableLiveRotation(next.Live)
		next.Live.Party = newDetectedConfig()
		next.Live.Party.Scope = "self"
		soloAutoEnabled = autoCapturePrefs.AutoSelf
		soloAutoSelected = false
		soloIdentityFloor = 0
		selectedPlayer = ""
		jpInput = next
		captureInput = next.Live
		session = nil
		historyIndex = -1
		jpUpdateAt = time.Time{}
		updateDisplay()
		return
	}
	onCommand(idReset, 0)
	soloScope()
}
func soloUsage() []SkillUsageRow {
	who, ok := soloIdentity()
	if !ok {
		return nil
	}
	if historyIndex >= 0 && historyIndex < len(history) {
		a := history[historyIndex].Archive
		return ownSkillUsage(a.Request.Log, who.ID, a.Request.Until)
	}
	if session != nil {
		return ownSkillUsage(session.Log, who.ID, session.Cursor)
	}
	return nil
}

var soloAutoEnabled = true
var soloAutoSelected bool

var soloIdentityFloor int64

func soloReidentify() {
	if soloMeasurement != nil {
		cancelSoloMeasurement()
	}
	soloAutoEnabled = true
	autoCapturePrefs.AutoSelf = true
	saveAutoCapturePrefs()
	soloAutoSelected = false
	selectedPlayer = ""
	historyIndex = -1
	displayCaptureTotal = false
	if session != nil {
		soloIdentityFloor = session.Cursor
		c := session.Log.Party
		if c != nil {
			for id, e := range c.Entities {
				if e.Role == RoleSelf {
					e.Role = RoleOutside
					e.Evidence = ""
					c.Entities[id] = e
				}
			}
			c.Scope = "self"
			c.StartAt = soloIdentityFloor
		}
		_ = session.recompute()
	}
	errorText = "再判定待ち：新しいスキル・ダメージ通知を受信してください。"
	updateDisplay()
}
func soloAutomaticSelection() {
	if !soloAutoEnabled || session == nil || session.Log.Party == nil {
		return
	}
	c := session.Log.Party
	current := ""
	for id, e := range c.Entities {
		if e.Role == RoleSelf {
			current = id
			break
		}
	}
	candidateLog := session.Log
	candidateLog.SkillUses = nil
	for _, u := range session.Log.SkillUses {
		e, ok := c.Entities[u.ActorID]
		if ok && e.ClassFamily != "" && e.Role != RoleEnemy && e.Role != RoleSummon {
			candidateLog.SkillUses = append(candidateLog.SkillUses, u)
		}
	}
	id, hits := recentSoloCandidate(candidateLog, session.Cursor, soloIdentityFloor)
	if id == "" || id == current {
		return
	}
	e, ok := c.Entities[id]
	if !ok || e.Role == RoleEnemy || e.Role == RoleSummon || e.ClassFamily == "" {
		return
	}
	stale := staleSelfCanSwitch(session.Log, current, id, session.Cursor, hits)
	if current != "" && !stale {
		return
	}

	if soloMeasurement != nil && soloMeasurement.Party != nil {
		if !stale || current == "" {
			return
		}
		cancelSoloMeasurement()
	}
	if selectSelf(c, id) == nil {
		e = c.Entities[id]
		e.Evidence = "auto-traffic-heuristic"
		c.Entities[id] = e
		soloAutoSelected = true
		soloAutoEnabled = true
		selectedPlayer = id
		if current != "" {
			first := session.Cursor
			for _, h := range session.Log.Events {
				if h.PlayerID == id && h.Damage > 0 && h.At >= session.Cursor-15000 && h.At < first {
					first = h.At
				}
			}
			c.StartAt = first
			historyIndex = -1
			displayCaptureTotal = false
			errorText = "旧IDの活動停止と新IDの活動を検出：自動で再判定しました（推定）。"
		}
		_ = session.recompute()
	}
}
