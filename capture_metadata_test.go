package main

import (
	"bytes"
	"encoding/json"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"testing"
	"time"
)

func reverseTCP(t *testing.T, data []byte) []byte {
	t.Helper()
	p := gopacket.NewPacket(data, layers.LinkTypeEthernet, gopacket.Default)
	ip := p.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
	tcp := p.Layer(layers.LayerTypeTCP).(*layers.TCP)
	ip.SrcIP, ip.DstIP = ip.DstIP, ip.SrcIP
	tcp.SrcPort, tcp.DstPort = tcp.DstPort, tcp.SrcPort
	_ = tcp.SetNetworkLayerForChecksum(ip)
	b := gopacket.NewSerializeBuffer()
	if e := gopacket.SerializeLayers(b, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, p.Layer(layers.LayerTypeEthernet).(*layers.Ethernet), ip, tcp, gopacket.Payload(tcp.Payload)); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestTrafficDirectionsBytesACKAndPrivacy(t *testing.T) {
	p := pipelineFixture()
	now := time.Now()
	frame := wireFixture(730, true)
	p.feed(ethernetTCP(t, 100, nil, 40000), layers.LinkTypeEthernet, now)
	if len(p.streams) != 0 {
		t.Fatal("ACK created parser stream")
	}
	p.feed(ethernetTCP(t, 100, frame, 40000), layers.LinkTypeEthernet, now)
	p.feed(ethernetTCP(t, 100, frame, 40000), layers.LinkTypeEthernet, now)
	if out := p.feed(reverseTCP(t, ethernetTCP(t, 900, frame, 40000)), layers.LinkTypeEthernet, now); len(out) != 0 {
		t.Fatal("outbound damage reached engine")
	}
	if p.Stats.DecodeSuccess != 1 || len(p.Stats.Traffic) != 2 {
		t.Fatal(p.Stats)
	}
	r, s := p.Stats.Traffic[0], p.Stats.Traffic[1]
	if r.Packets != 3 || r.EmptyPayloadPackets != 1 || r.PayloadBytes != uint64(2*len(frame)) || r.ReassembledBytes != uint64(len(frame)) || s.Direction != "send" || s.ReassembledBytes != uint64(len(frame)) || s.PeerPort != 16000 {
		t.Fatal(r, s)
	}
	b, _ := json.Marshal(p.Stats)
	for _, bad := range []string{"192.0.2.1", "198.51.100.2", "\"Skill\"", "payload\"", "sequence"} {
		if bytes.Contains(b, []byte(bad)) {
			t.Fatalf("contents exported: %s", bad)
		}
	}
	saved := cloneStats(p.Stats)
	p.Stats.Traffic[0].Packets++
	if saved.Traffic[0].Packets != 3 {
		t.Fatal("mutable snapshot")
	}
}
func TestTrafficWindowDeltaAndGap(t *testing.T) {
	p := pipelineFixture()
	now := time.Now()
	p.feed(ethernetTCP(t, 100, []byte{1, 2}, 40000), layers.LinkTypeEthernet, now)
	before := cloneStats(p.Stats)
	p.feed(ethernetTCP(t, 105, []byte{3, 4}, 40000), layers.LinkTypeEthernet, now)
	p.expire(now.Add(4 * time.Second))
	delta := trafficDelta(before, p.Stats)
	if len(delta) != 1 || delta[0].PayloadBytes != 2 || delta[0].ReassembledBytes != 2 || delta[0].Gaps != 1 || delta[0].PendingBytes != 0 {
		t.Fatal(delta)
	}
	if p.Stats.ReassemblyGaps != 1 {
		t.Fatal(p.Stats)
	}
}
func TestUDPOutboundBytesNeverEnterDecoder(t *testing.T) {
	p := newPipeline()
	p.setUDPEndpoints(map[string]bool{udpEndpointKey("0.0.0.0", 40000): true})
	ip := &layers.IPv4{Version: 4, IHL: 5, TTL: 64, Protocol: layers.IPProtocolUDP, SrcIP: []byte{198, 51, 100, 2}, DstIP: []byte{192, 0, 2, 1}}
	u := &layers.UDP{SrcPort: 40000, DstPort: 16000}
	_ = u.SetNetworkLayerForChecksum(ip)
	eth := &layers.Ethernet{SrcMAC: []byte{2, 0, 0, 0, 0, 1}, DstMAC: []byte{2, 0, 0, 0, 0, 2}, EthernetType: layers.EthernetTypeIPv4}
	b := gopacket.NewSerializeBuffer()
	frame := wireFixture(730, false)
	if e := gopacket.SerializeLayers(b, gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true}, eth, ip, u, gopacket.Payload(frame)); e != nil {
		t.Fatal(e)
	}
	if out := p.feed(b.Bytes(), layers.LinkTypeEthernet, time.Now()); len(out) > 0 {
		t.Fatal(out)
	}
	if len(p.Stats.Traffic) != 1 || p.Stats.Traffic[0].Direction != "send" || p.Stats.Traffic[0].PayloadBytes != uint64(len(frame)) || p.Stats.DecodeSuccess != 0 {
		t.Fatal(p.Stats)
	}
}
