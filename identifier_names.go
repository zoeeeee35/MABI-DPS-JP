package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Optional local display dictionary. It never changes event or skill identity.
const identifierTranslationCSVName = "識別子翻訳.csv"

var identifierNamesOnce sync.Once
var identifierNames = map[effectNameKey]string{}
var identifierNamesStatus = "ID辞書未配置"

func loadIdentifierNames() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	data, err := readBoundedFile(filepath.Join(filepath.Dir(exe), identifierTranslationCSVName), 4<<20)
	if err != nil {
		if !os.IsNotExist(err) {
			identifierNamesStatus = "ID辞書を読み込めません"
		}
		return
	}
	names, err := parseEffectNames(data)
	if err == nil {
		identifierNames = names
		identifierNamesStatus = fmt.Sprintf("ID辞書 %d件読込", len(names))
	} else {
		identifierNamesStatus = "ID辞書の形式が不正です"
	}
}

// Learn display aliases only from explicit skill/effect associations belonging
// to this actor in this log. Never use timing proximity or persist session IDs.
func identifierSkillAliases(log CombatLog, actor string, names map[effectNameKey]string) map[string]string {
	out := map[string]string{}
	conflicts := map[string]bool{}
	if actor == "" {
		return out
	}
	class := effectActorClass(log, actor)
	observe := func(code string, effect uint32) {
		if code == "" || effect == 0 {
			return
		}
		label := names[effectNameKey{"*", effect}]
		if label == "" {
			label = names[effectNameKey{class, effect}]
		}
		if label == "" {
			return
		}
		if old := out[code]; old != "" && old != label {
			conflicts[code] = true
		}
		out[code] = label
	}
	for _, u := range log.SkillUses {
		if u.ActorID == actor && u.HasTail {
			observe(u.Code, u.EffectCandidate)
		}
	}
	for _, h := range log.Events {
		if h.PlayerID == actor && h.Link != nil && h.Link.Paired && len(h.Link.Names) == 1 {
			observe(h.Link.Names[0], h.Link.Effect)
		}
	}
	for code := range conflicts {
		delete(out, code)
	}
	return out
}

func skillRowDisplayNameWithAliases(r SkillDamageRow, aliases map[string]string) string {
	identifierNamesOnce.Do(loadIdentifierNames)
	if strings.HasPrefix(r.Key, "skill:") || strings.HasPrefix(r.Key, "group:") {
		if label := identifierRowName(r, identifierNames); label != "" {
			return label
		}
		codes := r.names
		if len(codes) == 0 && strings.HasPrefix(r.Key, "skill:") {
			codes = []string{strings.TrimPrefix(r.Key, "skill:")}
		}
		label := ""
		for _, code := range codes {
			n := aliases[code]
			if n == "" || (label != "" && label != n) {
				return skillRowDisplayName(r)
			}
			label = n
		}
		if label != "" {
			return label
		}
	}
	return skillRowDisplayName(r)
}

func identifierRowName(r SkillDamageRow, names map[effectNameKey]string) string {
	label := ""
	if len(r.events) == 0 {
		return ""
	}
	for _, h := range r.events {
		if h.Link == nil || !h.Link.Paired || h.Link.Effect == 0 {
			return ""
		}
		n := names[effectNameKey{"*", h.Link.Effect}]
		if n == "" || (label != "" && label != n) {
			return ""
		}
		label = n
	}
	return label
}
