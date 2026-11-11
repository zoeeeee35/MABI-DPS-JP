package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTranslationCSVAcceptsCatalogAndLoadsPendingLabels(t *testing.T) {
	data := []byte("\ufeff内部名（変更しない）,職業候補,現在の辞書訳,日本語訳（記入欄）,確認状態,受信通知数,送信通知数,出現ファイル数,備考\r\nIceMage_Test,氷結術師,旧名,新しい表示名,要確認,2,0,1,出典付き\r\nEmpty,不明,既存名,,未確認,0,0,0,\r\n")
	aliases, pending, e := parseSkillTranslationCSV(data)
	if e != nil || aliases["IceMage_Test"] != "新しい表示名" || len(aliases) != 2 || pending != 1 {
		t.Fatal(aliases, pending, e)
	}
}
func TestTranslationCSVRejectsMalformedInputAtomically(t *testing.T) {
	for _, data := range [][]byte{[]byte("内部名,日本語訳\nGood,正しい\nBad,不正,余分\n"), []byte("内部名,日本語訳\nSame,一\nSame,二\n"), []byte("内部名,日本語訳\nBad-ID,訳\n"), []byte("内部名,日本語訳\nA,\"改行\nあり\"\n"), {0x82, 0xa0}} {
		aliases, _, e := parseSkillTranslationCSV(data)
		if e == nil || aliases != nil {
			t.Fatal("invalid file partly applied", aliases, e)
		}
	}
}
func TestTranslationCSVSimpleSchemaAndDirectoryIndependentReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, skillTranslationCSVName)
	a, s := readSkillTranslationCSV(dir)
	if a != nil || s.Present || s.Error != "" {
		t.Fatal(a, s)
	}
	for _, name := range []string{"最初の表示名", "編集後の表示名"} {
		if e := os.WriteFile(path, []byte("内部名,日本語訳\nHighMage_Fireball,"+name+"\n"), 0600); e != nil {
			t.Fatal(e)
		}
		a, s = readSkillTranslationCSV(dir)
		if a["HighMage_Fireball"] != name || s.Loaded != 1 || s.Error != "" {
			t.Fatal(a, s)
		}
	}
}
func TestTranslationCSVFileSizeAndIdenticalDuplicate(t *testing.T) {
	if _, _, e := parseSkillTranslationCSV([]byte(strings.Repeat("x", maxSkillTranslationCSV+1))); e == nil {
		t.Fatal("size not checked")
	}
	a, n, e := parseSkillTranslationCSV([]byte("内部名,日本語訳\nA,名前\nA,名前\n"))
	if e != nil || len(a) != 1 || n != 0 {
		t.Fatal(a, n, e)
	}
}
