package main

import (
	"bytes"
	"testing"
	"time"
)

func recoveryHit(v uint32) []byte { b := jpHit(1, 2, v); b[8] = 0; return b }
func TestJP9RecoversAfterGapAndSplitFrames(t *testing.T) {
	var d jp9Decoder
	now := time.Now()
	if len(d.feed(recoveryHit(10), false, now)) != 1 {
		t.Fatal("before gap")
	}
	d.gap()
	wire := append(bytes.Repeat([]byte{255}, 23), recoveryHit(20)...)
	wire = append(wire, recoveryHit(30)...)
	wire = append(wire, recoveryHit(40)...)
	var out []jp9Candidate
	for _, b := range wire {
		out = append(out, d.feed([]byte{b}, false, now)...)
	}
	if len(out) != 3 || d.stopped || d.counts.Recovered != 1 || d.counts.Candidates != 4 {
		t.Fatal(len(out), d.counts)
	}
	if out[0].Value != 20 || out[2].Value != 40 {
		t.Fatal(out)
	}
}
func TestJP9ResumesMidFrameWithoutCaptureRestart(t *testing.T) {
	var d jp9Decoder
	wire := bytes.Repeat([]byte{255}, 17)
	for _, v := range []uint32{1, 2, 3} {
		wire = append(wire, recoveryHit(v)...)
	}
	out := d.feed(wire, false, time.Now())
	if len(out) != 3 || d.counts.Recovered != 1 {
		t.Fatal(len(out), d.counts)
	}
}
func TestJP9RecoveryRejectsSingleAnchorAndUnknownFrames(t *testing.T) {
	var d jp9Decoder
	d.gap()
	d.feed(recoveryHit(1), false, time.Now())
	if !d.stopped || d.counts.Candidates != 0 {
		t.Fatal("single signature accepted")
	}
	wire := bytes.Repeat([]byte{0}, 4096)
	for _, v := range []uint32{3, 4, 5} {
		b := jpFrame(v, nil, false)
		b[8] = 0
		wire = append(wire, b...)
	}
	if jp9RecoveryOffset(wire) >= 0 {
		t.Fatal("unknown frames accepted")
	}
	for i := 0; i < 5; i++ {
		d.feed(bytes.Repeat([]byte{255}, 100000), false, time.Now())
	}
	if len(d.buffer) > 128<<10 {
		t.Fatal("unbounded recovery")
	}
}
