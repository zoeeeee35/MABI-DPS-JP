package main

import (
	"encoding/json"
	"fmt"
	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/gopacket/gopacket/pcapgo"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const recordingLimit int64 = 16 << 20

type RecordingStats struct {
	Manual  bool      `json:"manualStopMode"`
	Active  bool      `json:"active"`
	Path    string    `json:"path"`
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
	Packets uint64    `json:"packets"`
	Bytes   int64     `json:"bytes"`
	Reason  string    `json:"reason"`
}
type localRecorder struct {
	file   *os.File
	writer *pcapgo.NgWriter
	flows  map[string]bool
	link   layers.LinkType
	stats  RecordingStats
}

func gamePacketDirection(data []byte, link layers.LinkType, flows map[string]bool) (string, bool) {
	p := gopacket.NewPacket(data, link, gopacket.Default)
	n := p.NetworkLayer()
	l := p.Layer(layers.LayerTypeTCP)
	if n == nil || l == nil || p.ErrorLayer() != nil {
		return "", false
	}
	t := l.(*layers.TCP)
	src, dst := n.NetworkFlow().Endpoints()
	if t.SrcPort != 16000 && t.DstPort != 16000 {
		return "", false
	}
	if flows[flowKey(src.String(), uint16(t.SrcPort), dst.String(), uint16(t.DstPort))] {
		return "receive", true
	}
	if flows[flowKey(dst.String(), uint16(t.DstPort), src.String(), uint16(t.SrcPort))] {
		return "send", true
	}
	return "", false
}
func openLocalRecorder(dir, label string, link layers.LinkType, flows map[string]bool, now time.Time) (*localRecorder, error) {
	names := map[string]string{"idle": "idle.pcapng", "normal_attack": "normal_attack.pcapng", "skill_attack": "skill_attack.pcapng"}
	name, ok := names[label]
	if label == "manual" {
		name = fmt.Sprintf("classification-%s-%09d.pcapng", now.Format("20060102-150405"), now.Nanosecond())
		ok = true
	}
	if !ok || len(flows) == 0 {
		return nil, fmt.Errorf("ゲーム接続または記録条件を確認してください")
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, fmt.Errorf("記録フォルダーを作成できません")
	}
	f, e := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return nil, fmt.Errorf("同名記録が存在するか、記録先に書き込めません")
	}
	w, e := pcapgo.NewNgWriterInterface(f, pcapgo.NgInterface{Name: "game-owned-tcp16000", LinkType: link, SnapLength: 65535, TimestampResolution: 9}, pcapgo.NgWriterOptions{})
	if e != nil {
		f.Close()
		return nil, fmt.Errorf("PCAPNGヘッダーを書き込めません")
	}
	allowed := map[string]bool{}
	for k, v := range flows {
		allowed[k] = v
	}
	r := &localRecorder{f, w, allowed, link, RecordingStats{Manual: label == "manual", Active: true, Path: f.Name(), Start: now}}
	if e = w.Flush(); e != nil {
		f.Close()
		return nil, e
	}
	return r, nil
}
func (r *localRecorder) write(data []byte, ci gopacket.CaptureInfo) error {
	if !r.stats.Active {
		return nil
	}
	if ci.Timestamp.Before(r.stats.Start) || (!r.stats.Manual && ci.Timestamp.After(r.stats.Start.Add(30*time.Second))) {
		return nil
	}
	direction, ok := gamePacketDirection(data, r.link, r.flows)
	if !ok {
		return nil
	}
	limit := recordingLimit
	if r.stats.Manual {
		limit = 64 << 20
	}
	if r.stats.Bytes+int64(len(data))+512 >= limit {
		return r.close("capacity_limit")
	}
	flag := pcapgo.NgEpbFlagDirectionInbound
	if direction == "send" {
		flag = pcapgo.NgEpbFlagDirectionOutbound
	}
	ci.InterfaceIndex = 0
	if e := r.writer.WritePacketWithOptions(ci, data, pcapgo.NgPacketOptions{Flags: &pcapgo.NgEpbFlags{Direction: flag}}); e != nil {
		return e
	}
	if e := r.writer.Flush(); e != nil {
		return e
	}
	info, e := r.file.Stat()
	if e != nil {
		return e
	}
	r.stats.Bytes = info.Size()
	r.stats.Packets++
	return nil
}
func (r *localRecorder) close(reason string) error {
	if !r.stats.Active {
		return nil
	}
	r.stats.Active = false
	r.stats.End = time.Now()
	r.stats.Reason = reason
	e := r.writer.Flush()
	if e == nil {
		e = r.file.Sync()
	}
	if c := r.file.Close(); e == nil {
		e = c
	}
	if e != nil {
		r.stats.Reason = "finalize_error"
	}
	b, _ := json.MarshalIndent(r.stats, "", "  ")
	if c := os.WriteFile(r.stats.Path+".json", b, 0600); e == nil {
		e = c
	}
	return e
}

// Restrict deletion to the three owned recording filenames and their metadata.
func deleteLocalRecordings(dir string) error {
	names := []string{"idle.pcapng", "normal_attack.pcapng", "skill_attack.pcapng"}
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	pattern := regexp.MustCompile(`^classification-\d{8}-\d{6}-\d{9}\.pcapng$`)
	for _, entry := range entries {
		if pattern.MatchString(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	for _, name := range names {
		for _, suffix := range []string{"", ".json"} {
			path := filepath.Join(dir, name+suffix)
			info, e := os.Lstat(path)
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return e
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("通常ファイル以外は削除しません")
			}
			if e = os.Remove(path); e != nil {
				return e
			}
		}
	}
	return nil
}
