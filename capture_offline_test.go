package main

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"github.com/andybalholm/brotli"
	"io"
	"testing"
	"time"
)

func TestOfflineAllOffsetScanAndSplitTiming(t *testing.T) {
	for _, seed := range []int{3, 29, 81} {
		b := structureFixture(seed)
		r := &offlineRun{data: b}
		for pos := 0; pos < len(b); pos += 7 {
			r.starts = append(r.starts, pos)
			r.at = append(r.at, time.Unix(0, int64(pos)*1000000))
		}
		hyps := scanFrameHypotheses(r)
		found := false
		for _, h := range hyps {
			if h.Chain >= 12 && h.Bytes == len(b) && h.CrossChunk > 0 {
				found = true
			}
		}
		if !found {
			t.Fatal(seed, hyps)
		}
	}
}
func TestOfflineBoundedTransformProbes(t *testing.T) {
	for _, kind := range []string{"gzip", "zlib", "brotli"} {
		body := wireFixture(730, false)
		body = body[8 : len(body)-8]
		var compressed bytes.Buffer
		var w io.WriteCloser
		switch kind {
		case "gzip":
			w = gzip.NewWriter(&compressed)
		case "zlib":
			w = zlib.NewWriter(&compressed)
		default:
			w = brotli.NewWriter(&compressed)
		}
		_, _ = w.Write(body)
		_ = w.Close()
		var data []byte
		for i := 0; i < 12; i++ {
			frame := make([]byte, 4)
			binary.LittleEndian.PutUint32(frame, uint32(compressed.Len()+4))
			frame = append(frame, compressed.Bytes()...)
			data = append(data, frame...)
		}
		s := analyzeTransforms(&offlineRun{data: data}, []FrameHypothesis{{Chain: 12, off: 0, width: 4}})
		if s.LegacyEvents != 12 || s.Attempts != 12 || s.Failures != 0 {
			t.Fatal(kind, s)
		}
	}
}
func TestOfflineEnvelopeHypothesisDoesNotExportPrefix(t *testing.T) {
	prefix := []byte("SECRET!!")
	r := &offlineRun{}
	for i := 0; i < 40; i++ {
		frame := wireFixture(730, false)
		copy(frame[:8], prefix)
		r.starts = append(r.starts, len(r.data))
		r.at = append(r.at, time.Now())
		r.data = append(r.data, frame...)
	}
	s := inferLegacyEnvelope(r)
	if s.Frames != 40 || s.Success != 40 {
		t.Fatal(s.Frames, s.Success)
	}
	b, _ := json.Marshal(s)
	if bytes.Contains(b, prefix) {
		t.Fatal("raw prefix serialized")
	}
}
