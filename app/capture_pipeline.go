package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
)

type CaptureStats struct {
	AggregationSegments     uint64 "json:\"aggregationSegments\""
	LastSegmentReason       string "json:\"lastSegmentReason\""
	FirstPacket, LastPacket time.Time
	replayAliases           map[uint64]string
	Identity                IdentityProbe `json:"identityResearch"`
	Provisional             *Result       `json:"provisionalDPS,omitempty"`
	provisionalLog          *CombatLog
	JP                      JPStats         `json:"jp9"`
	Offline                 []OfflineStream `json:"offlineStructure,omitempty"`
	Recording               RecordingStats  `json:"recording"`
	Traffic                 []TrafficFlow   `json:"traffic"`
	ReassembledStreams      uint64          `json:"reassembledStreams"`
	CaptureDecodeErrors     uint64          `json:"captureDecodeErrors"`
	UDPPackets              uint64          `json:"udpPackets"`
	GameUDPCandidates       uint64          `json:"gameUdpCandidates"`
	UDPEndpoints            int             `json:"udpEndpoints"`
	Profile                 string          `json:"profile"`
	StartMarkers            uint64          `json:"legacyStartMarkers"`
	DamageMarkers           uint64          `json:"legacyDamageMarkers"`
	SkillMarkers            uint64          `json:"legacySkillMarkers"`
	Running                 bool            `json:"running"`
	Stage                   string          `json:"stage"`
	Detail                  string          `json:"detail"`
	Packets                 uint64          `json:"packets"`
	GamePackets             uint64          `json:"gamePackets"`
	GamePayloads            uint64          `json:"gamePayloads"`
	Connections             int             `json:"connections"`
	Frames                  uint64          `json:"frames"`
	DecodeSuccess           uint64          `json:"decodeSuccess"`
	DecodeFailures          uint64          `json:"decodeFailures"`
	UnsupportedFrames       uint64          `json:"unsupportedFrames"`
	TLSStreams              uint64          `json:"tlsStreams"`
	ReassemblyGaps          uint64          `json:"reassemblyGaps"`
	ResourceDrops           uint64          `json:"resourceDrops"`
	QueueDrops              uint64          `json:"queueDrops"`
	Aggregated              uint64          `json:"aggregated"`
	DriverDrops             uint64          `json:"driverDrops"`
	LiveVerified            bool            `json:"liveVerified"`
}

func diagnosticStage(s CaptureStats) string {
	if !s.Running {
		return s.Stage
	}
	if s.ResourceDrops+s.QueueDrops > 0 {
		return "resource_limit"
	}
	if s.JP.Count > 0 && s.Aggregated > 0 {
		return "jp_provisional_dps"
	}
	if s.JP.Count > 0 {
		return "jp_candidate"
	}
	if s.JP.InFrames > 0 {
		return "jp_frames"
	}
	if s.Aggregated > 0 {
		return "aggregating"
	}
	if s.DecodeSuccess > 0 {
		return "events_extracted"
	}
	if s.GameUDPCandidates > 0 {
		return "udp_observed"
	}
	if s.Connections == 0 {
		return "game_unidentified"
	}
	if s.Packets == 0 {
		return "waiting_packets"
	}
	if s.GamePackets == 0 {
		return "game_unidentified"
	}
	if s.GamePayloads > 0 {
		return "decode_failed"
	}
	return "waiting_game_payload"
}
func stageText(stage string) string {
	switch stage {
	case "jp_provisional_dps":
		return "実受信DPSを集計中（種別未分類）"
	case "jp_candidate":
		return "ダメージ候補受信（未照合・DPS未計算）"
	case "jp_frames":
		return "日本版フレーム受信・0x5243候補待ち"
	case "udp_observed":
		return "ゲーム所有UDP候補あり・TCP戦闘形式は未検出"
	case "driver_missing":
		return "Npcapを読み込めません"
	case "adapter_unavailable":
		return "アダプターを利用できません"
	case "permission_denied":
		return "キャプチャ権限が不足しています"
	case "waiting_packets":
		return "通信未受信"
	case "game_unidentified":
		return "対象ゲームのTCP接続を識別できません"
	case "waiting_game_payload":
		return "対象接続を検出・受信データ待ち"
	case "decode_failed":
		return "ゲーム通信受信・既知戦闘形式は未検出"
	case "events_extracted":
		return "戦闘イベント候補を抽出・集計対象の選択待ち"
	case "aggregating":
		return "実受信イベント候補をDPS集計中（解釈未検証）"
	case "resource_limit":
		return "上限到達・欠落があるため結果は不完全です"
	case "stopping":
		return "キャプチャ停止中"
	case "capture_error":
		return "キャプチャ処理に失敗しました"
	default:
		return "停止"
	}
}
func flowKey(src string, sport uint16, dst string, dport uint16) string {
	return fmt.Sprintf("%s|%d>%s|%d", src, sport, dst, dport)
}

type captureStream struct {
	tcp       tcpStreamState
	decoder   wireDecoder
	last      time.Time
	meta      int
	outbound  bool
	delivered bool
	structure structureProbe
	jp        jp9Decoder
	jpEnabled bool
}
type CapturePipeline struct {
	jpFrameAudit    func(int, uint64, time.Time, uint32, []byte, bool)
	jpFrameObserver func(uint32, []byte, bool)
	jpKey           [32]byte
	jpKeyReady      bool
	jpSequence      uint64
	jpQueue         []JPValue
	jpActors        map[uint64]string
	jpTargets       map[uint64]string
	offline         *offlineCollector
	metadata        map[string]int
	Stats           CaptureStats
	flows           map[string]bool
	udpEndpoints    map[string]bool
	streams         map[string]*captureStream
	next            uint32
}

func newPipeline() *CapturePipeline {
	return &CapturePipeline{flows: map[string]bool{}, streams: map[string]*captureStream{}, Stats: CaptureStats{Running: true, Stage: "waiting_packets"}}
}
func (p *CapturePipeline) setFlows(flows map[string]bool) {
	p.flows = flows
	p.Stats.Connections = len(flows)
}
func (p *CapturePipeline) collect(st *captureStream, payload gamePacketPayload) []RawDamage {
	if st.jpEnabled {
		p.collectJP9(st, payload)
	}
	if payload.gap {
		st.structure.reset()
		st.decoder.reset()
		p.Stats.ReassemblyGaps++
	}
	if len(payload.data) > 0 && !st.delivered {
		st.delivered = true
		p.Stats.ReassembledStreams++
	}
	if st.meta >= 0 {
		v := &p.Stats.Traffic[st.meta]
		v.ReassembledBytes += uint64(len(payload.data))
		if payload.gap {
			v.Gaps++
		}
	}
	if p.offline != nil {
		p.offline.feed(st, payload)
	}
	st.structure.feed(payload.data)
	if st.meta >= 0 {
		p.Stats.Traffic[st.meta].Structure = st.structure.snapshot()
	}
	before := st.decoder.Counts
	events := st.decoder.feed(payload.data, payload.at)
	after := st.decoder.Counts
	if st.meta >= 0 {
		v := &p.Stats.Traffic[st.meta]
		v.Starts += after.StartMarkers - before.StartMarkers
		v.DamageMarkers += after.DamageMarkers - before.DamageMarkers
		v.SkillMarkers += after.SkillMarkers - before.SkillMarkers
		v.Failures += after.Failures - before.Failures
		v.Frames += after.Frames - before.Frames
		v.Events += after.Success - before.Success
		v.TLS += after.TLSStreams - before.TLSStreams
	}
	// Outbound probes cannot establish actual server damage.
	if st.outbound {
		return nil
	}
	p.Stats.StartMarkers += after.StartMarkers - before.StartMarkers
	p.Stats.DamageMarkers += after.DamageMarkers - before.DamageMarkers
	p.Stats.SkillMarkers += after.SkillMarkers - before.SkillMarkers
	p.Stats.Frames += after.Frames - before.Frames
	p.Stats.DecodeSuccess += after.Success - before.Success
	p.Stats.DecodeFailures += after.Failures - before.Failures
	p.Stats.UnsupportedFrames += after.Unsupported - before.Unsupported
	p.Stats.TLSStreams += after.TLSStreams - before.TLSStreams
	p.Stats.ResourceDrops += after.Overflows - before.Overflows
	if st.jpEnabled {
		return nil
	}
	return events
}
func (p *CapturePipeline) expire(now time.Time) []RawDamage {
	var out []RawDamage
	for key, st := range p.streams {
		st.tcp.expire(now, func(v gamePacketPayload) { out = append(out, p.collect(st, v)...) })
		if st.meta >= 0 {
			p.Stats.Traffic[st.meta].PendingBytes = st.tcp.pendingBytes
		}
		if now.Sub(st.last) > 60*time.Second {
			delete(p.streams, key)
		}
	}
	return out
}
func (p *CapturePipeline) feed(data []byte, link layers.LinkType, at time.Time) []RawDamage {
	p.Stats.Packets++
	packet := gopacket.NewPacket(data, link, gopacket.DecodeOptions{Lazy: false, NoCopy: true})
	if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		p.Stats.UDPPackets++
		if network := packet.NetworkLayer(); network != nil {
			src, dst := network.NetworkFlow().Endpoints()
			u := udpLayer.(*layers.UDP)
			owns := func(address string, port uint16) bool {
				wildcard := "0.0.0.0"
				if strings.Contains(address, ":") {
					wildcard = "::"
				}
				return p.udpEndpoints[udpEndpointKey(address, port)] || p.udpEndpoints[udpEndpointKey(wildcard, port)]
			}
			direction := ""
			peer := uint16(u.SrcPort)
			if owns(dst.String(), uint16(u.DstPort)) {
				direction = "receive"
			} else if owns(src.String(), uint16(u.SrcPort)) {
				direction = "send"
				peer = uint16(u.DstPort)
			}
			if direction != "" {
				p.Stats.GameUDPCandidates++
				p.traffic("udp:"+flowKey(src.String(), uint16(u.SrcPort), dst.String(), uint16(u.DstPort)), "udp", direction, peer, uint16(u.DstPort), len(u.Payload))
			}
		}
		return nil
	}
	layer := packet.Layer(layers.LayerTypeTCP)
	if layer == nil {
		return nil
	}
	tcp := layer.(*layers.TCP)
	var src, dst string
	if ip := packet.Layer(layers.LayerTypeIPv4); ip != nil {
		v := ip.(*layers.IPv4)
		if v.FragOffset != 0 || v.Flags&layers.IPv4MoreFragments != 0 {
			p.Stats.CaptureDecodeErrors++
			return nil
		}
		src = v.SrcIP.String()
		dst = v.DstIP.String()
	} else if ip := packet.Layer(layers.LayerTypeIPv6); ip != nil {
		v := ip.(*layers.IPv6)
		if packet.Layer(layers.LayerTypeIPv6Fragment) != nil {
			p.Stats.CaptureDecodeErrors++
			return nil
		}
		src = v.SrcIP.String()
		dst = v.DstIP.String()
	} else {
		return nil
	}
	key := flowKey(src, uint16(tcp.SrcPort), dst, uint16(tcp.DstPort))
	outbound := false
	if !p.flows[key] {
		if p.flows[flowKey(dst, uint16(tcp.DstPort), src, uint16(tcp.SrcPort))] {
			outbound = true
		} else {
			return nil
		}
	}
	direction := "receive"
	peer := uint16(tcp.SrcPort)
	if outbound {
		direction = "send"
		peer = uint16(tcp.DstPort)
	}
	meta := p.traffic("tcp:"+key, "tcp", direction, peer, uint16(tcp.DstPort), len(tcp.Payload))
	if !outbound {
		p.Stats.GamePackets++
	}
	if packet.ErrorLayer() != nil {
		p.Stats.CaptureDecodeErrors++
		return nil
	}
	st := p.streams[key]
	if tcp.SYN && st != nil {
		delete(p.streams, key)
		st = nil
	}
	if st == nil {
		if len(tcp.Payload) == 0 && !tcp.SYN {
			return nil
		}
		if len(p.streams) >= 32 {
			p.Stats.ResourceDrops++
			return nil
		}
		p.next++
		st = &captureStream{tcp: tcpStreamState{id: p.next, baseSeq: tcp.Seq}, last: at, meta: meta, outbound: outbound, jpEnabled: (tcp.SrcPort == 16000 || tcp.DstPort == 16000) && (p.Stats.Profile == "jp-kr-legacy-probe" || p.Stats.Profile == "offline-legacy-probe")}
		if tcp.SYN {
			st.tcp.nextSeq = tcp.Seq + 1
			st.tcp.synced = true
		}
		p.streams[key] = st
	}
	st.last = at
	out := []RawDamage{}
	if len(tcp.Payload) > 0 {
		if !outbound {
			p.Stats.GamePayloads++
		}
		seq := tcp.Seq
		if tcp.SYN {
			seq++
		}
		st.tcp.push(seq, tcp.Payload, at, func(v gamePacketPayload) { out = append(out, p.collect(st, v)...) })
	}
	if st.meta >= 0 {
		p.Stats.Traffic[st.meta].PendingBytes = st.tcp.pendingBytes
	}
	if tcp.FIN || tcp.RST {
		st.tcp.flush(func(v gamePacketPayload) { out = append(out, p.collect(st, v)...) })
		delete(p.streams, key)
	}
	return out
}

func udpEndpointKey(address string, port uint16) string { return fmt.Sprintf("%s|%d", address, port) }
func (p *CapturePipeline) setUDPEndpoints(endpoints map[string]bool) {
	p.udpEndpoints = endpoints
	p.Stats.UDPEndpoints = len(endpoints)
}
