package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type skillNameDictionary struct {
	KnownAliases map[string]string `json:"known_aliases"`
}

var skillNamesOnce sync.Once
var skillNamesJA skillNameDictionary

func loadSkillDisplayNames() {
	data, _ := assets.ReadFile("config/skill-name-ja.json")
	_ = json.Unmarshal(data, &skillNamesJA)
	if skillNamesJA.KnownAliases == nil {
		skillNamesJA.KnownAliases = map[string]string{}
	}
	if exe, e := os.Executable(); e == nil {
		if local, e := readBoundedFile(filepath.Join(filepath.Dir(exe), "config", "skill-name-ja.json"), 1<<20); e == nil {
			var custom skillNameDictionary
			if json.Unmarshal(local, &custom) == nil && custom.KnownAliases != nil {
				for k, v := range custom.KnownAliases {
					if strings.TrimSpace(v) != "" {
						skillNamesJA.KnownAliases[k] = v
					}
				}
			}
		}
	}
	if exe, e := os.Executable(); e == nil {
		aliases, status := readSkillTranslationCSV(filepath.Dir(exe))
		skillTranslationCSVStatus = status
		if status.Error == "" {
			for code, label := range aliases {
				skillNamesJA.KnownAliases[code] = label
			}
		}
	}
}
func japaneseSkillName(internal string) string {
	skillNamesOnce.Do(loadSkillDisplayNames)
	if alias := skillNamesJA.KnownAliases[internal]; strings.TrimSpace(alias) != "" {
		return alias
	}
	return internal
}
func skillRowDisplayName(r SkillDamageRow) string {
	if strings.HasPrefix(r.Key, "skill:") || strings.HasPrefix(r.Key, "group:") {
		identifierNamesOnce.Do(loadIdentifierNames)
		if label := identifierRowName(r, identifierNames); label != "" {
			return label
		}
	}
	if !strings.HasPrefix(r.Key, "group:") {
		return japaneseSkillName(r.Name)
	}
	allMelee := len(r.names) > 1
	names := []string{}
	seen := map[string]bool{}
	for _, n := range r.names {
		if n != "MeleeDefaultAttack_1" && n != "MeleeDefaultAttack_2" && n != "MeleeDefaultAttack_3" {
			allMelee = false
		}
		label := japaneseSkillName(n)
		if label == n {
			allMelee = false
		}
		if !seen[label] {
			seen[label] = true
			names = append(names, label)
		}
	}
	if allMelee {
		return "通常攻撃（近接）"
	}
	if len(names) > 0 {
		return strings.Join(names, " / ")
	}
	return r.Name
}
