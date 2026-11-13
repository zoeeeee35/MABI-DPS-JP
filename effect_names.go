package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

const effectTranslationCSVName = "効果ID翻訳.csv"

type effectNameKey struct {
	Class string
	ID    uint32
}

var effectNamesOnce sync.Once
var effectNames = map[effectNameKey]string{}
var effectNamesStatus = "未配置（効果IDは未分類に表示）"

func parseEffectNames(data []byte) (map[effectNameKey]string, error) {
	if len(data) > 4<<20 || !utf8.Valid(data) {
		return nil, fmt.Errorf("CSV UTF-8・4MiB以内で保存してください")
	}
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte{239, 187, 191})))
	head, e := r.Read()
	if e != nil {
		return nil, e
	}
	cols := map[string]int{}
	for i, h := range head {
		h = strings.TrimSpace(h)
		if _, exists := cols[h]; exists {
			return nil, fmt.Errorf("列名が重複しています")
		}
		cols[h] = i
	}
	for _, h := range []string{"職業", "効果ID", "日本語訳"} {
		if _, ok := cols[h]; !ok {
			return nil, fmt.Errorf("%s列がありません", h)
		}
	}
	out := map[effectNameKey]string{}
	for line := 2; ; line++ {
		row, e := r.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		if line > 10001 {
			return nil, fmt.Errorf("行数上限")
		}
		class, id, label := strings.TrimSpace(row[cols["職業"]]), strings.TrimSpace(row[cols["効果ID"]]), strings.TrimSpace(row[cols["日本語訳"]])
		if label == "" {
			continue
		}
		for code, ja := range jpClassNames {
			if class == ja {
				class = code
				break
			}
		}
		if class != "*" && jpClassNames[class] == "" {
			return nil, fmt.Errorf("%d行目の職業を確認してください", line)
		}
		n, e := strconv.ParseUint(strings.TrimPrefix(strings.TrimPrefix(id, "0x"), "0X"), 16, 32)
		if e != nil || n == 0 {
			return nil, fmt.Errorf("%d行目の効果IDは非ゼロの16進数で指定してください", line)
		}
		if len(label) > 512 || strings.ContainsAny(label, "\r\n\x00") {
			return nil, fmt.Errorf("%d行目の訳が不正です", line)
		}
		key := effectNameKey{class, uint32(n)}
		if old, ok := out[key]; ok && old != label {
			return nil, fmt.Errorf("%d行目の対応が重複しています", line)
		}
		out[key] = label
	}
	return out, nil
}
func loadEffectNames() {
	identifierNamesOnce.Do(loadIdentifierNames)
	for k, v := range identifierNames {
		effectNames[k] = v
	}
	exe, e := os.Executable()
	if e != nil {
		return
	}
	f, e := os.Open(filepath.Join(filepath.Dir(exe), effectTranslationCSVName))
	if os.IsNotExist(e) {
		return
	}
	if e != nil {
		effectNamesStatus = "読込失敗"
		return
	}
	defer f.Close()
	data, e := io.ReadAll(io.LimitReader(f, (4<<20)+1))
	if e != nil {
		effectNamesStatus = "読込失敗"
		return
	}
	names, e := parseEffectNames(data)
	if e != nil {
		effectNamesStatus = "読込失敗：" + e.Error()
		return
	}
	for k, v := range names {
		effectNames[k] = v
	}
	effectNamesStatus = fmt.Sprintf("%d件読込", len(names))
}
func effectActorClass(log CombatLog, actor string) string {
	for _, c := range []*PartyConfig{log.Party, log.Detected} {
		if c != nil {
			if e, ok := c.Entities[actor]; ok && e.ClassFamily != "" && e.Evidence != "conflicting-skill-prefixes" {
				return e.ClassFamily
			}
		}
	}
	found := ""
	for _, u := range log.SkillUses {
		if u.ActorID != actor {
			continue
		}
		for code := range jpClassNames {
			if strings.HasPrefix(u.Code, code+"_") {
				if found != "" && found != code {
					return ""
				}
				found = code
			}
		}
	}
	return found
}
func effectName(log CombatLog, h Hit, names map[effectNameKey]string) string {
	if h.Link == nil || !h.Link.Paired || h.Link.Effect == 0 {
		return ""
	}
	identifierNamesOnce.Do(loadIdentifierNames)
	if label := identifierNames[effectNameKey{"*", h.Link.Effect}]; label != "" {
		return label
	}
	if label := names[effectNameKey{effectActorClass(log, h.PlayerID), h.Link.Effect}]; label != "" {
		return label
	}
	return names[effectNameKey{"*", h.Link.Effect}]
}
func summarizeEffectRow(key, name string, events []Hit, total int64) SkillDamageRow {
	r := SkillDamageRow{Key: key, Name: name, events: events}
	for _, h := range events {
		r.Hits++
		r.Damage += h.Damage
		if r.Hits == 1 || h.Damage < r.Min {
			r.Min = h.Damage
		}
		if h.Damage > r.Max {
			r.Max = h.Damage
		}
	}
	if r.Hits > 0 {
		r.Mean = float64(r.Damage) / float64(r.Hits)
	}
	if total > 0 {
		r.Share = float64(r.Damage) / float64(total)
	}
	r.Flags = damageFlagStatistics(events, "")
	return r
}

// Display classification only; original Hit, Battle and DPS statistics are immutable.
func classifyUnknownEffectRows(rows []SkillDamageRow, log CombatLog, names map[effectNameKey]string) []SkillDamageRow {
	var total int64
	for _, r := range rows {
		total += r.Damage
	}
	out := []SkillDamageRow{}
	for _, r := range rows {
		if r.Key != "unknown" {
			out = append(out, r)
			continue
		}
		pending := map[string][]Hit{}
		labeled := map[string][]Hit{}
		labels := map[string]string{}
		remaining := []Hit{}
		for _, h := range r.events {
			if label := effectName(log, h, names); label != "" {
				key := fmt.Sprintf("effect:%s:%08X", effectActorClass(log, h.PlayerID), h.Link.Effect)
				labeled[key] = append(labeled[key], h)
				labels[key] = label
				continue
			}
			key := "効果ID未取得"
			if h.Link != nil && h.Link.Paired {
				key = fmt.Sprintf("0x%08X", h.Link.Effect)
			}
			pending[key] = append(pending[key], h)
			remaining = append(remaining, h)
		}
		for key, events := range labeled {
			out = append(out, summarizeEffectRow(key, labels[key], events, total))
		}
		if len(remaining) > 0 {
			parent := summarizeEffectRow("unknown", "未分類", remaining, total)
			for key, events := range pending {
				parent.Children = append(parent.Children, summarizeEffectRow("unknown-child:"+key, key, events, total))
			}
			sortLinkedSkills(parent.Children, 0, false)
			out = append(out, parent)
		}
	}
	return out
}
func expandUnknownEffectRows(rows []SkillDamageRow, expanded bool) []SkillDamageRow {
	out := []SkillDamageRow{}
	for _, r := range rows {
		out = append(out, r)
		if expanded && r.Key == "unknown" {
			out = append(out, r.Children...)
		}
	}
	return out
}
