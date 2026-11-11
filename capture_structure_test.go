package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"
)

func structureFixture(seed int) []byte {
	var out []byte
	for i := 0; i < 12; i++ {
		n := 20 + (i*17+seed)%91
		b := make([]byte, n)
		binary.LittleEndian.PutUint32(b, uint32(n))
		for j := 4; j < n; j++ {
			b[j] = byte(j*13 + i + seed)
		}
		out = append(out, b...)
	}
	return out
}
func modelNamed(s StructureStats, name string) LengthCandidate {
	for _, v := range s.Models {
		if v.Model == name {
			return v
		}
	}
	return LengthCandidate{}
}
func TestStructureLengthHypothesisIndependentSamplesAndSegmentSplits(t *testing.T) {
	for _, seed := range []int{3, 29, 81} {
		b := structureFixture(seed)
		var whole, split structureProbe
		whole.feed(b)
		for _, v := range b {
			split.feed([]byte{v})
		}
		a, c := modelNamed(whole.snapshot(), "offset0-u32le-total"), modelNamed(split.snapshot(), "offset0-u32le-total")
		if a.Completed != 12 || a.BestChain != 12 || a.CoveredBytes != uint64(len(b)) || a != c {
			t.Fatalf("seed%d whole%+v split%+v", seed, a, c)
		}
	}
}
func TestStructureMidstreamAmbiguityAndIndependentWindow(t *testing.T) {
	b := structureFixture(29)
	var p structureProbe
	p.feed(append([]byte{255, 255, 255, 255}, b...))
	m := modelNamed(p.snapshot(), "offset0-u32le-total")
	// An overlapping plausible length can stall a greedy hypothesis; do not label it recovered.
	if m.BestChain == 12 || m.Invalid == 0 {
		t.Fatal(m)
	}
	pipe := pipelineFixture()
	pipe.streams["test"] = &captureStream{structure: p, meta: 0}
	pipe.Stats.Traffic = []TrafficFlow{{Structure: p.snapshot()}}
	pipe.resetStructureWindow()
	if pipe.Stats.Traffic[0].Structure.Bytes != 0 || pipe.streams["test"].structure.stats.Bytes != 0 {
		t.Fatal("window retained contents/statistics")
	}
}
func TestStructureBoundedUnknownInputAndNoContentExport(t *testing.T) {
	var p structureProbe
	b := bytes.Repeat([]byte{255}, 80000)
	p.feed(b)
	s := p.snapshot()
	if s.Entropy != 0 {
		t.Fatal(s.Entropy)
	}
	for _, v := range s.Models {
		if v.Buffered >= maxCandidateFrame || v.Completed != 0 {
			t.Fatal(v)
		}
	}
	secret := []byte("private-player-auth-content")
	p.feed(secret)
	j, _ := json.Marshal(p.snapshot())
	if bytes.Contains(j, secret) || bytes.Contains(j, []byte("histogram")) {
		t.Fatal("payload exported")
	}
}
func TestStructureGapResetDoesNotJoinFramesAndMagicAcrossChunks(t *testing.T) {
	var p structureProbe
	b := structureFixture(3)
	p.feed(b[:10])
	p.reset()
	p.feed(b[10:])
	p.feed([]byte{0x1f})
	p.feed([]byte{0x8b, 8})
	p.feed([]byte{0x28, 0xb5})
	p.feed([]byte{0x2f, 0xfd})
	s := p.snapshot()
	if s.Resets != 1 || s.GzipLike != 1 || s.ZstdLike != 1 {
		t.Fatal(s)
	}
	if modelNamed(s, "offset0-u32le-total").Completed == 12 {
		t.Fatal("joined across hole")
	}
}

func TestStructureProbeWorksBeforeLegacyTLSGate(t *testing.T) {
	var d wireDecoder
	var p structureProbe
	tls := []byte{0x17, 3, 3, 0, 20, 9, 8, 7}
	frame := wireFixture(730, true)
	d.feed(tls, time.Now())
	d.feed(frame, time.Now())
	p.feed(tls)
	for _, b := range frame {
		p.feed([]byte{b})
	}
	if d.Counts.StartMarkers != 0 || d.Counts.TLSStreams != 1 || p.snapshot().LegacyStarts != 1 {
		t.Fatal(d.Counts, p.snapshot())
	}
}
