package main

import (
	"os"
	"strings"
	"testing"
)

func TestLocalArchiveIdentifierDisplayNames(t *testing.T) {
	filename, dict := os.Getenv("MABI_NAME_ARCHIVE"), os.Getenv("MABI_NAME_DICTIONARY")
	if filename == "" || dict == "" {
		t.Skip("local archive/dictionary not specified")
	}
	b, e := readBoundedFile(dict, 4<<20)
	if e != nil {
		t.Fatal(e)
	}
	names, e := parseEffectNames(b)
	if e != nil {
		t.Fatal(e)
	}
	saved, e := loadArchive(filename)
	if e != nil {
		t.Fatal(e)
	}
	log := saved.Archive.Request.Log
	if log.Party == nil {
		t.Fatal("archive has no self selection")
	}
	actor := ""
	for id, p := range log.Party.Entities {
		if p.Role == RoleSelf {
			actor = id
		}
	}
	if actor == "" {
		t.Fatal("self not specified in archive")
	}
	identifierNamesOnce.Do(loadIdentifierNames)
	old := identifierNames
	identifierNames = names
	defer func() { identifierNames = old }()
	aliases := identifierSkillAliases(log, actor, names)
	rows := classifyUnknownEffectRows(linkedSkillRows(&saved.Battle, log, actor), log, names)
	var before, after int64
	englishBefore, englishAfter := 0, 0
	for i := range rows {
		before += rows[i].Damage
		if strings.Contains(rows[i].Name, "_") {
			englishBefore++
		}
		rows[i].Name = skillRowDisplayNameWithAliases(rows[i], aliases)
		after += rows[i].Damage
		if strings.Contains(rows[i].Name, "_") {
			englishAfter++
		}
	}
	if before != after || englishAfter > englishBefore {
		t.Fatal("damage or display regression")
	}
	t.Logf("rows=%d beforeDamage=%d afterDamage=%d internalNamesBefore=%d internalNamesAfter=%d", len(rows), before, after, englishBefore, englishAfter)
	for _, code := range []string{"IceMage_IceBarrier", "IceMage_SplitSlash", "MeleeDefaultAttack_1", "MeleeDefaultAttack_2", "MeleeDefaultAttack_3"} {
		if name := aliases[code]; name != "" {
			t.Logf("%s => %s", code, name)
		}
	}
}
