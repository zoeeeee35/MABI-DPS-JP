package main

import (
	"regexp"
	"testing"
)

func TestReleaseVersionFormat(t *testing.T) {
	if !regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`).MatchString(appVersion()) {
		t.Fatal("invalid semantic version", appVersion())
	}
	if !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(translationRevision()) {
		t.Fatal("invalid data revision")
	}
	if appWindowTitle() != "MABI DPS JP v"+appVersion() {
		t.Fatal("title mismatch")
	}
}
func TestTranslationRevisionDetectsUserEdits(t *testing.T) {
	old, seed := skillTranslationCSVStatus, translationSeedSHA256
	defer func() { skillTranslationCSVStatus = old; translationSeedSHA256 = seed }()
	translationSeedSHA256 = dataSHA256([]byte("seed"))
	skillTranslationCSVStatus = SkillTranslationCSVStatus{Present: true, SHA256: translationSeedSHA256}
	if translationEdited() {
		t.Fatal("unchanged seed marked edited")
	}
	skillTranslationCSVStatus.SHA256 = dataSHA256([]byte("user edit"))
	if !translationEdited() {
		t.Fatal("edit not detected")
	}
	skillTranslationCSVStatus = SkillTranslationCSVStatus{}
	if translationEdited() {
		t.Fatal("missing CSV marked edited")
	}
}
