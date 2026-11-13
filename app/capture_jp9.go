package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/andybalholm/brotli"
	"io"
	"time"
)

// JP9 is an observed envelope, not a confirmed combat schema. Unknown tag/extra
// fields are never interpreted as compression, skill or critical indicators.
type jp9Counts struct{ Frames, Candidates, Sum, Paired, Invalid, Short5243, HPUpdates, HPMatches, BrotliOK, BrotliFailed, Recovered, Discarded uint64 }
type jp9Candidate struct {
	Link          *JPSkillLink
	MatchField    [8]byte
	Flags         *DamageFlags
	Actor, Target uint64
	Damage64      uint64
	Value         uint32
	At            time.Time
}
type jp9Decoder struct {
	skillRegistry *jpSkillRegistry
	health        jpHealthTracker
	healthQueue   []JPHealth
	audit         func(uint64, time.Time, uint32, []byte, bool)
	observer      func(uint32, []byte, bool)
	buffer        []byte
	counts        jp9Counts
	stopped       bool
	last          *jp9Candidate
	skillActors   map[uint64]bool
	skillQueue    []jpSkillUse
	skillBodies   [][]byte
	entities      map[uint64]bool
	skillMatches  uint64
}

func (d *jp9Decoder) feed(b []byte, outbound bool, at time.Time) []jp9Candidate {
	if d.stopped && outbound {
		return nil
	}
	if len(d.buffer)+len(b) > maxWireBuffer {
		d.counts.Invalid++
		d.stopped = true
		d.counts.Discarded += uint64(len(d.buffer))
		d.buffer = nil
	}
	if len(b) > maxWireBuffer {
		d.counts.Discarded += uint64(len(b) - (128 << 10))
		b = b[len(b)-(128<<10):]
	}
	d.buffer = append(d.buffer, b...)
	if d.stopped && (outbound || !d.recoverReceive()) {
		return nil
	}
	header := 9
	if outbound {
		header = 13
	}
	out := []jp9Candidate{}
	for len(d.buffer) >= header {
		id := binary.LittleEndian.Uint32(d.buffer)
		wireID := id
		if !outbound {
			id = jpReceiveMessageKind(wireID)
		}
		size := binary.LittleEndian.Uint32(d.buffer[4:8])
		if size > maxDecodedBlock {
			d.counts.Invalid++
			d.stopped = true
			d.last = nil
			if !outbound && d.recoverReceive() {
				continue
			}
			break
		}
		n := header + int(size)
		if len(d.buffer) < n {
			if !outbound && d.counts.Frames == 0 && len(d.buffer) >= 64 && jp9RecoveryOffset(d.buffer) > 0 {
				d.stopped = true
				if d.recoverReceive() {
					continue
				}
			}
			break
		}
		body := d.buffer[header:n]
		if !outbound && d.buffer[8] == 1 {
			decoded, err := io.ReadAll(io.LimitReader(brotli.NewReader(bytes.NewReader(body)), maxDecodedBlock+1))
			if err == nil && len(decoded) <= maxDecodedBlock {
				body = decoded
				d.counts.BrotliOK++
			}
		}
		if !outbound {
			previousMatch := uint64(0)
			target := uint64(0)
			if id == 0x5243 && len(body) == 53 {
				target = binary.LittleEndian.Uint64(body[8:])
				previousMatch = d.health.Values[target].MatchedDamage
			}
			d.health.Frame = d.counts.Frames + 1
			d.health.observe(id, body, at)
			if target != 0 && previousMatch == 0 && d.health.Values[target].MatchedDamage != 0 {
				d.counts.HPMatches++
				d.healthQueue = append(d.healthQueue, d.health.Values[target])
			}
			if id == 0x1d596 && len(body) == 21 && binary.LittleEndian.Uint32(body[8:]) == 8 {
				d.healthQueue = append(d.healthQueue, d.health.Values[binary.LittleEndian.Uint64(body)])
				d.counts.HPUpdates++
				if d.health.Values[binary.LittleEndian.Uint64(body)].MatchedDamage != 0 {
					d.counts.HPMatches++
				}
			}
		}
		if d.audit != nil {
			d.audit(d.counts.Frames+1, at, wireID, body, outbound)
		}
		if d.observer != nil {
			d.observer(wireID, body, outbound)
		}
		d.counts.Frames++
		if id == 0x1d4f8 {
			if !outbound {
				d.observeSkillLink(body)
			}
			if skill, ok := decodeJPSkillUse(body); ok && len(d.skillQueue) < 4096 {
				if d.skillActors == nil {
					d.skillActors = map[uint64]bool{}
				}
				if len(d.skillActors) < 4096 {
					d.skillActors[skill.Actor] = true
				}
				d.skillQueue = append(d.skillQueue, skill)
			}
		}
		if id == 0x1d4f8 && len(body) <= 4096 && len(d.skillBodies) < 256 {
			d.skillBodies = append(d.skillBodies, append([]byte(nil), body...))
		}
		if !outbound {
			d.joinSkillLink(id, body)
			if d.last != nil && id == 0x4ff7 && len(body) >= 16 && binary.LittleEndian.Uint64(body) == d.last.Actor && binary.LittleEndian.Uint64(body[8:]) == d.last.Target {
				d.counts.Paired++
			}
			d.last = nil
			if id == 0x5243 {
				if len(body) == 53 {
					c := jp9Candidate{Flags: decodeDamageFlags(body), Actor: binary.LittleEndian.Uint64(body), Target: binary.LittleEndian.Uint64(body[8:]), Damage64: binary.LittleEndian.Uint64(body[16:24]), Value: binary.LittleEndian.Uint32(body[16:20]), At: at}

					if d.skillRegistry == nil {
						d.skillRegistry = &jpSkillRegistry{entries: map[jpSkillKey]map[string]bool{}}
					}
					c.Link = &JPSkillLink{Actor: c.Actor, registry: d.skillRegistry, state: &jpSkillLinkState{}}
					copy(c.MatchField[:], body[32:40])
					if d.entities == nil {
						d.entities = map[uint64]bool{}
					}
					d.entities[c.Actor] = true
					d.entities[c.Target] = true
					d.counts.Candidates++
					d.counts.Sum += uint64(c.Value)
					out = append(out, c)
					d.last = &c
				} else {
					d.counts.Short5243++
				}
			}
		}
		d.buffer = d.buffer[n:]
	}
	d.buffer = append([]byte(nil), d.buffer...)
	return out
}
func (d *jp9Decoder) gap() { d.buffer = nil; d.last = nil; d.stopped = true; d.counts.Invalid++ }

type JPValue struct {
	SkillTarget   string
	SkillEffect   uint32
	SkillHasTail  bool
	Link          *JPSkillLink `json:"skillLink,omitempty"`
	Flags         *DamageFlags `json:"flagCandidates,omitempty"`
	ActorShortID  string       `json:"actorDisplayId,omitempty"`
	TargetShortID string       `json:"targetDisplayId,omitempty"`
	SkillCode     string       `json:"skillCode,omitempty"`
	ClassCode     string       `json:"classCode,omitempty"`
	ActorToken    string       `json:"actorToken,omitempty"`
	TargetToken   string       `json:"targetToken,omitempty"`
	EventToken    string       `json:"eventToken,omitempty"`
	At            time.Time    `json:"at"`
	Actor         string       `json:"actorAlias"`
	Target        string       `json:"targetAlias"`
	Damage64      uint64       `json:"damage64,omitempty"`
	HP            *JPHealth    `json:"hp,omitempty"`
	Value         uint32       `json:"candidateValue"`
}
type JPActor struct {
	Actor string `json:"actorAlias"`
	Count uint64 `json:"count"`
	Sum   uint64 `json:"candidateSum"`
}
type JPStats struct {
	Recovered    uint64    `json:"framingRecoveries"`
	Discarded    uint64    `json:"framingDiscardedBytes"`
	HPUpdates    uint64    `json:"hpUpdates"`
	HPMatches    uint64    `json:"hpDamageMatches"`
	BrotliOK     uint64    `json:"jp9BrotliDecoded"`
	SkillFrames  uint64    `json:"skill1d4f8Frames"`
	SkillMatches uint64    `json:"skillPayloadEntityWindowsHypothesisOnly"`
	InFrames     uint64    `json:"inboundFrames"`
	OutFrames    uint64    `json:"outboundFrames"`
	Count        uint64    `json:"candidateCount"`
	Sum          uint64    `json:"candidateSum"`
	Paired       uint64    `json:"pairedNext4ff7"`
	Invalid      uint64    `json:"invalidLengthOrGap"`
	WrongSize    uint64    `json:"unexpected5243PayloadLength"`
	Remainder    int       `json:"bufferedBytes"`
	Recent       []JPValue `json:"recentCandidates"`
	Actors       []JPActor `json:"actors"`
	Verified     bool      `json:"gameDamageVerified"`
}

func (p *CapturePipeline) collectJP9(st *captureStream, v gamePacketPayload) {
	before := st.jp.counts
	beforeSkill := len(st.jp.skillBodies)
	beforeEntities := len(st.jp.entities)
	beforeMatches := st.jp.skillMatches
	beforeRemainder := len(st.jp.buffer)
	if v.gap {
		st.jp.gap()
	}
	st.jp.health.Stream = st.meta
	st.jp.observer = p.jpFrameObserver
	if p.jpFrameAudit != nil {
		st.jp.audit = func(index uint64, at time.Time, id uint32, body []byte, out bool) {
			p.jpFrameAudit(st.meta, index, at, id, body, out)
		}
	}
	events := st.jp.feed(v.data, st.outbound, v.at)
	after := st.jp.counts
	s := &p.Stats.JP
	s.HPUpdates += after.HPUpdates - before.HPUpdates
	s.HPMatches += after.HPMatches - before.HPMatches
	s.BrotliOK += after.BrotliOK - before.BrotliOK
	s.SkillFrames += uint64(len(st.jp.skillBodies) - beforeSkill)
	if beforeSkill != len(st.jp.skillBodies) || beforeEntities != len(st.jp.entities) {
		st.jp.skillMatches = 0
		for _, body := range st.jp.skillBodies {
			for off := 0; off+8 <= len(body); off++ {
				id := binary.LittleEndian.Uint64(body[off:])
				if id != 0 && st.jp.entities[id] {
					st.jp.skillMatches++
				}
			}
		}
		s.SkillMatches += st.jp.skillMatches - beforeMatches
	}
	if st.outbound {
		s.OutFrames += after.Frames - before.Frames
	} else {
		s.InFrames += after.Frames - before.Frames
	}
	s.Count += after.Candidates - before.Candidates
	s.Sum += after.Sum - before.Sum
	s.Paired += after.Paired - before.Paired
	s.Invalid += after.Invalid - before.Invalid
	s.Recovered += after.Recovered - before.Recovered
	s.Discarded += after.Discarded - before.Discarded
	s.WrongSize += after.Short5243 - before.Short5243
	s.Remainder += len(st.jp.buffer) - beforeRemainder
	if p.jpActors == nil {
		p.jpActors = map[uint64]string{}
		p.jpTargets = map[uint64]string{}
	}
	if !p.jpKeyReady {
		_, err := rand.Read(p.jpKey[:])
		p.jpKeyReady = err == nil
	}
	for _, hp := range st.jp.healthQueue {
		if p.jpKeyReady && len(p.jpQueue) < 4096 {
			var a [8]byte
			binary.LittleEndian.PutUint64(a[:], hp.Entity)
			copyHP := hp
			p.jpQueue = append(p.jpQueue, JPValue{ActorToken: jpToken(p.jpKey, "entity", a[:]), ActorShortID: fmt.Sprintf("%06X", hp.Entity), At: hp.At, HP: &copyHP})
		}
	}
	st.jp.healthQueue = nil
	for _, skill := range st.jp.skillQueue {
		if p.jpKeyReady && len(p.jpQueue) < 4096 {
			var a [8]byte
			binary.LittleEndian.PutUint64(a[:], skill.Actor)
			p.jpQueue = append(p.jpQueue, JPValue{ActorToken: jpToken(p.jpKey, "entity", a[:]), ActorShortID: fmt.Sprintf("%06X", skill.Actor), At: v.at, SkillCode: skill.Code, ClassCode: skill.Class, SkillTarget: fmt.Sprintf("%06X", skill.Target), SkillEffect: skill.Effect, SkillHasTail: skill.HasTail})
		}
	}
	st.jp.skillQueue = nil
	for _, e := range events {
		actor, ok := p.jpActors[e.Actor]
		if !ok {
			if len(p.jpActors) >= 64 {
				actor = "攻撃者候補（上限）"
			} else {
				actor = fmt.Sprintf("攻撃者候補%d", len(p.jpActors)+1)
				p.jpActors[e.Actor] = actor
			}
		}
		target, ok := p.jpTargets[e.Target]
		if !ok {
			if len(p.jpTargets) >= 256 {
				target = "対象候補（上限）"
			} else {
				target = fmt.Sprintf("対象候補%d", len(p.jpTargets)+1)
				p.jpTargets[e.Target] = target
			}
		}
		value := JPValue{Link: e.Link.snapshot(), Flags: e.Flags, ActorShortID: fmt.Sprintf("%06X", e.Actor), TargetShortID: fmt.Sprintf("%06X", e.Target), At: e.At, Actor: actor, Target: target, Value: e.Value, Damage64: e.Damage64}
		if value.Flags != nil {
			value.Flags.Timestamp = e.At
			value.Flags.AttackerID = value.ActorShortID
			value.Flags.TargetID = value.TargetShortID
		}
		if p.jpKeyReady {
			var a, b [8]byte
			binary.LittleEndian.PutUint64(a[:], e.Actor)
			binary.LittleEndian.PutUint64(b[:], e.Target)
			if e.Actor != 0 {
				value.ActorToken = jpToken(p.jpKey, "entity", a[:])
			}
			if e.Target != 0 {
				value.TargetToken = jpToken(p.jpKey, "entity", b[:])
			}

		}
		p.jpSequence++
		value.EventToken = fmt.Sprintf("jp:%d", p.jpSequence)
		if len(p.jpQueue) < 4096 {
			p.jpQueue = append(p.jpQueue, value)
		} else {
			p.Stats.QueueDrops++
		}
		s.Recent = append(s.Recent, value)
		if len(s.Recent) > 50 {
			s.Recent = s.Recent[len(s.Recent)-50:]
		}
		found := false
		for i := range s.Actors {
			if s.Actors[i].Actor == actor {
				s.Actors[i].Count++
				s.Actors[i].Sum += uint64(e.Value)
				found = true
				break
			}
		}
		if !found {
			s.Actors = append(s.Actors, JPActor{actor, 1, uint64(e.Value)})
		}
	}
}
