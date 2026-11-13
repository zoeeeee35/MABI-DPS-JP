package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
)

var jpClassNames = loadJPClassNames()

type IdentityProbe struct {
	NameLayouts map[string]int `json:"knownClassNameLayouts"`
	Frames      int            `json:"skill1d4f8Frames"`
	Lengths     map[int]int    `json:"skillPayloadLengths"`
	Classes     map[string]int `json:"knownClassStrings"`
	Links       map[string]int `json:"exact8ByteActorMatchesByOffsetHypothesis"`
	Partial     map[string]int `json:"partial4ByteActorMatchesNotProof"`
}

func classStrings(body []byte) []string {
	u := make([]uint16, len(body)/2)
	for i := range u {
		u[i] = binary.LittleEndian.Uint16(body[i*2:])
	}
	txt := string(body) + " " + string(utf16.Decode(u))
	found := []string{}
	for code := range jpClassNames {
		if strings.Contains(txt, code) {
			found = append(found, code)
		}
	}
	sort.Strings(found)
	return found
}
func (c *offlineCollector) identityProbe(p *CapturePipeline) IdentityProbe {
	r := IdentityProbe{NameLayouts: map[string]int{}, Lengths: map[int]int{}, Classes: map[string]int{}, Links: map[string]int{}, Partial: map[string]int{}}
	actors := map[uint64]bool{}
	bodies := [][]byte{}
	for _, run := range c.runs {
		if run.meta < 0 || run.meta >= len(p.Stats.Traffic) {
			continue
		}
		out := p.Stats.Traffic[run.meta].Direction == "send"
		header := 9
		if out {
			header = 13
		}
		data := run.data
		for len(data) >= header {
			id := binary.LittleEndian.Uint32(data)
			n := int(binary.LittleEndian.Uint32(data[4:]))
			if n > maxDecodedBlock || n < 0 || n+header > len(data) {
				break
			}
			body := data[header : header+n]
			if id == 0x5243 && !out && n == 53 {
				actors[binary.LittleEndian.Uint64(body)] = true
			}
			if id == 0x1d4f8 {
				r.Frames++
				r.Lengths[n]++
				bodies = append(bodies, body)
			}
			data = data[header+n:]
		}
	}
	ids := []uint64{}
	for id := range actors {
		if id != 0 {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, body := range bodies {
		classes := classStrings(body)
		for off := 0; off < len(body); off++ {
			for _, wide := range []bool{false, true} {
				name, start := skillCodeAt(body, off, wide)
				if len(classStrings([]byte(name))) > 0 {
					prefix := uint32(0)
					if start >= 4 {
						prefix = binary.LittleEndian.Uint32(body[start-4:])
					}
					valid := false
					_, valid = decodeJPSkill(body)
					tailZero := false
					if start+int(prefix) <= len(body) && prefix >= 2 {
						tailZero = binary.LittleEndian.Uint16(body[start+int(prefix)-2:]) == 0
					}
					r.NameLayouts[fmt.Sprintf("offset%d/wide%v/prefix%d/terminator%v/decoded%v/%s", start, wide, prefix, tailZero, valid, name)]++
				}
			}
		}
		for _, code := range classes {
			r.Classes[code]++
		}
		for i, id := range ids {
			for off := 0; off+4 <= len(body); off++ {
				if binary.LittleEndian.Uint32(body[off:]) == uint32(id) {
					r.Partial[fmt.Sprintf("actor%d/+%d", i+1, off)]++
				}
				if off+8 <= len(body) && binary.LittleEndian.Uint64(body[off:]) == id {
					r.Links[fmt.Sprintf("actor%d/+%d/classes:%s", i+1, off, strings.Join(classes, ","))]++
				}
			}
		}
	}
	return r
}

func skillCodeAt(b []byte, off int, wide bool) (string, int) {
	out := []byte{}
	step := 1
	if wide {
		step = 2
	}
	for i := off; i < len(b); i += step {
		ch := b[i]
		if wide && (i+1 >= len(b) || b[i+1] != 0) {
			break
		}
		if !((ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '.' || ch == '/') {
			break
		}
		out = append(out, ch)
		if len(out) > 128 {
			return "", off
		}
	}
	if len(out) < 3 {
		return "", off
	}
	return string(out), off
}

func loadJPClassNames() map[string]string {
	data, _ := assets.ReadFile("config/jp-class-prefixes.json")
	out := map[string]string{}
	_ = json.Unmarshal(data, &out)
	exe, _ := os.Executable()
	if data, e := os.ReadFile(filepath.Join(filepath.Dir(exe), "config", "jp-class-prefixes.json")); e == nil {
		var custom map[string]string
		if json.Unmarshal(data, &custom) == nil {
			for key, value := range custom {
				if len(key) > 0 && len(key) < 64 && len(value) > 0 && len(value) < 128 {
					out[key] = value
				}
			}
		}
	}
	return out
}
