package main

import (
	"bytes"
	"encoding/binary"
	"github.com/gopacket/gopacket/pcapgo"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
)

func wireFixture(damage uint32, compressed bool) []byte {
	payload := make([]byte, 46+5)
	binary.LittleEndian.PutUint32(payload[0:4], 0x12345678)
	binary.LittleEndian.PutUint32(payload[8:12], 0x87654321)
	binary.LittleEndian.PutUint32(payload[16:20], 5)
	copy(payload[20:25], []byte("Skill"))
	binary.LittleEndian.PutUint32(payload[25:29], damage)
	payload[41] = 1
	copy(payload[47:51], []byte{1, 2, 3, 4})
	encoding := byte(0)
	if compressed {
		var b bytes.Buffer
		w := brotli.NewWriter(&b)
		_, _ = w.Write(payload)
		_ = w.Close()
		payload = b.Bytes()
		encoding = 1
	}
	block := make([]byte, 9)
	binary.LittleEndian.PutUint32(block, 1283)
	binary.LittleEndian.PutUint32(block[4:], uint32(len(payload)))
	block[8] = encoding
	frame := append([]byte{}, wireStart...)
	frame = append(frame, block...)
	frame = append(frame, payload...)
	return append(frame, wireEnd...)
}
func ethernetTCP(t *testing.T, seq uint32, data []byte, port uint16) []byte {
	t.Helper()
	eth := &layers.Ethernet{SrcMAC: []byte{2, 0, 0, 0, 0, 1}, DstMAC: []byte{2, 0, 0, 0, 0, 2}, EthernetType: layers.EthernetTypeIPv4}
	ip := &layers.IPv4{Version: 4, IHL: 5, TTL: 64, Protocol: layers.IPProtocolTCP, SrcIP: []byte{192, 0, 2, 1}, DstIP: []byte{198, 51, 100, 2}}
	tcp := &layers.TCP{SrcPort: 16000, DstPort: layers.TCPPort(port), Seq: seq, ACK: true, Window: 65535}
	_ = tcp.SetNetworkLayerForChecksum(ip)
	buf := gopacket.NewSerializeBuffer()
	if e := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, eth, ip, tcp, gopacket.Payload(data)); e != nil {
		t.Fatal(e)
	}
	return buf.Bytes()
}
func pipelineFixture() *CapturePipeline {
	p := newPipeline()
	p.setFlows(map[string]bool{flowKey("192.0.2.1", 16000, "198.51.100.2", 40000): true, flowKey("192.0.2.1", 16000, "198.51.100.2", 40001): true})
	return p
}
func TestLegacyWirePlainAndBrotli(t *testing.T) {
	for _, compressed := range []bool{false, true} {
		d := wireDecoder{}
		events := d.feed(wireFixture(1250, compressed), time.Now())
		if len(events) != 1 || events[0].Damage != 1250 || !events[0].Critical || events[0].Skill != "wire:01020304" || events[0].Attacker != "78563412" {
			t.Fatalf("compressed=%v: %#v counts=%+v", compressed, events, d.Counts)
		}
	}
}
func TestFrameSplitEveryByteAndDelimiterInPayload(t *testing.T) {
	for _, damage := range []uint32{4, 1000} {
		d := wireDecoder{}
		out := []RawDamage{}
		for _, b := range wireFixture(damage, false) {
			out = append(out, d.feed([]byte{b}, time.Now())...)
		}
		if len(out) != 1 || out[0].Damage != int64(damage) {
			t.Fatalf("damage=%d %+v %+v", damage, out, d.Counts)
		}
	}
}
func TestTCPReorderRetransmitAndIsolation(t *testing.T) {
	p := pipelineFixture()
	frame := wireFixture(1000, false)
	now := time.Now()
	if len(p.feed(ethernetTCP(t, 100, frame[:12], 40000), layers.LinkTypeEthernet, now)) != 0 {
		t.Fatal("incomplete frame decoded")
	}
	if len(p.feed(ethernetTCP(t, 128, frame[28:], 40000), layers.LinkTypeEthernet, now)) != 0 {
		t.Fatal("out of order frame decoded")
	}
	other := p.feed(ethernetTCP(t, 500, wireFixture(2000, true), 40001), layers.LinkTypeEthernet, now)
	if len(other) != 1 || other[0].Damage != 2000 {
		t.Fatal(other)
	}
	events := p.feed(ethernetTCP(t, 112, frame[12:28], 40000), layers.LinkTypeEthernet, now)
	if len(events) != 1 || events[0].Damage != 1000 {
		t.Fatal(events)
	}
	events = p.feed(ethernetTCP(t, 128, frame[28:], 40000), layers.LinkTypeEthernet, now)
	if len(events) != 0 || p.Stats.DecodeSuccess != 2 {
		t.Fatal("retransmission counted twice", p.Stats)
	}
}
func TestForeignFlowNeverDecoded(t *testing.T) {
	p := pipelineFixture()
	out := p.feed(ethernetTCP(t, 100, wireFixture(9999, false), 45000), layers.LinkTypeEthernet, time.Now())
	if len(out) != 0 || p.Stats.GamePackets != 0 || p.Stats.DecodeSuccess != 0 || p.Stats.Packets != 1 {
		t.Fatal(p.Stats)
	}
}
func TestTLSIsDiagnosedWithoutDecryption(t *testing.T) {
	d := wireDecoder{}
	out := d.feed([]byte{0x17, 3, 3, 0, 6, 1, 2, 3, 4, 5, 6}, time.Now())
	if len(out) != 0 || d.Counts.TLSStreams != 1 || !d.tls {
		t.Fatal(d.Counts)
	}
	if len(d.feed(wireFixture(1000, false), time.Now())) != 0 {
		t.Fatal("attempted plaintext parsing on TLS stream")
	}
}
func TestMalformedDamageAndEncoding(t *testing.T) {
	for n := 0; n < 46; n++ {
		if _, _, e := decodeDamage(make([]byte, n)); e == nil {
			t.Fatalf("accepted %d-byte payload", n)
		}
	}
	d := wireDecoder{}
	f := wireFixture(1000, false)
	f[len(wireStart)+8] = 2
	_ = d.feed(f, time.Now())
	if d.Counts.Failures != 1 || d.Counts.Success != 0 {
		t.Fatal(d.Counts)
	}
	d = wireDecoder{}
	_ = d.feed(wireFixture(0xffffffff, false), time.Now())
	if d.Counts.InvalidDamage != 1 || d.Counts.Success != 0 {
		t.Fatal(d.Counts)
	}
}
func TestLiveBridgeRequiresExplicitPartyAndPseudonymizes(t *testing.T) {
	now := time.Now()
	v, e := newLiveInput("jp", now, 5000)
	if e != nil {
		t.Fatal(e)
	}
	raw := RawDamage{Attacker: "raw-actor-sensitive", Target: "raw-target-sensitive", Skill: "wire:01020304", Damage: 1000, Critical: true, At: now}
	if e = v.append(raw); e != nil {
		t.Fatal(e)
	}
	_ = v.tick(now.Add(time.Second))
	if v.Aggregated != 0 || len(v.Session.Result.Battles) != 0 {
		t.Fatal("unknown actor classified as party")
	}
	if e = v.selectActor(0, true); e != nil {
		t.Fatal(e)
	}
	_ = v.tick(now.Add(time.Second))
	if v.Aggregated != 1 || v.Session.current().Players[0].DPS != 1000 {
		t.Fatal(v.Session.Result)
	}
	if bytes.Contains([]byte(v.Session.Log.Players[0].ID), []byte("raw-actor")) {
		t.Fatal("raw identifier leaked")
	}
	_ = v.tick(now.Add(5 * time.Second))
	if v.Session.current().Active {
		t.Fatal("live idle end not detected")
	}
}
func TestLiveArchiveKeyStableAsNewEventsArrive(t *testing.T) {
	s, _ := newSession(nativeFixture(t), 5000)
	s.Playing = true
	_ = s.advance(12000)
	b := s.Result.Battles[0]
	q := Request{Log: s.Log, Until: s.Cursor, Idle: s.Idle}
	key := archiveKey(q, b)
	q.Log.Events = append(q.Log.Events, Hit{ID: "later", At: 30000, Type: "end"})
	if archiveKey(q, b) != key {
		t.Fatal("completed battle dedup depends on future events")
	}
}
func TestCaptureDiagnosticStages(t *testing.T) {
	for _, c := range []struct {
		s    CaptureStats
		want string
	}{{CaptureStats{Running: false, Stage: "permission_denied"}, "permission_denied"}, {CaptureStats{Running: true}, "game_unidentified"}, {CaptureStats{Running: true, Connections: 1}, "waiting_packets"}, {CaptureStats{Running: true, Connections: 1, Packets: 1, GamePackets: 1, GamePayloads: 1}, "decode_failed"}, {CaptureStats{Running: true, DecodeSuccess: 1}, "events_extracted"}, {CaptureStats{Running: true, Aggregated: 1}, "aggregating"}, {CaptureStats{Running: true, QueueDrops: 1}, "resource_limit"}} {
		if got := diagnosticStage(c.s); got != c.want {
			t.Fatalf("got %s want %s", got, c.want)
		}
	}
}

func TestPCAPReplayWithOutOfOrderAndRetransmit(t *testing.T) {
	var capture bytes.Buffer
	w := pcapgo.NewWriter(&capture)
	if e := w.WriteFileHeader(65535, layers.LinkTypeEthernet); e != nil {
		t.Fatal(e)
	}
	frame := wireFixture(1234, true)
	now := time.Now()
	for _, segment := range []struct {
		seq  uint32
		data []byte
	}{{100, frame[:12]}, {128, frame[28:]}, {112, frame[12:28]}, {128, frame[28:]}} {
		data := ethernetTCP(t, segment.seq, segment.data, 40000)
		if e := w.WritePacket(gopacket.CaptureInfo{Timestamp: now, CaptureLength: len(data), Length: len(data)}, data); e != nil {
			t.Fatal(e)
		}
	}
	stats, e := replayPCAP(bytes.NewReader(capture.Bytes()), 16000)
	if e != nil || stats.DecodeSuccess != 1 || stats.Frames != 1 || stats.Packets != 4 {
		t.Fatal(stats, e)
	}
	if _, e = replayPCAP(bytes.NewReader(capture.Bytes()), 0); e == nil {
		t.Fatal("accepted implicit port")
	}
}
func TestWireToLiveEngineAndArchive(t *testing.T) {
	now := time.Now()
	p := pipelineFixture()
	v, _ := newLiveInput("jp", now, 5000)
	events := p.feed(ethernetTCP(t, 100, wireFixture(1250, true), 40000), layers.LinkTypeEthernet, now)
	if len(events) != 1 {
		t.Fatal(p.Stats)
	}
	for _, e := range events {
		if err := v.append(e); err != nil {
			t.Fatal(err)
		}
	}
	_ = v.selectActor(0, true)
	_ = v.tick(now.Add(5 * time.Second))
	b := v.Session.current()
	if b == nil || b.Active || b.Damage != 1250 || b.Players[0].DPS != 1250 || v.Aggregated != 1 {
		t.Fatal(v.Session.Result)
	}
	path, e := saveBattle(t.TempDir(), Request{Log: v.Session.Log, Until: v.Session.Cursor, Idle: v.Session.Idle}, *b)
	if e != nil {
		t.Fatal(e)
	}
	saved, e := loadArchive(filepath.Clean(path))
	if e != nil || saved.Battle.Damage != 1250 || saved.Archive.LiveVerified {
		t.Fatal(saved, e)
	}
}
func TestLegacyMarkerDiagnosticsAcrossSegments(t *testing.T) {
	d := wireDecoder{}
	frame := wireFixture(1000, false)
	for _, b := range frame {
		_ = d.feed([]byte{b}, time.Now())
	}
	if d.Counts.StartMarkers != 1 || d.Counts.DamageMarkers != 1 || d.Counts.Success != 1 {
		t.Fatal(d.Counts)
	}
}

func TestUDPMetadataDoesNotDecodePayload(t *testing.T) {
	for _, tc := range []struct {
		name, dst, binding string
		v6                 bool
		want               uint64
	}{{"exact v4", "198.51.100.2", "198.51.100.2", false, 1}, {"wildcard v4", "198.51.100.2", "0.0.0.0", false, 1}, {"foreign v4", "198.51.100.2", "198.51.100.9", false, 0}, {"wildcard v6", "2001:db8::2", "::", true, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			p := newPipeline()
			p.setUDPEndpoints(map[string]bool{udpEndpointKey(tc.binding, 40000): true})
			udp := &layers.UDP{SrcPort: 16000, DstPort: 40000}
			eth := &layers.Ethernet{SrcMAC: []byte{2, 0, 0, 0, 0, 1}, DstMAC: []byte{2, 0, 0, 0, 0, 2}, EthernetType: layers.EthernetTypeIPv4}
			var network gopacket.SerializableLayer
			if tc.v6 {
				eth.EthernetType = layers.EthernetTypeIPv6
				ip := &layers.IPv6{Version: 6, NextHeader: layers.IPProtocolUDP, HopLimit: 64, SrcIP: net.ParseIP("2001:db8::1"), DstIP: net.ParseIP(tc.dst)}
				_ = udp.SetNetworkLayerForChecksum(ip)
				network = ip
			} else {
				ip := &layers.IPv4{Version: 4, IHL: 5, TTL: 64, Protocol: layers.IPProtocolUDP, SrcIP: net.ParseIP("192.0.2.1"), DstIP: net.ParseIP(tc.dst)}
				_ = udp.SetNetworkLayerForChecksum(ip)
				network = ip
			}
			buffer := gopacket.NewSerializeBuffer()
			if e := gopacket.SerializeLayers(buffer, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, eth, network, udp, gopacket.Payload(wireFixture(730, false))); e != nil {
				t.Fatal(e)
			}
			out := p.feed(buffer.Bytes(), layers.LinkTypeEthernet, time.Now())
			if len(out) != 0 || p.Stats.DecodeSuccess != 0 || p.Stats.UDPPackets != 1 || p.Stats.GameUDPCandidates != tc.want || p.Stats.GamePackets != 0 {
				t.Fatal(p.Stats)
			}
		})
	}
}
func TestUDPDiagnosticIsCandidateOnly(t *testing.T) {
	s := CaptureStats{Running: true, GameUDPCandidates: 1}
	if diagnosticStage(s) != "udp_observed" {
		t.Fatal(diagnosticStage(s))
	}
	s.Aggregated = 1
	if diagnosticStage(s) != "aggregating" {
		t.Fatal("aggregation must take precedence")
	}
}
