package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

// Structural hypotheses only; none of these counters establish a game frame.
// No content, byte histogram, magic value or prefix fingerprint is exported.
type LengthCandidate struct {
	Sizes        [15]uint64 `json:"candidateSizeLog2Buckets"`
	Model        string     `json:"model"`
	Completed    uint64     `json:"completedCandidates"`
	Invalid      uint64     `json:"invalidLengths"`
	BestChain    uint64     `json:"longestConsecutiveChain"`
	CoveredBytes uint64     `json:"candidateBytes"`
	Buffered     int        `json:"bufferedBytes"`
}
type StructureStats struct {
	LegacyStarts  uint64            `json:"legacyStartsBeforeTLSGate"`
	LegacyEnds    uint64            `json:"legacyEndsBeforeTLSGate"`
	Chunks        uint64            `json:"tcpOutputChunksNotFrames"`
	Bytes         uint64            `json:"bytes"`
	Sizes         [17]uint64        `json:"chunkSizeLog2Buckets"`
	PrefixSamples uint64            `json:"prefixSamples"`
	EqualPrefix   [16]uint64        `json:"samePositionAsFirstChunkCounts"`
	Entropy       float64           `json:"byteEntropyBitsNotEncryptionProof"`
	GzipLike      uint64            `json:"gzipMagicOccurrences"`
	ZstdLike      uint64            `json:"zstdMagicOccurrences"`
	Resets        uint64            `json:"gapOrGenerationResets"`
	Models        []LengthCandidate `json:"lengthHypotheses"`
}
type lengthProbe struct {
	offset, width int
	be, includes  bool
	buf           []byte
	chain         uint64
	stats         LengthCandidate
}

const maxCandidateFrame = 16384

func (p *lengthProbe) feed(b []byte) {
	p.buf = append(p.buf, b...)
	header := p.offset + p.width
	for len(p.buf) >= header {
		v := p.buf[p.offset:header]
		var n uint64
		if p.width == 2 {
			if p.be {
				n = uint64(binary.BigEndian.Uint16(v))
			} else {
				n = uint64(binary.LittleEndian.Uint16(v))
			}
		} else if p.be {
			n = uint64(binary.BigEndian.Uint32(v))
		} else {
			n = uint64(binary.LittleEndian.Uint32(v))
		}
		if !p.includes {
			n += uint64(header)
		}
		if n < uint64(header+1) || n > maxCandidateFrame {
			p.stats.Invalid++
			p.chain = 0
			p.buf = p.buf[1:]
			continue
		}
		if n > uint64(len(p.buf)) {
			break
		}
		p.stats.Completed++
		bucket := 0
		for size := n; size > 1 && bucket < 14; size >>= 1 {
			bucket++
		}
		p.stats.Sizes[bucket]++
		p.stats.CoveredBytes += n
		p.chain++
		if p.chain > p.stats.BestChain {
			p.stats.BestChain = p.chain
		}
		p.buf = p.buf[int(n):]
	}
	// Detach from large capture buffers and bound retained content per hypothesis.
	p.buf = append([]byte(nil), p.buf...)
	p.stats.Buffered = len(p.buf)
}

type structureProbe struct {
	stats     StructureStats
	histogram [256]uint64
	first     [16]byte
	haveFirst bool
	tail      []byte
	models    []lengthProbe
}

func (p *structureProbe) init() {
	if len(p.models) > 0 {
		return
	}
	for _, off := range []int{0, 2, 4} {
		for _, w := range []int{2, 4} {
			for _, be := range []bool{false, true} {
				for _, inc := range []bool{false, true} {
					order := "le"
					if be {
						order = "be"
					}
					interpretation := "body"
					if inc {
						interpretation = "total"
					}
					p.models = append(p.models, lengthProbe{offset: off, width: w, be: be, includes: inc, stats: LengthCandidate{Model: fmt.Sprintf("offset%d-u%d%s-%s", off, w*8, order, interpretation)}})
				}
			}
		}
	}
}
func (p *structureProbe) reset() {
	p.init()
	p.stats.Resets++
	p.tail = nil
	p.haveFirst = false
	for i := range p.models {
		p.models[i].buf = nil
		p.models[i].chain = 0
		p.models[i].stats.Buffered = 0
	}
}
func (p *structureProbe) feed(b []byte) {
	if len(b) == 0 {
		return
	}
	p.init()
	p.stats.Chunks++
	p.stats.Bytes += uint64(len(b))
	bucket := 0
	for n := len(b); n > 1 && bucket < 16; n >>= 1 {
		bucket++
	}
	p.stats.Sizes[bucket]++
	for _, v := range b {
		p.histogram[v]++
	}
	if len(b) >= 16 {
		if !p.haveFirst {
			copy(p.first[:], b[:16])
			p.haveFirst = true
		} else {
			p.stats.PrefixSamples++
			for i := 0; i < 16; i++ {
				if b[i] == p.first[i] {
					p.stats.EqualPrefix[i]++
				}
			}
		}
	}
	scan := append(append([]byte(nil), p.tail...), b...)
	for i := 0; i < len(scan); i++ {
		if i+8 <= len(scan) && i+8 > len(p.tail) {
			if bytes.Equal(scan[i:i+8], wireStart) {
				p.stats.LegacyStarts++
			}
			if bytes.Equal(scan[i:i+8], wireEnd) {
				p.stats.LegacyEnds++
			}
		}
		if i+3 <= len(scan) && i+3 > len(p.tail) && scan[i] == 0x1f && scan[i+1] == 0x8b && scan[i+2] == 8 {
			p.stats.GzipLike++
		}
		if i+4 <= len(scan) && i+4 > len(p.tail) && scan[i] == 0x28 && scan[i+1] == 0xb5 && scan[i+2] == 0x2f && scan[i+3] == 0xfd {
			p.stats.ZstdLike++
		}
	}
	n := 7
	if len(scan) < n {
		n = len(scan)
	}
	p.tail = append([]byte(nil), scan[len(scan)-n:]...)
	for i := range p.models {
		p.models[i].feed(b)
	}
}
func (p *structureProbe) snapshot() StructureStats {
	s := p.stats
	for _, n := range p.histogram {
		if n > 0 {
			v := float64(n) / float64(s.Bytes)
			s.Entropy -= v * math.Log2(v)
		}
	}
	for i := range p.models {
		s.Models = append(s.Models, p.models[i].stats)
	}
	return s
}

// A measurement begins a fresh independent structural sample, without resetting
// TCP sequence tracking or the existing combat parser. Mid-frame starts remain possible.
func (p *CapturePipeline) resetStructureWindow() {
	for _, st := range p.streams {
		st.structure = structureProbe{}
	}
	for i := range p.Stats.Traffic {
		p.Stats.Traffic[i].Structure = StructureStats{}
	}
}
