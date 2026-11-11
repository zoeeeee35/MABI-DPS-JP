package main

import (
	"bytes"
	"encoding/binary"
	"github.com/andybalholm/brotli"
	"testing"
	"time"
)

func TestJPHealthCompressedSplitFrame(t *testing.T) {
	p := make([]byte, 21)
	binary.LittleEndian.PutUint64(p, 12)
	binary.LittleEndian.PutUint32(p[8:], 8)
	binary.LittleEndian.PutUint64(p[12:], 19159)
	var compressed bytes.Buffer
	w := brotli.NewWriter(&compressed)
	_, _ = w.Write(p)
	_ = w.Close()
	frame := make([]byte, 9)
	binary.LittleEndian.PutUint32(frame, 0x1d596)
	binary.LittleEndian.PutUint32(frame[4:], uint32(compressed.Len()))
	frame[8] = 1
	frame = append(frame, compressed.Bytes()...)
	d := jp9Decoder{}
	for _, b := range frame {
		if got := d.feed([]byte{b}, false, time.Now()); len(got) != 0 {
			t.Fatal("HP counted as damage")
		}
	}
	if len(d.healthQueue) != 1 || d.healthQueue[0].Current != 19159 {
		t.Fatal(d.healthQueue)
	}
}
func TestJP64BitDamageDecode(t *testing.T) {
	p := make([]byte, 62)
	binary.LittleEndian.PutUint32(p, 0x5243)
	binary.LittleEndian.PutUint32(p[4:], 53)
	binary.LittleEndian.PutUint64(p[25:], 1<<33)
	d := jp9Decoder{}
	got := d.feed(p, false, time.Now())
	if len(got) != 1 || got[0].Damage64 != 1<<33 {
		t.Fatal(got)
	}
}
