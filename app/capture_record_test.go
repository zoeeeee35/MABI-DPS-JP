package main

import (
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLocalPCAPNGFilteringDirectionReplayAndDeletion(t *testing.T) {
	dir := t.TempDir()
	p := pipelineFixture()
	now := time.Now()
	r, e := openLocalRecorder(dir, "normal_attack", layers.LinkTypeEthernet, p.flows, now)
	if e != nil {
		t.Fatal(e)
	}
	frame := wireFixture(730, true)
	for _, b := range [][]byte{ethernetTCP(t, 100, frame, 45000), reverseTCP(t, ethernetTCP(t, 100, []byte{1, 2, 3}, 40000)), ethernetTCP(t, 100, frame, 40000)} {
		if e = r.write(b, gopacket.CaptureInfo{Timestamp: now, CaptureLength: len(b), Length: len(b)}); e != nil {
			t.Fatal(e)
		}
	}
	if e = r.close("test"); e != nil {
		t.Fatal(e)
	}
	if r.stats.Packets != 2 || r.stats.Bytes == 0 {
		t.Fatal(r.stats)
	}
	f, e := os.Open(r.stats.Path)
	if e != nil {
		t.Fatal(e)
	}
	ng, e := pcapgo.NewNgReader(f, pcapgo.DefaultNgReaderOptions)
	if e != nil {
		t.Fatal(e)
	}
	_, ci, opts, e := ng.ReadPacketDataWithOptions()
	if e != nil || !ci.Timestamp.Equal(now) || opts.Flags == nil || opts.Flags.Direction != pcapgo.NgEpbFlagDirectionOutbound {
		t.Fatal(ci, opts, e)
	}
	f.Close()
	f, _ = os.Open(r.stats.Path)
	s, e := replayPCAP(f, 16000)
	f.Close()
	if e != nil || s.DecodeSuccess != 1 || len(s.Traffic) != 2 {
		t.Fatal(s, e)
	}
	if _, e = openLocalRecorder(dir, "normal_attack", layers.LinkTypeEthernet, p.flows, now); e == nil {
		t.Fatal("overwrote recording")
	}
	untouched := filepath.Join(dir, "keep.txt")
	_ = os.WriteFile(untouched, []byte("keep"), 0600)
	if e = deleteLocalRecordings(dir); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(r.stats.Path); !os.IsNotExist(e) {
		t.Fatal("recording retained")
	}
	if _, e = os.Stat(untouched); e != nil {
		t.Fatal("unrelated file deleted")
	}
}
func TestLocalRecordBoundsAndRequiresOwnership(t *testing.T) {
	dir := t.TempDir()
	if _, e := openLocalRecorder(dir, "idle", layers.LinkTypeEthernet, nil, time.Now()); e == nil {
		t.Fatal("no ownership accepted")
	}
	p := pipelineFixture()
	r, e := openLocalRecorder(dir, "idle", layers.LinkTypeEthernet, p.flows, time.Now())
	if e != nil {
		t.Fatal(e)
	}
	r.stats.Bytes = recordingLimit - 100
	b := ethernetTCP(t, 100, []byte{1}, 40000)
	if e = r.write(b, gopacket.CaptureInfo{Timestamp: time.Now(), CaptureLength: len(b), Length: len(b)}); e != nil {
		t.Fatal(e)
	}
	if r.stats.Active || r.stats.Reason != "capacity_limit" || r.stats.Packets != 0 {
		t.Fatal(r.stats)
	}
}
func TestManualRecordingContinuesPastThirtySecondsAndReplays(t *testing.T) {
	dir := t.TempDir()
	p := pipelineFixture()
	now := time.Now()
	r, e := openLocalRecorder(dir, "manual", layers.LinkTypeEthernet, p.flows, now)
	if e != nil {
		t.Fatal(e)
	}
	b := ethernetTCP(t, 100, jpHit(1, 2, 730), 40000)
	at := now.Add(90 * time.Second)
	if e = r.write(b, gopacket.CaptureInfo{Timestamp: at, CaptureLength: len(b), Length: len(b)}); e != nil {
		t.Fatal(e)
	}
	if !r.stats.Active || !r.stats.Manual || r.stats.Packets != 1 {
		t.Fatal(r.stats)
	}
	if e = r.close("manual_stop"); e != nil {
		t.Fatal(e)
	}
	f, _ := os.Open(r.stats.Path)
	s, e := replayPCAP(f, 16000)
	f.Close()
	if e != nil || s.JP.Count != 1 || s.JP.Sum != 730 {
		t.Fatal(s.JP, e)
	}
	r2, e := openLocalRecorder(dir, "manual", layers.LinkTypeEthernet, p.flows, now.Add(time.Nanosecond))
	if e != nil {
		t.Fatal(e)
	}
	if r2.stats.Path == r.stats.Path {
		t.Fatal("overwritten")
	}
	_ = r2.close("manual_stop")
	keep := filepath.Join(dir, "classification-user-notes.pcapng")
	_ = os.WriteFile(keep, []byte("keep"), 0600)
	if e = deleteLocalRecordings(dir); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(r.stats.Path); !os.IsNotExist(e) {
		t.Fatal("record retained")
	}
	if _, e = os.Stat(keep); e != nil {
		t.Fatal("unrelated file deleted")
	}
}
func TestManualCapacityBound(t *testing.T) {
	p := pipelineFixture()
	now := time.Now()
	r, e := openLocalRecorder(t.TempDir(), "manual", layers.LinkTypeEthernet, p.flows, now)
	if e != nil {
		t.Fatal(e)
	}
	r.stats.Bytes = (64 << 20) - 100
	b := ethernetTCP(t, 100, []byte{1}, 40000)
	if e = r.write(b, gopacket.CaptureInfo{Timestamp: now, CaptureLength: len(b), Length: len(b)}); e != nil {
		t.Fatal(e)
	}
	if r.stats.Active || r.stats.Reason != "capacity_limit" {
		t.Fatal(r.stats)
	}
}
