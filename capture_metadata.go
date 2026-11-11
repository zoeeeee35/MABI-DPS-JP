package main

import "fmt"

// Metadata only: never export addresses, sequence numbers or contents.
type TrafficFlow struct {
	Structure           StructureStats `json:"structure"`
	DamageMarkers       uint64         `json:"legacyDamageMarkers"`
	SkillMarkers        uint64         `json:"legacySkillMarkers"`
	Failures            uint64         `json:"decodeFailures"`
	ID                  string         `json:"id"`
	Protocol            string         `json:"protocol"`
	Direction           string         `json:"direction"`
	PeerPort            uint16         `json:"peerPort"`
	DestinationPort     uint16         `json:"destinationPort"`
	Packets             uint64         `json:"packets"`
	PayloadBytes        uint64         `json:"payloadBytes"`
	EmptyPayloadPackets uint64         `json:"emptyPayloadPackets"`
	ReassembledBytes    uint64         `json:"reassembledBytes"`
	Gaps                uint64         `json:"gaps"`
	Starts              uint64         `json:"legacyStarts"`
	Frames              uint64         `json:"frames"`
	Events              uint64         `json:"events"`
	TLS                 uint64         `json:"tlsStreams"`
	PendingBytes        int            `json:"pendingBytes"`
}

func (p *CapturePipeline) traffic(key, protocol, direction string, peer, destination uint16, size int) int {
	if p.metadata == nil {
		p.metadata = map[string]int{}
	}
	i, ok := p.metadata[key]
	if !ok {
		if len(p.Stats.Traffic) >= 256 {
			p.Stats.ResourceDrops++
			return -1
		}
		i = len(p.Stats.Traffic)
		p.metadata[key] = i
		p.Stats.Traffic = append(p.Stats.Traffic, TrafficFlow{ID: fmt.Sprintf("flow-%03d", i+1), Protocol: protocol, Direction: direction, PeerPort: peer, DestinationPort: destination})
	}
	v := &p.Stats.Traffic[i]
	v.Packets++
	v.PayloadBytes += uint64(size)
	if size == 0 {
		v.EmptyPayloadPackets++
	}
	return i
}
func cloneStats(s CaptureStats) CaptureStats {
	s.JP.Recent = append([]JPValue(nil), s.JP.Recent...)
	for i := range s.JP.Recent {
		s.JP.Recent[i].Link = s.JP.Recent[i].Link.snapshot()
	}
	s.JP.Actors = append([]JPActor(nil), s.JP.Actors...)
	s.Traffic = append([]TrafficFlow(nil), s.Traffic...)
	for i := range s.Traffic {
		s.Traffic[i].Structure.Models = append([]LengthCandidate(nil), s.Traffic[i].Structure.Models...)
	}
	return s
}
func trafficDelta(before, after CaptureStats) []TrafficFlow {
	old := map[string]TrafficFlow{}
	for _, v := range before.Traffic {
		old[v.ID] = v
	}
	out := []TrafficFlow{}
	for _, v := range after.Traffic {
		b := old[v.ID]
		v.Packets -= b.Packets
		v.PayloadBytes -= b.PayloadBytes
		v.EmptyPayloadPackets -= b.EmptyPayloadPackets
		v.ReassembledBytes -= b.ReassembledBytes
		v.DamageMarkers -= b.DamageMarkers
		v.SkillMarkers -= b.SkillMarkers
		v.Failures -= b.Failures
		v.Gaps -= b.Gaps
		v.Starts -= b.Starts
		v.Frames -= b.Frames
		v.Events -= b.Events
		v.TLS -= b.TLS
		if v.Packets > 0 || v.ReassembledBytes > 0 {
			out = append(out, v)
		}
	}
	return out
}
