package main

import (
	"os"
	"path/filepath"
	"sync"
)

// Optional local display dictionary. It never changes event or skill identity.
const identifierTranslationCSVName = "識別子翻訳.csv"

var identifierNamesOnce sync.Once
var identifierNames = map[effectNameKey]string{}

func loadIdentifierNames() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	data, err := readBoundedFile(filepath.Join(filepath.Dir(exe), identifierTranslationCSVName), 4<<20)
	if err != nil {
		return
	}
	names, err := parseEffectNames(data)
	if err == nil {
		identifierNames = names
	}
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
