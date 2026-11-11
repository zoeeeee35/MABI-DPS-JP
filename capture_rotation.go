package main

import (
	"fmt"
)

// Capacity affects an analysis segment, never the lifetime of the Npcap reader.
// Archive must succeed before discarding any events.
func (v *LiveInput) rotateIfNeeded(e RawDamage) error {
	if v.Session == nil {
		return nil
	}
	_, known := v.actors[e.Attacker]
	reason := ""
	if len(v.Session.Log.Events) >= 20000 {
		reason = "event-capacity"
	} else if !known && len(v.actors) >= 64 {
		reason = "actor-capacity"
	}
	if reason == "" {
		return nil
	}
	if v.OnRotate == nil {
		return fmt.Errorf("集計区間の保存先がありません（%s）", reason)
	}
	end := v.Session.endTime()
	if err := v.OnRotate(v.Session.Log, end, reason); err != nil {
		return fmt.Errorf("自動区間保存に失敗：%w", err)
	}
	v.Rotations++
	v.LastRotation = reason
	v.Session = nil
	v.actors = map[string]string{}
	// Keep recent skill evidence for delayed / ongoing effects across a boundary.
	if len(v.Uses) > 4096 {
		v.Uses = append([]SkillUse(nil), v.Uses[len(v.Uses)-4096:]...)
	}
	return nil
}
func (v *JPProvisionalInput) rememberEvent(token string) {
	v.Seen[token] = true
	v.seenOrder = append(v.seenOrder, token)
	if len(v.seenOrder) > 4096 {
		delete(v.Seen, v.seenOrder[0])
		v.seenOrder = v.seenOrder[1:]
	}
}
