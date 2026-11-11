package main

import (
	"encoding/binary"
	"github.com/gopacket/gopacket/layers"
	"testing"
	"time"
)

func jpFrame(id uint32, body []byte, outbound bool) []byte {
	n := 9
	if outbound {
		n = 13
	}
	b := make([]byte, n)
	binary.LittleEndian.PutUint32(b, id)
	binary.LittleEndian.PutUint32(b[4:], uint32(len(body)))
	b[n-1] = 7
	return append(b, body...)
}
func jpHit(actor, target uint64, value uint32) []byte {
	b := make([]byte, 53)
	binary.LittleEndian.PutUint64(b, actor)
	binary.LittleEndian.PutUint64(b[8:], target)
	binary.LittleEndian.PutUint64(b[16:], uint64(value))
	binary.LittleEndian.PutUint32(b[24:], 999999)
	return jpFrame(0x5243, b, false)
}
func TestJP9SplitCoalescedEmptyAndExactOffset(t *testing.T) {
	now := time.Now()
	hit := jpHit(1<<40|5, 2<<40|6, 730)
	pair := make([]byte, 16)
	binary.LittleEndian.PutUint64(pair, 1<<40|5)
	binary.LittleEndian.PutUint64(pair[8:], 2<<40|6)
	stream := append(jpFrame(1, nil, false), hit...)
	stream = append(stream, jpFrame(0x4ff7, pair, false)...)
	for _, split := range []bool{false, true} {
		var d jp9Decoder
		var out []jp9Candidate
		if split {
			for _, b := range stream {
				out = append(out, d.feed([]byte{b}, false, now)...)
			}
		} else {
			out = d.feed(stream, false, now)
		}
		if len(out) != 1 || out[0].Value != 730 || out[0].Actor != 1<<40|5 || d.counts.Frames != 3 || d.counts.Paired != 1 || len(d.buffer) != 0 {
			t.Fatal(out, d.counts)
		}
	}
}
func TestJP9OutboundAndMalformedStayCandidatesOnly(t *testing.T) {
	var out jp9Decoder
	body := make([]byte, 53)
	binary.LittleEndian.PutUint32(body[16:], 730)
	out.feed(jpFrame(0x5243, body, true), true, time.Now())
	if out.counts.Frames != 1 || out.counts.Candidates != 0 {
		t.Fatal(out.counts)
	}
	var d jp9Decoder
	d.feed(jpFrame(0x5243, body[:52], false), false, time.Now())
	if d.counts.Short5243 != 1 || d.counts.Candidates != 0 {
		t.Fatal(d.counts)
	}
	bad := make([]byte, 9)
	binary.LittleEndian.PutUint32(bad[4:], maxDecodedBlock+1)
	d.feed(bad, false, time.Now())
	d.feed(jpHit(1, 2, 730), false, time.Now())
	if !d.stopped || d.counts.Candidates != 0 {
		t.Fatal(d.counts)
	}
}
func TestJP9PipelineReorderDuplicateAndNoDPSEvents(t *testing.T) {
	p := pipelineFixture()
	p.Stats.Profile = "jp-kr-legacy-probe"
	b := jpHit(1, 2, 730)
	now := time.Now()
	for _, v := range []struct {
		seq uint32
		b   []byte
	}{{100, b[:12]}, {125, b[25:]}, {112, b[12:25]}, {100, b}} {
		if out := p.feed(ethernetTCP(t, v.seq, v.b, 40000), layers.LinkTypeEthernet, now); len(out) > 0 {
			t.Fatal("candidate entered DPS bridge")
		}
	}
	if p.Stats.JP.Count != 1 || p.Stats.JP.Sum != 730 || p.Stats.JP.Verified || len(p.Stats.JP.Recent) != 1 {
		t.Fatal(p.Stats.JP)
	}
	snapshot := cloneStats(p.Stats)
	p.Stats.JP.Recent[0].Value = 1
	if snapshot.JP.Recent[0].Value != 730 {
		t.Fatal("mutable snapshot")
	}
}
func TestJP9GapRejectsUnalignedContinuationAndPairNeedsNextFrame(t *testing.T) {
	var d jp9Decoder
	h := jpHit(1, 2, 730)
	d.feed(h[:20], false, time.Now())
	d.gap()
	d.feed(h[20:], false, time.Now())
	if d.counts.Candidates != 0 || d.counts.Invalid != 1 {
		t.Fatal(d.counts)
	}
	var paired jp9Decoder
	paired.feed(h, false, time.Now())
	paired.feed(jpFrame(1, nil, false), false, time.Now())
	paired.feed(jpFrame(0x4ff7, h[9:25], false), false, time.Now())
	if paired.counts.Paired != 0 {
		t.Fatal("non-adjacent pair accepted")
	}
}
