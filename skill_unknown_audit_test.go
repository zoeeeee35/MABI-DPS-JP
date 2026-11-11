package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Local-only semantic audit: never exports raw packet bodies or network endpoints.
func TestLocalUnidentifiedSkillAudit(t *testing.T) {
	path := os.Getenv("MABI_UNKNOWN_PCAP")
	if path == "" {
		t.Skip("local capture not supplied")
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	type notice struct {
		Conn                  int
		Index                 uint64
		At                    time.Time
		ID                    uint32
		Actor, Target, Damage uint64
		Effect                uint32
		Skill                 string
		Matched               bool
	}
	frames := []notice{}
	previous := map[int]struct {
		index uint64
		p     []byte
	}{}
	_, e = replayPCAPAudited(f, 16000, nil, func(conn int, index uint64, at time.Time, id uint32, p []byte, out bool) {
		if out {
			return
		}
		n := notice{Conn: conn, Index: index, At: at, ID: id}
		switch {
		case id == 0x5243 && len(p) == 53:
			n.Actor = binary.LittleEndian.Uint64(p)
			n.Target = binary.LittleEndian.Uint64(p[8:])
			n.Damage = binary.LittleEndian.Uint64(p[16:])
			previous[conn] = struct {
				index uint64
				p     []byte
			}{index, append([]byte(nil), p...)}
		case id == 0x4ff7 && len(p) == 35:
			n.Actor = binary.LittleEndian.Uint64(p)
			n.Target = binary.LittleEndian.Uint64(p[8:])
			n.Effect = binary.LittleEndian.Uint32(p[16:])
			prev := previous[conn]
			n.Matched = prev.index+1 == index && len(prev.p) == 53 && bytes.Equal(prev.p[:16], p[:16]) && bytes.Equal(prev.p[32:40], p[24:32])
		case id == 0x1d4f8:
			s, ok := decodeJPSkillUse(p)
			if !ok {
				return
			}
			n.Actor = s.Actor
			n.Skill = s.Code
			l := int(binary.LittleEndian.Uint32(p[8:]))
			tail := p[12+l:]
			if len(tail) == 29 {
				n.Target = binary.LittleEndian.Uint64(tail[17:])
				n.Effect = binary.LittleEndian.Uint32(tail[25:])
			}
		default:
			return
		}
		frames = append(frames, n)
	})
	if e != nil {
		t.Fatal(e)
	}
	actors := map[uint64]bool{}
	for _, n := range frames {
		if (os.Getenv("MABI_UNKNOWN_ACTOR") != "" && fmt.Sprintf("%X", n.Actor) == os.Getenv("MABI_UNKNOWN_ACTOR")) || (os.Getenv("MABI_UNKNOWN_ACTOR") == "" && (n.Damage == 37174 || n.Damage == 915)) {
			actors[n.Actor] = true
		}
	}
	relevant := []notice{}
	for _, n := range frames {
		if actors[n.Actor] {
			relevant = append(relevant, n)
		}
	}
	out := os.Getenv("MABI_UNKNOWN_REPORT")
	if out == "" {
		out = filepath.Join(t.TempDir(), "audit.json")
	}
	b, e := json.MarshalIndent(relevant, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(out, b, 0600); e != nil {
		t.Fatal(e)
	}
	t.Logf("matching actors=%d semantic notices=%d", len(actors), len(relevant))
}
