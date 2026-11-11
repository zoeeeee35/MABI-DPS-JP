package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Session struct {
	Log     CombatLog
	Cursor  int64
	Idle    int64
	Speed   int64
	Playing bool
	Result  Result
}

func newSession(l CombatLog, idle int64) (*Session, error) {
	s := &Session{Log: l, Idle: idle, Speed: 1}
	if err := s.recompute(); err != nil {
		return nil, err
	}
	return s, nil
}
func (s *Session) endTime() int64 {
	var last int64
	if len(s.Log.Events) > 0 {
		last = s.Log.Events[len(s.Log.Events)-1].At
	}
	if last+s.Idle > maxTime {
		return maxTime
	}
	return last + s.Idle
}
func (s *Session) recompute() error {
	applySkillLinks(&s.Log)
	r, e := analyze(Request{Log: s.Log, Until: s.Cursor, Idle: s.Idle})
	if e == nil {
		s.Result = r
	}
	return e
}
func (s *Session) advance(ms int64) error {
	if !s.Playing {
		return nil
	}
	if ms < 0 {
		return fmt.Errorf("再生時間が不正です")
	}
	s.Cursor += ms * s.Speed
	if s.Cursor >= s.endTime() {
		s.Cursor = s.endTime()
		s.Playing = false
	}
	return s.recompute()
}
func (s *Session) current() *Battle {
	if len(s.Result.Battles) == 0 {
		return nil
	}
	return &s.Result.Battles[len(s.Result.Battles)-1]
}
func readLog(path string) (CombatLog, error) {
	var l CombatLog
	f, e := os.Open(path)
	if e != nil {
		return l, fmt.Errorf("戦闘データを開けません")
	}
	defer f.Close()
	stat, e := f.Stat()
	if e != nil || stat.Size() > 8<<20 {
		return l, fmt.Errorf("戦闘データは8MB以下にしてください")
	}
	data, e := readBoundedFile(path, 8<<20)
	if e != nil {
		return l, e
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if e = decode(bytes.NewReader(data), &l); e != nil {
		return l, e
	}
	e = validate(Request{Log: l, Until: 0, Idle: 5000})
	return l, e
}

type Archive struct {
	SegmentReason string  `json:"segmentReason,omitempty"`
	LocationNote  string  `json:"locationNote,omitempty"`
	Version       int     `json:"version"`
	CreatedAt     string  `json:"createdAt"`
	Request       Request `json:"request"`
	BattleNumber  int     `json:"battleNumber"`
	LiveVerified  bool    `json:"liveVerified"`
}
type SavedBattle struct {
	Path    string
	Archive Archive
	Battle  Battle
}

func archiveKey(q Request, b Battle) string {
	data, _ := json.Marshal(struct {
		Region, Source string
		Idle           int64
		Battle         Battle
	}{q.Log.Region, q.Log.Source, q.Idle, b})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:12])
}
func saveBattle(dir string, q Request, b Battle) (string, error) {
	return saveBattleSegment(dir, q, b, "")
}
func saveBattleSegment(dir string, q Request, b Battle, segmentReason string) (string, error) {
	q = trimMeasurementRequest(q)
	if b.Active {
		return "", fmt.Errorf("戦闘が終了していません")
	}
	if e := os.MkdirAll(dir, 0700); e != nil {
		return "", fmt.Errorf("履歴フォルダーを作成できません: %w", e)
	}
	path := filepath.Join(dir, archiveKey(q, b)+".json")
	if _, e := os.Stat(path); e == nil {
		return path, nil
	}
	a := Archive{SegmentReason: segmentReason, Version: 1, CreatedAt: time.Now().UTC().Format(time.RFC3339), Request: q, BattleNumber: b.Number}
	data, e := json.MarshalIndent(a, "", "  ")
	if e != nil {
		return "", e
	}
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(e) {
		return path, nil
	}
	if e != nil {
		return "", fmt.Errorf("履歴を保存できません: %w", e)
	}
	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(path)
		}
	}()
	if _, e = f.Write(data); e != nil {
		return "", e
	}
	if e = f.Sync(); e != nil {
		return "", e
	}
	if e = f.Close(); e != nil {
		return "", e
	}
	ok = true
	return path, nil
}
func loadArchive(path string) (SavedBattle, error) {
	var v SavedBattle
	v.Path = path
	data, e := readBoundedFile(path, 8<<20)
	if e != nil {
		return v, e
	}
	if len(data) > 8<<20 {
		return v, fmt.Errorf("履歴のサイズが上限を超えています")
	}
	if e = decode(bytes.NewReader(data), &v.Archive); e != nil {
		return v, e
	}
	a := v.Archive
	if a.Version != 1 || a.LiveVerified || a.BattleNumber < 1 {
		return v, fmt.Errorf("未対応の履歴です")
	}
	if _, e = time.Parse(time.RFC3339, a.CreatedAt); e != nil {
		return v, fmt.Errorf("履歴の日時が不正です")
	}
	r, e := analyze(a.Request)
	if e != nil {
		return v, e
	}
	if a.BattleNumber > len(r.Battles) || r.Battles[a.BattleNumber-1].Active {
		return v, fmt.Errorf("履歴の戦闘が完了していません")
	}
	v.Battle = r.Battles[a.BattleNumber-1]
	if note, e := readHistoryLocation(path); e == nil {
		v.Archive.LocationNote = note
	}
	return v, nil
}
func loadHistory(dir string) ([]SavedBattle, int) {
	entries, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	sort.Strings(entries)
	out := []SavedBattle{}
	bad := 0
	for _, path := range entries {
		if len(out) >= 500 {
			break
		}
		v, e := loadArchive(path)
		if e != nil {
			bad++
			continue
		}
		out = append(out, v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, _ := historyRecordTime(out[i])
		b, _ := historyRecordTime(out[j])
		if a.Equal(b) {
			return out[i].Archive.CreatedAt > out[j].Archive.CreatedAt
		}
		return a.After(b)
	})
	return out, bad
}
