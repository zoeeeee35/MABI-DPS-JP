//go:build native && windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gopacket/gopacket/pcap"
)

type CaptureDevice struct {
	Name      string
	Label     string
	Suggested bool
}

func captureDevices() ([]CaptureDevice, error) {
	if e := pcap.LoadWinPCAP(); e != nil {
		return nil, fmt.Errorf("NpcapのDLLを読み込めません。サンプル再生は利用できます")
	}
	all, e := pcap.FindAllDevs()
	if e != nil {
		return nil, fmt.Errorf("Npcapからアダプター一覧を取得できません")
	}
	devices := []CaptureDevice{}
	profile, _ := profileFor("jp")
	flows, _ := gameConnections(profile)
	for _, v := range all {
		label := v.Description
		if label == "" {
			label = fmt.Sprintf("ネットワークアダプター%d", len(devices)+1)
		}
		suggested := false
		for _, addr := range v.Addresses {
			for key := range flows {
				if strings.Contains(key, ">"+addr.IP.String()+"|") {
					suggested = true
				}
			}
		}
		if suggested {
			label += "（ゲーム接続あり）"
		}
		devices = append(devices, CaptureDevice{Name: v.Name, Label: label, Suggested: suggested})
	}
	return devices, nil
}

type CaptureController struct {
	mu             sync.Mutex
	stats          CaptureStats
	events         chan RawDamage
	jpEvents       chan JPValue
	cancel         context.CancelFunc
	done           chan struct{}
	resetProbe     chan chan CaptureStats
	recordCommands chan recordCommand
}

func (c *CaptureController) snapshot() CaptureStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := cloneStats(c.stats)
	s.Stage = diagnosticStage(s)
	return s
}
func (c *CaptureController) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
		c.stats.Stage = "stopping"
	}
}
func (c *CaptureController) running() bool { c.mu.Lock(); defer c.mu.Unlock(); return c.stats.Running }
func (c *CaptureController) start(device CaptureDevice, profile CaptureProfile) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stats.Running {
		return fmt.Errorf("キャプチャが停止するまでお待ちください")
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.done = make(chan struct{})
	c.events = make(chan RawDamage, 4096)
	c.jpEvents = make(chan JPValue, 4096)
	c.resetProbe = make(chan chan CaptureStats, 1)
	c.recordCommands = make(chan recordCommand, 1)
	c.stats = CaptureStats{Running: true, Stage: "waiting_packets"}
	go c.loop(ctx, device, profile)
	return nil
}
func captureErrorStage(err error) string {
	s := strings.ToLower(err.Error())
	if strings.Contains(s, "access") || strings.Contains(s, "permission") || strings.Contains(s, "denied") {
		return "permission_denied"
	}
	return "adapter_unavailable"
}
func (c *CaptureController) loop(ctx context.Context, device CaptureDevice, profile CaptureProfile) {
	pipe := newPipeline()
	pipe.Stats.Profile = profile.ID
	defer func() {
		c.mu.Lock()
		c.stats.Running = false
		close(c.done)
		if c.stats.Stage == "stopping" || c.stats.Stage == "" {
			c.stats.Stage = "stopped"
		}
		c.mu.Unlock()
	}()
	fail := func(stage, detail string) { c.mu.Lock(); c.stats.Stage = stage; c.stats.Detail = detail; c.mu.Unlock() }
	if e := pcap.LoadWinPCAP(); e != nil {
		fail("driver_missing", "Npcapの導入状態を確認してください")
		return
	}
	h, e := pcap.OpenLive(device.Name, 65535, false, 100*time.Millisecond)
	if e != nil {
		stage := captureErrorStage(e)
		fail(stage, stageText(stage)+"。別アダプターまたはNpcapの権限設定を確認してください")
		return
	}
	defer h.Close()
	// Capture copies only. No send/reinject API is used. Nonmatching TCP data
	// is discarded before application-payload decoding; no raw packet file exists.
	if e = h.SetBPFFilter("tcp or udp"); e != nil {
		fail("capture_error", "TCP/UDP取得フィルターを設定できません")
		return
	}
	var recorder *localRecorder
	defer func() {
		if recorder != nil {
			_ = recorder.close("capture_stopped")
			pipe.Stats.Recording = recorder.stats
			publishFinal(c, pipe.Stats)
		}
	}()
	lastRefresh := time.Time{}
	lastStats := time.Time{}
	publish := func() {
		snapshot := cloneStats(pipe.Stats)
		snapshot.Running = true
		snapshot.Stage = diagnosticStage(snapshot)
		c.mu.Lock()
		if c.stats.Stage != "stopping" {
			snapshot.Aggregated = c.stats.Aggregated
			c.stats = snapshot
		}
		c.mu.Unlock()
	}
	emit := func(events []RawDamage) {
		for _, v := range events {
			select {
			case c.events <- v:
			default:
				pipe.Stats.QueueDrops++
			}
		}
	}
	for {
		select {
		case command := <-c.recordCommands:
			if rawCaptureTools {
				var err error
				if command.label == "stop" {
					if recorder != nil {
						err = recorder.close("manual_stop")
					}
				} else if recorder != nil && recorder.stats.Active {
					err = fmt.Errorf("記録中です。先に保存停止してください")
				} else {
					flows, checkErr := gameConnections(profile)
					if checkErr != nil {
						err = fmt.Errorf("ゲーム所有接続を再確認できません")
					} else {
						recorder, err = openLocalRecorder(command.dir, command.label, h.LinkType(), flows, time.Now())
					}
				}
				if recorder != nil {
					pipe.Stats.Recording = recorder.stats
				}
				publish()
				command.reply <- err
			}
		case reply := <-c.resetProbe:
			pipe.resetStructureWindow()
			publish()
			reply <- cloneStats(pipe.Stats)
		case <-ctx.Done():
			fail("stopped", "停止しました（パケット内容は保存していません）")
			publishFinal(c, pipe.Stats)
			return
		default:
		}
		now := time.Now()
		if now.Sub(lastRefresh) >= time.Second {
			flows, err := gameConnections(profile)
			pipe.setFlows(flows)
			if recorder != nil && recorder.stats.Active && recorder.stats.Manual {
				recorder.flows = pipe.flows
			}
			udpEndpoints, udpErr := gameUDPEndpoints(profile)
			pipe.setUDPEndpoints(udpEndpoints)
			if err != nil {
				pipe.Stats.Detail = err.Error()
			} else {
				pipe.Stats.Detail = ""
				if udpErr != nil {
					pipe.Stats.Detail = udpErr.Error()
				}
			}
			lastRefresh = now
		}
		data, ci, err := h.ReadPacketData()
		if err == nil {
			if recorder != nil && recorder.stats.Active {
				if _, owned := gamePacketDirection(data, h.LinkType(), pipe.flows); owned {
					if e := recorder.write(data, ci); e != nil {
						_ = recorder.close("write_error")
						pipe.Stats.Detail = "PCAPNG書込に失敗しました"
					}
				}
			}
			emit(pipe.feed(data, h.LinkType(), ci.Timestamp))
		} else if err != pcap.NextErrorTimeoutExpired {
			fail("capture_error", "Npcapの読取処理が終了しました")
			return
		}
		if recorder != nil {
			if recorder.stats.Active && !recorder.stats.Manual && time.Since(recorder.stats.Start) >= 30*time.Second {
				_ = recorder.close("duration_complete")
			}
			pipe.Stats.Recording = recorder.stats
		}
		emit(pipe.expire(now))
		for _, value := range pipe.jpQueue {
			select {
			case c.jpEvents <- value:
			default:
				pipe.Stats.QueueDrops++
			}
		}
		pipe.jpQueue = nil
		if now.Sub(lastStats) >= 200*time.Millisecond {
			if stats, err := h.Stats(); err == nil {
				pipe.Stats.DriverDrops = uint64(stats.PacketsDropped)
			}
			publish()
			lastStats = now
		}
	}
}
func publishFinal(c *CaptureController, s CaptureStats) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s.Aggregated = c.stats.Aggregated
	s.Running = false
	s.Stage = "stopped"
	s.Detail = "キャプチャ停止"
	if c.stats.Stage == "capture_error" {
		s.Stage = c.stats.Stage
		s.Detail = c.stats.Detail
	}
	c.stats = cloneStats(s)
}
func runCaptureProbe(path string, adapter int, seconds int, region string) error {
	devices, e := captureDevices()
	if e != nil {
		return e
	}
	if adapter == -1 {
		adapter = 0
		for i, d := range devices {
			if d.Suggested {
				adapter = i
				break
			}
		}
	}
	if adapter < 0 || adapter >= len(devices) {
		return fmt.Errorf("アダプター番号は0〜%dです", len(devices)-1)
	}
	profile, e := profileFor(region)
	if e != nil {
		return e
	}
	c := &CaptureController{}
	if e = c.start(devices[adapter], profile); e != nil {
		return e
	}
	defer c.stop()
	deadline := time.After(time.Duration(seconds) * time.Second)
	var last CaptureStats
loop:
	for {
		select {
		case <-deadline:
			break loop
		case <-time.After(100 * time.Millisecond):
			last = c.snapshot()
			if !last.Running {
				break loop
			}
		case <-c.events:
		}
	}
	c.stop()
	select {
	case <-c.done:
	case <-time.After(2 * time.Second):
	}
	payload := map[string]any{"profile": profile.ID, "adapterIndex": adapter, "statsBeforeStop": last, "finalStats": c.snapshot(), "liveVerified": false, "rawPacketsSaved": false}
	data, _ := json.MarshalIndent(payload, "", "  ")
	return os.WriteFile(path, data, 0600)
}

func runCaptureInventory(path string) error {
	devices, e := captureDevices()
	if e != nil {
		return e
	}
	items := []map[string]any{}
	for i, d := range devices {
		items = append(items, map[string]any{"index": i, "description": d.Label, "suggested": d.Suggested})
	}
	profile, _ := profileFor("jp")
	flows, e := gameConnections(profile)
	detail := ""
	if e != nil {
		detail = e.Error()
	}
	data, _ := json.MarshalIndent(map[string]any{"devices": items, "gameConnectionCount": len(flows), "detail": detail, "packetsCaptured": false}, "", "  ")
	return os.WriteFile(path, data, 0600)
}

func (c *CaptureController) beginStructureWindow() (CaptureStats, error) {
	c.mu.Lock()
	running := c.stats.Running
	ch := c.resetProbe
	c.mu.Unlock()
	if !running {
		return CaptureStats{}, fmt.Errorf("キャプチャは停止しています")
	}
	reply := make(chan CaptureStats, 1)
	select {
	case ch <- reply:
	default:
		return CaptureStats{}, fmt.Errorf("診断切替待ちです")
	}
	select {
	case s := <-reply:
		return cloneStats(s), nil
	case <-time.After(2 * time.Second):
		return CaptureStats{}, fmt.Errorf("構造診断の切替がタイムアウトしました")
	}
}

type recordCommand struct {
	dir, label string
	reply      chan error
}

func (c *CaptureController) record(dir, label string) error {
	if !rawCaptureTools {
		return fmt.Errorf("このビルドでは通信の記録を利用できません")
	}
	c.mu.Lock()
	running := c.stats.Running
	ch := c.recordCommands
	c.mu.Unlock()
	if !running {
		return fmt.Errorf("先にキャプチャを開始してください")
	}
	command := recordCommand{dir, label, make(chan error, 1)}
	select {
	case ch <- command:
	default:
		return fmt.Errorf("保存切替待ちです")
	}
	select {
	case e := <-command.reply:
		return e
	case <-time.After(2 * time.Second):
		return fmt.Errorf("保存切替タイムアウト。状態を確認してください")
	}
}
