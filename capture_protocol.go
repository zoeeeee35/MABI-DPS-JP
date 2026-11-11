package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/andybalholm/brotli"
)

// Wire facts are independently implemented from the public KR legacy layout.
// Neither JP compatibility nor the meaning of all fields is established.
type CaptureProfile struct {
	ID, Region   string
	ProcessNames []string
	Experimental bool
}

func profileFor(region string) (CaptureProfile, error) {
	switch region {
	case "jp":
		return CaptureProfile{"jp-kr-legacy-probe", "jp", []string{"MabinogiMobile.exe"}, true}, nil
	case "kr":
		return CaptureProfile{"kr-legacy-2025", "kr", []string{"MabinogiMobile.exe"}, true}, nil
	}
	return CaptureProfile{}, fmt.Errorf("取得プロファイルはjpまたはkrを指定してください")
}

var wireStart = []byte{0x3a, 4, 0, 0, 0, 0, 0, 0}
var wireEnd = []byte{4, 0, 0, 0, 0, 0, 0, 0}

const maxWireBuffer = 2 << 20
const maxDecodedBlock = 1 << 20

type RawDamage struct {
	Attacker, Target, Skill string
	Damage                  int64
	Critical                bool
	At                      time.Time
}
type DecodeCounts struct{ Frames, Blocks, Success, Failures, Unsupported, InvalidDamage, TLSStreams, Overflows, Gaps, StartMarkers, DamageMarkers, SkillMarkers uint64 }
type wireDecoder struct {
	start         []byte // offline envelope hypothesis only; nil retains the public legacy signature
	buffer        []byte
	signatureTail []byte
	Counts        DecodeCounts
	tls           bool
	testedTLS     bool
}

func (d *wireDecoder) reset() {
	d.buffer = nil
	d.signatureTail = nil
	d.tls = false
	d.testedTLS = false
}
func (d *wireDecoder) feed(data []byte, at time.Time) []RawDamage {
	if d.tls {
		return nil
	}
	d.probeMarkers(data)
	if len(d.buffer)+len(data) > maxWireBuffer {
		d.buffer = nil
		d.Counts.Overflows++
		d.Counts.Failures++
		return nil
	}
	d.buffer = append(d.buffer, data...)
	if !d.testedTLS && len(d.buffer) >= 5 {
		d.testedTLS = true
		b := d.buffer
		if (b[0] == 0x16 || b[0] == 0x17) && b[1] == 3 && b[2] <= 4 && binary.BigEndian.Uint16(b[3:5]) > 0 {
			d.tls = true
			d.Counts.TLSStreams++
			d.buffer = nil
			return nil
		}
	}
	startBytes := wireStart
	if len(d.start) == len(wireStart) {
		startBytes = d.start
	}
	var out []RawDamage
	for {
		start := bytes.Index(d.buffer, startBytes)
		if start < 0 {
			if len(d.buffer) > len(wireStart)-1 {
				d.buffer = append([]byte(nil), d.buffer[len(d.buffer)-len(wireStart)+1:]...)
			}
			break
		}
		if start > 0 {
			d.buffer = d.buffer[start:]
		}
		end := findFrameEnd(d.buffer)
		if end < 0 {
			break
		}
		frame := d.buffer[len(wireStart):end]
		d.Counts.Frames++
		events := d.blocks(frame, at)
		out = append(out, events...)
		d.buffer = append([]byte(nil), d.buffer[end+len(wireEnd):]...)
	}
	return out
}
func (d *wireDecoder) blocks(frame []byte, at time.Time) []RawDamage {
	out := []RawDamage{}
	found := false
	for i := 0; i+9 <= len(frame); {
		typ := binary.LittleEndian.Uint32(frame[i : i+4])
		if typ != 1283 && typ != 1432 {
			i++
			continue
		}
		found = true
		size := int(binary.LittleEndian.Uint32(frame[i+4 : i+8]))
		encoding := frame[i+8]
		if size <= 0 || size > maxDecodedBlock || size > len(frame)-i-9 {
			d.Counts.Failures++
			i++
			continue
		}
		d.Counts.Blocks++
		payload := frame[i+9 : i+9+size]
		i += 9 + size
		if encoding == 1 {
			var err error
			payload, err = io.ReadAll(io.LimitReader(brotli.NewReader(bytes.NewReader(payload)), maxDecodedBlock+1))
			if err != nil || len(payload) > maxDecodedBlock {
				d.Counts.Failures++
				continue
			}
		} else if encoding != 0 {
			d.Counts.Unsupported++
			d.Counts.Failures++
			continue
		}
		if typ == 1432 {
			continue
		} // Do not guess an Idle hit's skill from temporal proximity.
		e, invalid, err := decodeDamage(payload)
		if invalid {
			d.Counts.InvalidDamage++
			continue
		}
		if err != nil {
			d.Counts.Failures++
			continue
		}
		e.At = at
		out = append(out, e)
		d.Counts.Success++
	}
	if !found {
		d.Counts.Unsupported++
	}
	return out
}
func decodeDamage(p []byte) (RawDamage, bool, error) {
	var e RawDamage
	if len(p) < 46 {
		return e, false, fmt.Errorf("damage block is truncated")
	}
	n := int(binary.LittleEndian.Uint32(p[16:20]))
	if n < 0 || n > 4096 || n > len(p)-46 {
		return e, false, fmt.Errorf("skill string length is invalid")
	}
	// Skill-name bytes are skipped, not decoded: their JP encoding is unverified.

	value := binary.LittleEndian.Uint32(p[20+n : 24+n])
	if value == 0xffffffff {
		return e, true, nil
	}
	if value > 1000000000 {
		return e, false, fmt.Errorf("damage exceeds supported range")
	}
	// The reference reads four attacker bytes and skips four unknown bytes.
	// Those unknown bytes are not interpreted as a class or character name.
	e.Attacker = fmt.Sprintf("%x", p[:4])
	e.Target = fmt.Sprintf("%x", p[8:12])
	e.Skill = fmt.Sprintf("wire:%x", p[42+n:46+n])
	e.Damage = int64(value)
	e.Critical = p[36+n]&1 != 0
	if e.Attacker == "00000000" || e.Target == "00000000" {
		return RawDamage{}, false, fmt.Errorf("zero actor identifier")
	}
	return e, false, nil
}

type gamePacketPayload struct {
	stream uint32
	relSeq uint32
	data   []byte
	at     time.Time
	gap    bool
}

// Skip delimiter-shaped bytes inside a known length-delimited block. This
// does not invent an outer header or scan compressed payloads as messages.
func findFrameEnd(buffer []byte) int {
	search := len(wireStart)
	for search <= len(buffer) {
		rel := bytes.Index(buffer[search:], wireEnd)
		if rel < 0 {
			return -1
		}
		end := search + rel
		skip := end
		for i := len(wireStart); i+9 <= end; {
			typ := binary.LittleEndian.Uint32(buffer[i : i+4])
			if typ != 1283 && typ != 1432 {
				i++
				continue
			}
			n := int(binary.LittleEndian.Uint32(buffer[i+4 : i+8]))
			if n <= 0 || n > maxDecodedBlock {
				i++
				continue
			}
			blockEnd := i + 9 + n
			if blockEnd > end {
				skip = blockEnd
				break
			}
			i = blockEnd
		}
		if skip == end {
			return end
		}
		if skip > len(buffer) {
			return -1
		}
		search = skip
	}
	return -1
}

func (d *wireDecoder) probeMarkers(data []byte) {
	combined := append(append([]byte{}, d.signatureTail...), data...)
	old := len(d.signatureTail)
	count := func(pattern []byte) uint64 {
		var n uint64
		for i := 0; i+len(pattern) <= len(combined); i++ {
			if i+len(pattern) > old && bytes.Equal(combined[i:i+len(pattern)], pattern) {
				n++
			}
		}
		return n
	}
	d.Counts.StartMarkers += count(wireStart)
	d.Counts.DamageMarkers += count([]byte{3, 5, 0, 0})
	d.Counts.SkillMarkers += count([]byte{0x98, 5, 0, 0})
	keep := min(7, len(combined))
	d.signatureTail = append([]byte(nil), combined[len(combined)-keep:]...)
}
