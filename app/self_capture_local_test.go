package main

import (
	"encoding/binary"
	"os"
	"testing"
)

// Explicit opt-in local analysis. No packet contents or identifiers are logged.
func TestLocalOutboundSelfEvidence(t *testing.T) {
	filename := os.Getenv("MABI_SELF_CAPTURE")
	if filename == "" {
		t.Skip("local capture not specified")
	}
	f, e := os.Open(filename)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	in, out := map[uint64]int{}, map[uint64]int{}
	hits := map[uint64]int{}
	_, e = replayPCAPObserved(f, 16000, func(id uint32, p []byte, send bool) {
		if id == 0x1d4f8 {
			if skill, ok := decodeJPSkillUse(p); ok {
				if send {
					out[skill.Actor]++
				} else {
					in[skill.Actor]++
				}
			}
		}
		if !send && jpReceiveMessageKind(id) == 0x5243 && len(p) == 53 {
			hits[binary.LittleEndian.Uint64(p)]++
		}
	})
	if e != nil {
		t.Fatal(e)
	}
	nout, matched, nin := 0, 0, 0
	for actor, n := range out {
		nout += n
		if hits[actor] > 0 {
			matched++
		}
	}
	for _, n := range in {
		nin += n
	}
	t.Logf("inboundSkills=%d inboundSkillActors=%d outboundSkills=%d outboundSkillActors=%d outboundActorsWithInboundDamage=%d", nin, len(in), nout, len(out), matched)
}
