package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestSkillCatalogCSVOnlyImportsConfirmedTranslations(t *testing.T) {
	rows := []SkillCatalogEntry{{Internal: "IceMage_Test", Class: "氷結術師", Current: "既存", Received: 3, Files: 1}, {Internal: "FireMage_Test", Received: 2, Files: 1}}
	var b bytes.Buffer
	if e := writeSkillCatalog(&b, rows); e != nil {
		t.Fatal(e)
	}
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(b.String(), "\ufeff")))
	data, e := r.ReadAll()
	if e != nil {
		t.Fatal(e)
	}
	data[1][3] = "確認した和名"
	data[1][4] = "確認済み"
	data[2][3] = "未検証の訳"
	b.Reset()
	w := csv.NewWriter(&b)
	w.WriteAll(data)
	out, n, e := importSkillCatalog(&b, []byte(`{"note":"preserve","known_aliases":{"Other":"既存訳"}}`))
	if e != nil {
		t.Fatal(e)
	}
	var doc struct {
		Note    string
		Aliases map[string]string `json:"known_aliases"`
	}
	json.Unmarshal(out, &doc)
	if n != 1 || doc.Note != "preserve" || doc.Aliases["IceMage_Test"] != "確認した和名" || doc.Aliases["FireMage_Test"] != "" || doc.Aliases["Other"] != "既存訳" {
		t.Fatal(string(out))
	}
}
func TestSkillCatalogRejectsConflictingTranslations(t *testing.T) {
	var b bytes.Buffer
	writeSkillCatalog(&b, []SkillCatalogEntry{{Internal: "A"}, {Internal: "A"}})
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(b.String(), "\ufeff")))
	rows, _ := r.ReadAll()
	for i := 1; i < 3; i++ {
		rows[i][4] = "確認済み"
		rows[i][3] = string(rune('a' + i))
	}
	b.Reset()
	w := csv.NewWriter(&b)
	w.WriteAll(rows)
	if _, _, e := importSkillCatalog(&b, []byte(`{"known_aliases":{}}`)); e == nil {
		t.Fatal("conflict accepted")
	}
}
func TestLocalExportSkillCatalog(t *testing.T) {
	root := os.Getenv("MABI_SKILL_CATALOG_ROOT")
	out := os.Getenv("MABI_SKILL_CATALOG_OUT")
	if root == "" || out == "" {
		t.Skip("local captures")
	}
	if e := exportSkillCatalogFiles(root, out); e != nil {
		t.Fatal(e)
	}
}
