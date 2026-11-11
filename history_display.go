package main

import (
	"encoding/json"
	"fmt"
	"os"

	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

func historyClassLabel(h SavedBattle) string {
	normalize := func(s string) string {
		if v := jpClassNames[s]; v != "" {
			return v
		}
		if s == "unknown" || s == "" {
			return "職業不明"
		}
		return s
	}
	if c := h.Archive.Request.Log.Party; c != nil {
		for _, e := range c.Entities {
			if e.Role == RoleSelf {
				label := e.Class
				if label == "" {
					label = e.ClassFamily
				}
				return normalize(label)
			}
		}
	}
	classes := []string{}
	seen := map[string]bool{}
	for _, p := range h.Battle.Players {
		s := normalize(p.ClassID)
		if !seen[s] {
			seen[s] = true
			classes = append(classes, s)
		}
	}
	sort.Strings(classes)
	if len(classes) == 0 {
		return "職業不明"
	}
	return strings.Join(classes, " / ")
}
func historyDisplayTitle(h SavedBattle) string {
	date := "日時不明"
	if t, captured := historyRecordTime(h); !t.IsZero() {
		date = t.In(time.FixedZone("JST", 9*3600)).Format("2006/01/02 15:04:05")
		if !captured {
			date += "（保存）"
		}
	}
	kind := "戦闘時間"
	if h.Archive.SegmentReason != "" {
		kind = "自動保存区間"
	}
	if h.Archive.Request.Window != nil {
		kind = "時間指定"
	}
	return fmt.Sprintf("%s    %s    %s %.1f秒", date, historyClassLabel(h), kind, h.Battle.Duration)
}
func historyDisplayDetail(h SavedBattle) string {
	place := "場所未取得"
	if h.Archive.LocationNote != "" {
		place = "場所メモ：" + h.Archive.LocationNote
	}
	return fmt.Sprintf("%s    累計 %s    DPS %s", place, formatNum(float64(h.Battle.Damage)), formatNum(float64(h.Battle.Damage)/max(1, h.Battle.Duration)))
}
func updateHistoryLocation(path, note string) error {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > 80 || strings.ContainsAny(note, "\r\n") {
		return fmt.Errorf("場所メモは改行なしの80文字以内にしてください")
	}
	if _, e := loadArchive(path); e != nil {
		return e
	}
	data, e := json.Marshal(struct {
		Version int    `json:"version"`
		Note    string `json:"note"`
	}{1, note})
	if e != nil {
		return e
	}
	return os.WriteFile(path+".location", data, 0600)
}
func readHistoryLocation(path string) (string, error) {
	data, e := os.ReadFile(path + ".location")
	if e != nil {
		return "", e
	}
	if len(data) > 2048 {
		return "", fmt.Errorf("場所メモのサイズが不正です")
	}
	var v struct {
		Version int    `json:"version"`
		Note    string `json:"note"`
	}
	if e = json.Unmarshal(data, &v); e != nil {
		return "", e
	}
	if v.Version != 1 || utf8.RuneCountInString(v.Note) > 80 || strings.ContainsAny(v.Note, "\r\n") {
		return "", fmt.Errorf("場所メモが不正です")
	}
	return v.Note, nil
}

func historyRecordTime(h SavedBattle) (time.Time, bool) {
	for _, e := range h.Battle.DamageEvents {
		if e.Flags != nil && !e.Flags.Timestamp.IsZero() {
			return e.Flags.Timestamp.Add(time.Duration(h.Battle.Start-e.At) * time.Millisecond), true
		}
	}
	t, _ := time.Parse(time.RFC3339, h.Archive.CreatedAt)
	return t, false
}
