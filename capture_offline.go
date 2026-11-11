package main

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"github.com/andybalholm/brotli"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type FrameHypothesis struct {
	off, width, add int
	be              bool
	Model           string  `json:"model"`
	Offset          int     `json:"startOffset"`
	Chain           int     `json:"consecutiveFrames"`
	Bytes           int     `json:"coveredBytes"`
	StartOnChunk    int     `json:"startsOnTCPChunk"`
	CrossChunk      int     `json:"framesCrossingTCPChunks"`
	FirstSeconds    float64 `json:"firstSeconds"`
	LastSeconds     float64 `json:"lastSeconds"`
}
type OfflineStream struct {
	Envelope        EnvelopeProbe     `json:"dominantPrefixLegacyEnvelopeProbe"`
	ZeroPrefix      [48]float64       `json:"zeroFractionAtTCPChunkOffsets"`
	Transforms      TransformStats    `json:"transformProbesNotProtocolProof"`
	ChunkLengthFits map[string]int    `json:"lengthFitsAtTCPChunkStartsNotProof"`
	ID              string            `json:"id"`
	Direction       string            `json:"direction"`
	Port            uint16            `json:"peerPort"`
	Retained        int               `json:"analyzedBytes"`
	Limited         bool              `json:"analysisLimitReached"`
	Gap             bool              `json:"stoppedAtGap"`
	Best            []FrameHypothesis `json:"hypothesesNotConfirmedFrames"`
}
type offlineRun struct {
	data         []byte
	starts       []int
	at           []time.Time
	gap, limited bool
	meta         int
}
type offlineCollector struct{ runs map[uint32]*offlineRun }

func (c *offlineCollector) feed(st *captureStream, v gamePacketPayload) {
	if c.runs == nil {
		c.runs = map[uint32]*offlineRun{}
	}
	r := c.runs[st.tcp.id]
	if r == nil {
		if len(c.runs) >= 8 {
			return
		}
		r = &offlineRun{meta: st.meta}
		c.runs[st.tcp.id] = r
	}
	if v.gap {
		r.gap = true
	}
	if r.gap || r.limited {
		return
	}
	if len(r.data)+len(v.data) > 256<<10 {
		r.limited = true
		return
	}
	r.starts = append(r.starts, len(r.data))
	r.at = append(r.at, v.at)
	r.data = append(r.data, v.data...)
}
func lengthAt(b []byte, pos, off, w int, be bool) int64 {
	v := b[pos+off : pos+off+w]
	if w == 2 {
		if be {
			return int64(binary.BigEndian.Uint16(v))
		}
		return int64(binary.LittleEndian.Uint16(v))
	}
	if be {
		return int64(binary.BigEndian.Uint32(v))
	}
	return int64(binary.LittleEndian.Uint32(v))
}
func scanFrameHypotheses(r *offlineRun) []FrameHypothesis {
	b := r.data
	count := make([]int, len(b)+1)
	best := []FrameHypothesis{}
	for off := 0; off <= 12; off++ {
		for _, w := range []int{2, 4} {
			for _, be := range []bool{false, true} {
				for _, add := range []int{-8, -4, 0, 2, 4, 6, 8, 12, 16, 20, 24, 32} {
					for i := range count {
						count[i] = 0
					}
					chain, start := 0, 0
					size := func(pos int) int {
						raw := lengthAt(b, pos, off, w, be)
						if raw <= 0 {
							return 0
						}
						n := raw + int64(add)
						if n < int64(off+w+1) || n > 1<<20 || n > int64(len(b)-pos) {
							return 0
						}
						return int(n)
					}
					for pos := len(b) - off - w; pos >= 0; pos-- {
						n := size(pos)
						if n > 0 {
							count[pos] = 1 + count[pos+n]
							if count[pos] > chain {
								chain = count[pos]
								start = pos
							}
						}
					}
					if chain < 4 {
						continue
					}
					order := "le"
					if be {
						order = "be"
					}
					h := FrameHypothesis{Model: fmt.Sprintf("offset%d-u%d%s-add%d", off, w*8, order, add), Offset: start, Chain: chain, off: off, width: w, add: add, be: be}
					pos := start
					for i := 0; i < chain; i++ {
						n := size(pos)
						j := sort.SearchInts(r.starts, pos)
						if j < len(r.starts) && r.starts[j] == pos {
							h.StartOnChunk++
						}
						k := sort.SearchInts(r.starts, pos+1)
						if k < len(r.starts) && r.starts[k] < pos+n {
							h.CrossChunk++
						}
						pos += n
					}
					h.Bytes = pos - start
					if len(r.at) > 0 {
						a := sort.Search(len(r.starts), func(i int) bool { return r.starts[i] > start }) - 1
						z := sort.Search(len(r.starts), func(i int) bool { return r.starts[i] >= pos }) - 1
						if a >= 0 {
							h.FirstSeconds = r.at[a].Sub(r.at[0]).Seconds()
						}
						if z >= 0 {
							h.LastSeconds = r.at[z].Sub(r.at[0]).Seconds()
						}
					}
					best = append(best, h)
				}
			}
		}
	}
	sort.Slice(best, func(i, j int) bool {
		if best[i].Chain != best[j].Chain {
			return best[i].Chain > best[j].Chain
		}
		return best[i].Bytes > best[j].Bytes
	})
	if len(best) > 12 {
		best = best[:12]
	}
	return best
}
func (c *offlineCollector) report(p *CapturePipeline) []OfflineStream {
	out := []OfflineStream{}
	for _, r := range c.runs {
		if r.meta < 0 || r.meta >= len(p.Stats.Traffic) {
			continue
		}
		v := p.Stats.Traffic[r.meta]
		hyps := scanFrameHypotheses(r)
		out = append(out, OfflineStream{inferLegacyEnvelope(r), zeroPrefixRates(r), analyzeTransforms(r, hyps), chunkLengthFits(r), v.ID, v.Direction, v.PeerPort, len(r.data), r.limited, r.gap, hyps})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func chunkLengthFits(r *offlineRun) map[string]int {
	out := map[string]int{}
	for i, pos := range r.starts {
		end := len(r.data)
		if i+1 < len(r.starts) {
			end = r.starts[i+1]
		}
		size := end - pos
		for off := 0; off <= 48; off++ {
			for _, w := range []int{2, 4} {
				if size < off+w {
					continue
				}
				for _, be := range []bool{false, true} {
					v := lengthAt(r.data, pos, off, w, be)
					if v <= 0 {
						continue
					}
					for _, add := range []int{-8, -4, 0, 2, 4, 6, 8, 12, 16, 20, 24, 32} {
						if v+int64(add) == int64(size) {
							order := "le"
							if be {
								order = "be"
							}
							out[fmt.Sprintf("offset%d-u%d%s-add%d", off, w*8, order, add)]++
						}
					}
				}
			}
		}
	}
	for k, v := range out {
		if v < 3 {
			delete(out, k)
		}
	}
	return out
}

type TransformStats struct {
	Attempts      int `json:"attempts"`
	BrotliDecoded int `json:"brotliDecodedCandidates"`
	GzipDecoded   int `json:"gzipDecodedCandidates"`
	ZlibDecoded   int `json:"zlibDecodedCandidates"`
	ExpandedBytes int `json:"expandedBytes"`
	LegacyEvents  int `json:"legacyEventCandidates"`
	Failures      int `json:"invalidOrUnsupportedCandidates"`
}

func analyzeTransforms(r *offlineRun, hyps []FrameHypothesis) TransformStats {
	s := TransformStats{}
	for _, h := range hyps {
		if h.Chain < 8 {
			continue
		}
		pos := h.Offset
		for i := 0; i < h.Chain && s.Attempts < 96; i++ {
			n := int(lengthAt(r.data, pos, h.off, h.width, h.be)) + h.add
			if n <= h.off+h.width || pos+n > len(r.data) {
				break
			}
			body := r.data[pos+h.off+h.width : pos+n]
			pos += n
			if len(body) < 8 || len(body) > 65536 {
				continue
			}
			s.Attempts++
			var reader io.Reader
			kind := "brotli"
			if len(body) >= 3 && body[0] == 0x1f && body[1] == 0x8b && body[2] == 8 {
				v, e := gzip.NewReader(bytes.NewReader(body))
				if e != nil {
					s.Failures++
					continue
				}
				defer v.Close()
				reader = v
				kind = "gzip"
			} else if body[0]&15 == 8 && (int(body[0])*256+int(body[1]))%31 == 0 {
				v, e := zlib.NewReader(bytes.NewReader(body))
				if e != nil {
					s.Failures++
					continue
				}
				defer v.Close()
				reader = v
				kind = "zlib"
			} else {
				reader = brotli.NewReader(bytes.NewReader(body))
			}
			expanded, e := io.ReadAll(io.LimitReader(reader, 262145))
			if e != nil || len(expanded) == 0 || len(expanded) > 262144 {
				s.Failures++
				continue
			}
			switch kind {
			case "gzip":
				s.GzipDecoded++
			case "zlib":
				s.ZlibDecoded++
			default:
				s.BrotliDecoded++
			}
			s.ExpandedBytes += len(expanded)
			d := wireDecoder{}
			_ = d.blocks(expanded, time.Time{})
			s.LegacyEvents += int(d.Counts.Success)
		}
		if s.Attempts >= 96 {
			break
		}
	}
	return s
}

func zeroPrefixRates(r *offlineRun) [48]float64 {
	var out [48]float64
	var samples [48]int
	for i, pos := range r.starts {
		end := len(r.data)
		if i+1 < len(r.starts) {
			end = r.starts[i+1]
		}
		for j := 0; j < 48 && pos+j < end; j++ {
			samples[j]++
			if r.data[pos+j] == 0 {
				out[j]++
			}
		}
	}
	for i := range out {
		if samples[i] > 0 {
			out[i] /= float64(samples[i])
		}
	}
	return out
}

type EnvelopeProbe struct {
	pattern       string
	PrefixSamples int    `json:"prefixSamples"`
	Dominant      int    `json:"dominantPrefixCount"`
	Frames        uint64 `json:"delimiterFrameCandidates"`
	Blocks        uint64 `json:"legacyBlocks"`
	Success       uint64 `json:"legacyDamageCandidates"`
	Failures      uint64 `json:"legacyDecodeFailures"`
}

func inferLegacyEnvelope(r *offlineRun) EnvelopeProbe {
	s := EnvelopeProbe{}
	counts := map[string]int{}
	var key string
	for i, pos := range r.starts {
		end := len(r.data)
		if i+1 < len(r.starts) {
			end = r.starts[i+1]
		}
		if end-pos < 8 {
			continue
		}
		s.PrefixSamples++
		k := string(r.data[pos : pos+8])
		counts[k]++
		if counts[k] > s.Dominant {
			key = k
			s.Dominant = counts[k]
		}
	}
	s.pattern = key
	if s.Dominant < 30 || s.Dominant*10 < s.PrefixSamples*9 {
		return s
	}
	d := wireDecoder{start: []byte(key)}
	_ = d.feed(r.data, time.Time{})
	s.Frames = d.Counts.Frames
	s.Blocks = d.Counts.Blocks
	s.Success = d.Counts.Success
	s.Failures = d.Counts.Failures
	return s
}

// Compare summaries locally; never serialize the internal common prefix bytes.
func compareCaptureDirectory(dir string) (map[string]any, error) {
	result := map[string]any{"liveVerified": false, "rawContentInReport": false}
	headers := map[string]string{}
	for _, label := range []string{"idle", "normal_attack", "skill_attack"} {
		f, e := os.Open(filepath.Join(dir, label+".pcapng"))
		if e != nil {
			return nil, fmt.Errorf("%s.pcapngを開けません", label)
		}
		s, e := replayPCAP(f, 16000)
		f.Close()
		if e != nil {
			return nil, e
		}
		result[label] = s
		for _, v := range s.Offline {
			if v.Direction == "receive" {
				headers[label] = v.Envelope.pattern
				break
			}
		}
	}
	summaries := map[string]ConditionSummary{}
	for _, label := range []string{"idle", "normal_attack", "skill_attack"} {
		s := result[label].(CaptureStats)
		v := ConditionSummary{}
		for _, f := range s.Traffic {
			if f.Direction == "receive" {
				v.ReceiveBytes += int64(f.PayloadBytes)
				v.ReceiveDataPackets += int64(f.Packets - f.EmptyPayloadPackets)
			} else {
				v.SendBytes += int64(f.PayloadBytes)
				v.SendDataPackets += int64(f.Packets - f.EmptyPayloadPackets)
			}
		}
		summaries[label] = v
	}
	result["conditionSummaries"] = summaries
	a, b, c := summaries["idle"], summaries["normal_attack"], summaries["skill_attack"]
	result["normalMinusIdle"] = subtractCondition(b, a)
	result["skillMinusNormal"] = subtractCondition(c, b)
	result["sameDominantReceivePrefix"] = len(headers["idle"]) > 0 && headers["idle"] == headers["normal_attack"] && headers["normal_attack"] == headers["skill_attack"]
	return result, nil
}

type ConditionSummary struct {
	ReceiveBytes       int64 `json:"receiveBytes"`
	SendBytes          int64 `json:"sendBytes"`
	ReceiveDataPackets int64 `json:"receiveDataPackets"`
	SendDataPackets    int64 `json:"sendDataPackets"`
}

func subtractCondition(a, b ConditionSummary) ConditionSummary {
	return ConditionSummary{a.ReceiveBytes - b.ReceiveBytes, a.SendBytes - b.SendBytes, a.ReceiveDataPackets - b.ReceiveDataPackets, a.SendDataPackets - b.SendDataPackets}
}
