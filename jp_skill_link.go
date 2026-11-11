package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type jpSkillKey struct {
	Actor  uint64
	Effect uint32
}

// Shared by the capture producer and UI consumer. Never expose its maps.
type jpSkillRegistry struct {
	mu      sync.RWMutex
	entries map[jpSkillKey]map[string]bool
}
type jpSkillLinkState struct {
	mu     sync.RWMutex
	paired bool
	effect uint32
}
type JPSkillLink struct {
	Paired   bool     `json:"paired"`
	Effect   uint32   `json:"effectCandidate"`
	Names    []string `json:"candidateSkills,omitempty"`
	Actor    uint64   `json:"-"`
	registry *jpSkillRegistry
	state    *jpSkillLinkState
}

// snapshot creates consumer-owned fields. The shared references are immutable;
// only the registry maps and late pairing state need their respective locks.
func (l *JPSkillLink) snapshot() *JPSkillLink {
	if l == nil {
		return nil
	}
	out := &JPSkillLink{Actor: l.Actor, registry: l.registry, state: l.state}
	if l.state != nil {
		l.state.mu.RLock()
		out.Paired, out.Effect = l.state.paired, l.state.effect
		l.state.mu.RUnlock()
	} else {
		out.Paired, out.Effect = l.Paired, l.Effect
	}
	if l.registry == nil {
		out.Names = append([]string(nil), l.Names...)
		return out
	}
	if !out.Paired {
		return out
	}
	l.registry.mu.RLock()
	for n := range l.registry.entries[jpSkillKey{out.Actor, out.Effect}] {
		out.Names = append(out.Names, n)
	}
	l.registry.mu.RUnlock()
	sort.Strings(out.Names)
	return out
}

// Only the decoder calls this on its private link. UI code replaces its own
// link with a snapshot rather than mutating a producer-owned pointer.
func (l *JPSkillLink) resolve() {
	if l == nil {
		return
	}
	s := l.snapshot()
	l.Paired, l.Effect, l.Names = s.Paired, s.Effect, s.Names
}
func (d *jp9Decoder) observeSkillLink(p []byte) {
	s, ok := decodeJPSkillUse(p)
	if !ok {
		return
	}
	n := int(binary.LittleEndian.Uint32(p[8:]))
	tail := p[12+n:]
	if len(tail) != 29 {
		return
	}
	if d.skillRegistry == nil {
		d.skillRegistry = &jpSkillRegistry{entries: map[jpSkillKey]map[string]bool{}}
	}
	key := jpSkillKey{s.Actor, binary.LittleEndian.Uint32(tail[25:29])}
	d.skillRegistry.mu.Lock()
	defer d.skillRegistry.mu.Unlock()
	names := d.skillRegistry.entries[key]
	if names == nil {
		if len(d.skillRegistry.entries) >= 4096 {
			return
		}
		names = map[string]bool{}
		d.skillRegistry.entries[key] = names
	}
	if len(names) < 128 {
		names[s.Code] = true
	}
}
func (d *jp9Decoder) joinSkillLink(id uint32, p []byte) {
	if d.last == nil || id != 0x4ff7 || len(p) != 35 {
		return
	}
	c := d.last
	if c.Actor != binary.LittleEndian.Uint64(p) || c.Target != binary.LittleEndian.Uint64(p[8:]) || !bytes.Equal(c.MatchField[:], p[24:32]) {
		return
	}
	c.Link.state.mu.Lock()
	c.Link.state.paired = true
	c.Link.state.effect = binary.LittleEndian.Uint32(p[16:20])
	c.Link.state.mu.Unlock()
	c.Link.resolve()
}
func applySkillLinks(log *CombatLog) {
	for i := range log.Events {
		h := &log.Events[i]
		if h.Link == nil {
			continue
		}
		h.Link = h.Link.snapshot()
		h.SkillID = "jp5243:unidentified"
		if !h.Link.Paired {
			continue
		}
		if len(h.Link.Names) == 1 {
			if len(h.Link.Names[0])+9 <= 128 {
				h.SkillID = "jp-skill:" + h.Link.Names[0]
			}
		} else if len(h.Link.Names) > 1 {
			hash := sha256.Sum256([]byte(strings.Join(h.Link.Names, "|")))
			h.SkillID = fmt.Sprintf("jp-group:%08X:%x", h.Link.Effect, hash[:6])
		}
	}
}

type SkillLinkStats struct{ Damage, Paired, Unique, Group, Unknown int }

func skillLinkStats(events []Hit) SkillLinkStats {
	s := SkillLinkStats{}
	for _, h := range events {
		if h.Damage <= 0 {
			continue
		}
		s.Damage++
		if h.Link == nil || !h.Link.Paired {
			s.Unknown++
			continue
		}
		s.Paired++
		if len(h.Link.Names) == 1 {
			s.Unique++
		} else if len(h.Link.Names) > 1 {
			s.Group++
		} else {
			s.Unknown++
		}
	}
	return s
}
