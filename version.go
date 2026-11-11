package main

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"strings"
)

//go:embed VERSION
var versionFile string

//go:embed TRANSLATIONS_VERSION
var translationRevisionFile string

// Set by build-native.ps1. Development builds remain distinguishable.
var buildUTC = "development"
var translationSeedSHA256 string

func appVersion() string          { return strings.TrimSpace(versionFile) }
func appWindowTitle() string      { return "MABI DPS JP v" + appVersion() }
func translationRevision() string { return strings.TrimSpace(translationRevisionFile) }
func translationEdited() bool {
	s := skillTranslationCSVStatus
	return s.Present && s.SHA256 != "" && translationSeedSHA256 != "" && s.SHA256 != translationSeedSHA256
}
func versionInformation() map[string]any {
	skillNamesOnce.Do(loadSkillDisplayNames)
	return map[string]any{"application": "MABI DPS JP", "version": appVersion(), "buildUTC": buildUTC,
		"translationRevision": translationRevision(), "translationEdited": translationEdited(),
		"translationCSV": skillTranslationCSVStatus}
}
func dataSHA256(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }
