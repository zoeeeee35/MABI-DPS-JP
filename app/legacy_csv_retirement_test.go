package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLegacyCSVsAreIgnoredAtStartup(t *testing.T) {
	if mode := os.Getenv("MABI_TEST_RETIRED_CSV"); mode != "" {
		if got := japaneseSkillName("IceMage_FreezingField"); got != "フリージングフィールド" {
			t.Fatal("legacy skill CSV changed display", got)
		}
		loadEffectNames()
		want := ""
		if mode == "2" {
			want = "ID辞書の表示名"
		}
		if effectNames[effectNameKey{"*", 0x12345678}] != want {
			t.Fatal("legacy effect CSV loaded")
		}
		return
	}
	dir := t.TempDir()
	for name, data := range map[string]string{
		"スキル翻訳.csv":  "内部名,日本語訳\nIceMage_FreezingField,DO NOT LOAD\n",
		"効果ID翻訳.csv": "職業,効果ID,日本語訳\n*,0x12345678,DO NOT LOAD\n",
	} {
		if e := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); e != nil {
			t.Fatal(e)
		}
	}
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(exe)
	if e != nil {
		t.Fatal(e)
	}
	copy := filepath.Join(dir, "retired-csv-test.exe")
	if e = os.WriteFile(copy, b, 0700); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(copy, "-test.run=^TestLegacyCSVsAreIgnoredAtStartup$")
	cmd.Env = append(os.Environ(), "MABI_TEST_RETIRED_CSV=1")
	if output, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("%v: %s", e, output)
	}
	if e = os.WriteFile(filepath.Join(dir, identifierTranslationCSVName), []byte("職業,効果ID,日本語訳\n*,0x12345678,ID辞書の表示名\n"), 0600); e != nil {
		t.Fatal(e)
	}
	cmd = exec.Command(copy, "-test.run=^TestLegacyCSVsAreIgnoredAtStartup$")
	cmd.Env = append(os.Environ(), "MABI_TEST_RETIRED_CSV=2")
	if output, e := cmd.CombinedOutput(); e != nil {
		t.Fatalf("new ID dictionary no longer loads: %v: %s", e, output)
	}
}
