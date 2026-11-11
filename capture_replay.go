package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
)

// Offline replay requires an explicit source port; unlike live capture,
// a file has no OS process ownership table. No live port is assumed.
func replayPCAP(input io.Reader, serverPort uint16) (CaptureStats, error) {
	return replayPCAPObserved(input, serverPort, nil)
}
func replayPCAPObserved(input io.Reader, serverPort uint16, observer func(uint32, []byte, bool)) (CaptureStats, error) {
	return replayPCAPAudited(input, serverPort, observer, nil)
}
func replayPCAPAudited(input io.Reader, serverPort uint16, observer func(uint32, []byte, bool), audit func(int, uint64, time.Time, uint32, []byte, bool)) (CaptureStats, error) {
	if serverPort == 0 {
		return CaptureStats{}, fmt.Errorf("PCAP再生では対象サーバーポートを明示してください")
	}
	source := bufio.NewReader(io.LimitReader(input, 64<<20))
	magic, e := source.Peek(4)
	if e != nil {
		return CaptureStats{}, fmt.Errorf("キャプチャヘッダーを読めません")
	}
	var reader interface {
		ReadPacketData() ([]byte, gopacket.CaptureInfo, error)
		LinkType() layers.LinkType
	}
	if bytes.Equal(magic, []byte{0x0a, 0x0d, 0x0d, 0x0a}) {
		ng, err := pcapgo.NewNgReader(source, pcapgo.DefaultNgReaderOptions)
		if err != nil {
			return CaptureStats{}, fmt.Errorf("PCAPNGを読めません")
		}
		reader = ng
	} else {
		classic, err := pcapgo.NewReader(source)
		if err != nil {
			return CaptureStats{}, fmt.Errorf("PCAP/PCAPNGを読めません")
		}
		classic.SetSnaplen(1 << 20)
		reader = classic
	}
	pipe := newPipeline()
	pipe.jpFrameObserver = observer
	pipe.jpFrameAudit = audit
	pipe.offline = &offlineCollector{}
	pipe.Stats.Profile = "offline-legacy-probe"
	flows := map[string]bool{}
	var provisional *JPProvisionalInput
	var lastAt time.Time
	drain := func() error {
		if provisional == nil && len(pipe.jpQueue) > 0 {
			var e error
			provisional, e = newJPProvisionalInput(pipe.jpQueue[0].At, 5000)
			if e != nil {
				return e
			}
		}
		for _, v := range pipe.jpQueue {
			if _, e := provisional.append(v); e != nil {
				return e
			}
		}
		pipe.jpQueue = nil
		if provisional != nil {
			return provisional.tick(lastAt)
		}
		return nil
	}
	for i := 0; i < 1000000; i++ {
		data, ci, e := reader.ReadPacketData()
		if e == io.EOF {
			for _, st := range pipe.streams {
				st.tcp.flush(func(v gamePacketPayload) { _ = pipe.collect(st, v) })
			}
			if e := drain(); e != nil {
				return pipe.Stats, e
			}
			if provisional != nil {
				if e := provisional.tick(lastAt.Add(5 * time.Second)); e != nil {
					return pipe.Stats, e
				}
				s := provisional.Live.Session
				if s != nil {
					pipe.Stats.Provisional = &s.Result
					pipe.Stats.provisionalLog = &s.Log
					pipe.Stats.replayAliases = map[uint64]string{}
					rawIDs := map[uint64]bool{}
					for raw := range pipe.jpActors {
						rawIDs[raw] = true
					}
					for _, st := range pipe.streams {
						for raw := range st.jp.skillActors {
							rawIDs[raw] = true
						}
					}
					for raw := range rawIDs {
						var b [8]byte
						binary.LittleEndian.PutUint64(b[:], raw)
						token := jpToken(pipe.jpKey, "entity", b[:])
						pipe.Stats.replayAliases[raw] = provisional.Live.alias("actor", token)
					}
				}
			}
			pipe.Stats.Running = false
			pipe.Stats.Stage = "replay_complete"
			pipe.Stats.Offline = pipe.offline.report(pipe)
			pipe.Stats.Identity = pipe.offline.identityProbe(pipe)
			return pipe.Stats, nil
		}
		if e != nil {
			return pipe.Stats, fmt.Errorf("PCAPの記録が不正・切断、または上限超過です")
		}
		if pipe.Stats.FirstPacket.IsZero() {
			pipe.Stats.FirstPacket = ci.Timestamp
		}
		pipe.Stats.LastPacket = ci.Timestamp
		packet := gopacket.NewPacket(data, reader.LinkType(), gopacket.DecodeOptions{Lazy: false, NoCopy: true})
		tcpLayer := packet.Layer(layers.LayerTypeTCP)
		if tcpLayer != nil {
			tcp := tcpLayer.(*layers.TCP)
			if uint16(tcp.SrcPort) == serverPort || uint16(tcp.DstPort) == serverPort {
				network := packet.NetworkLayer()
				if network != nil {
					src, dst := network.NetworkFlow().Endpoints()
					if len(flows) < 128 {
						if uint16(tcp.SrcPort) == serverPort {
							flows[flowKey(src.String(), uint16(tcp.SrcPort), dst.String(), uint16(tcp.DstPort))] = true
						} else {
							flows[flowKey(dst.String(), uint16(tcp.DstPort), src.String(), uint16(tcp.SrcPort))] = true
						}
					}
				}
			}
		}
		pipe.setFlows(flows)
		_ = pipe.feed(data, reader.LinkType(), ci.Timestamp)
		_ = pipe.expire(ci.Timestamp)
		lastAt = ci.Timestamp
		if e := drain(); e != nil {
			return pipe.Stats, e
		}
	}
	pipe.Stats.ResourceDrops++
	for _, st := range pipe.streams {
		st.tcp.flush(func(v gamePacketPayload) { _ = pipe.collect(st, v) })
	}
	if e := drain(); e != nil {
		return pipe.Stats, e
	}
	if provisional != nil {
		if e := provisional.tick(lastAt.Add(5 * time.Second)); e != nil {
			return pipe.Stats, e
		}
		s := provisional.Live.Session
		if s != nil {
			pipe.Stats.Provisional = &s.Result
			pipe.Stats.provisionalLog = &s.Log
			pipe.Stats.replayAliases = map[uint64]string{}
			rawIDs := map[uint64]bool{}
			for raw := range pipe.jpActors {
				rawIDs[raw] = true
			}
			for _, st := range pipe.streams {
				for raw := range st.jp.skillActors {
					rawIDs[raw] = true
				}
			}
			for raw := range rawIDs {
				var b [8]byte
				binary.LittleEndian.PutUint64(b[:], raw)
				token := jpToken(pipe.jpKey, "entity", b[:])
				pipe.Stats.replayAliases[raw] = provisional.Live.alias("actor", token)
			}
		}
	}
	pipe.Stats.Running = false
	pipe.Stats.Stage = "resource_limit"
	return pipe.Stats, fmt.Errorf("PCAPのパケット上限に達しました")
}
