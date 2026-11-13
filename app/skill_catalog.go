package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type SkillCatalogEntry struct {
	Internal, Class, Current string
	Received, Sent, Files    int
}
type SkillCatalogReport struct {
	Files, Duplicates int
	Failed            []string
	Entries           int
}

var skillCatalogHeader = []string{"内部名（変更しない）", "職業候補", "現在の辞書訳", "日本語訳（記入欄）", "確認状態", "受信通知数", "送信通知数", "出現ファイル数", "備考"}

func collectSkillCatalog(root string) ([]SkillCatalogEntry, SkillCatalogReport, error) {
	report := SkillCatalogReport{}
	entries := map[string]*SkillCatalogEntry{}
	hashes := map[[32]byte]bool{}
	paths := []string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !d.IsDir() && (ext == ".pcapng" || ext == ".pcap") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, report, err
	}
	sort.Strings(paths)
	for _, path := range paths {
		f, e := os.Open(path)
		if e != nil {
			report.Failed = append(report.Failed, filepath.Base(path))
			continue
		}
		stat, e := f.Stat()
		if e != nil || stat.Size() > 64<<20 {
			f.Close()
			report.Failed = append(report.Failed, filepath.Base(path))
			continue
		}
		hash := sha256.New()
		_, e = io.Copy(hash, f)
		if e != nil {
			f.Close()
			report.Failed = append(report.Failed, filepath.Base(path))
			continue
		}
		var sum [32]byte
		copy(sum[:], hash.Sum(nil))
		if hashes[sum] {
			f.Close()
			report.Duplicates++
			continue
		}
		hashes[sum] = true
		_, _ = f.Seek(0, 0)
		local := map[string]*SkillCatalogEntry{}
		_, e = replayPCAPObserved(f, 16000, func(id uint32, p []byte, out bool) {
			if !out {
				id = jpReceiveMessageKind(id)
			}
			if id != 0x1d4f8 {
				return
			}
			s, ok := decodeJPSkillUse(p)
			if !ok {
				return
			}
			r := local[s.Code]
			if r == nil {
				r = &SkillCatalogEntry{Internal: s.Code, Class: jpClassNames[s.Class]}
				local[s.Code] = r
			}
			if out {
				r.Sent++
			} else {
				r.Received++
			}
		})
		f.Close()
		if e != nil {
			report.Failed = append(report.Failed, filepath.Base(path))
			continue
		}
		report.Files++
		for code, r := range local {
			v := entries[code]
			if v == nil {
				label := japaneseSkillName(code)
				if label == code {
					label = ""
				}
				v = &SkillCatalogEntry{Internal: code, Class: r.Class, Current: label}
				entries[code] = v
			}
			v.Received += r.Received
			v.Sent += r.Sent
			v.Files++
		}
	}
	rows := []SkillCatalogEntry{}
	for _, v := range entries {
		rows = append(rows, *v)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Class != rows[j].Class {
			return rows[i].Class < rows[j].Class
		}
		return rows[i].Internal < rows[j].Internal
	})
	report.Entries = len(rows)
	return rows, report, nil
}
func writeSkillCatalog(w io.Writer, rows []SkillCatalogEntry) error {
	if _, e := io.WriteString(w, "\ufeff"); e != nil {
		return e
	}
	c := csv.NewWriter(w)
	c.UseCRLF = true
	if e := c.Write(skillCatalogHeader); e != nil {
		return e
	}
	for _, r := range rows {
		if e := c.Write([]string{r.Internal, r.Class, r.Current, "", "未確認", strconv.Itoa(r.Received), strconv.Itoa(r.Sent), strconv.Itoa(r.Files), ""}); e != nil {
			return e
		}
	}
	c.Flush()
	return c.Error()
}

// Only explicitly confirmed rows become display aliases; event classification never changes.
func importSkillCatalog(r io.Reader, base []byte) ([]byte, int, error) {
	c := csv.NewReader(io.LimitReader(r, 4<<20))
	header, e := c.Read()
	if e != nil {
		return nil, 0, e
	}
	header[0] = strings.TrimPrefix(header[0], "\ufeff")
	if len(header) != len(skillCatalogHeader) {
		return nil, 0, fmt.Errorf("列数が異なります")
	}
	for i := range header {
		if header[i] != skillCatalogHeader[i] {
			return nil, 0, fmt.Errorf("見出しは変更しないでください")
		}
	}
	doc := map[string]json.RawMessage{}
	if e = json.Unmarshal(base, &doc); e != nil {
		return nil, 0, e
	}
	aliases := map[string]string{}
	if b := doc["known_aliases"]; len(b) > 0 {
		if e = json.Unmarshal(b, &aliases); e != nil {
			return nil, 0, e
		}
	}
	changed := map[string]string{}
	for {
		row, e := c.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, 0, e
		}
		if strings.TrimSpace(row[4]) != "確認済み" {
			continue
		}
		code, label := strings.TrimSpace(row[0]), strings.TrimSpace(row[3])
		if code == "" || label == "" {
			return nil, 0, fmt.Errorf("確認済み行の内部名・日本語訳が空欄です")
		}
		for _, ch := range code {
			if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '_') {
				return nil, 0, fmt.Errorf("内部名が不正です")
			}
		}
		if len(label) > 512 || strings.ContainsAny(label, "\r\n") {
			return nil, 0, fmt.Errorf("日本語訳が長すぎるか改行があります")
		}
		if old, ok := changed[code]; ok && old != label {
			return nil, 0, fmt.Errorf("同じ内部名に異なる訳があります")
		}
		changed[code] = label
	}
	for code, label := range changed {
		aliases[code] = label
	}
	doc["known_aliases"], _ = json.Marshal(aliases)
	out, e := json.MarshalIndent(doc, "", "  ")
	return out, len(changed), e
}
func exportSkillCatalogFiles(root, path string) error {
	rows, report, e := collectSkillCatalog(root)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	e = writeSkillCatalog(f, rows)
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	b, _ := json.MarshalIndent(report, "", "  ")
	return os.WriteFile(path+".report.json", b, 0600)
}
func importSkillCatalogFile(input, output string) error {
	base, _ := assets.ReadFile("config/skill-name-ja.json")
	if exe, e := os.Executable(); e == nil {
		if b, e := readBoundedFile(filepath.Join(filepath.Dir(exe), "config", "skill-name-ja.json"), 1<<20); e == nil {
			base = b
		}
	}
	f, e := os.Open(input)
	if e != nil {
		return e
	}
	defer f.Close()
	b, n, e := importSkillCatalog(f, base)
	if e != nil {
		return e
	}
	if n == 0 {
		return fmt.Errorf("日本語訳を記入し、確認状態を「確認済み」にした行がありません")
	}
	if _, e = os.Stat(output); e == nil {
		return fmt.Errorf("出力先が既に存在します。別名を指定してください")
	}
	fout, e := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	_, e = fout.Write(b)
	ce := fout.Close()
	if e != nil {
		return e
	}
	return ce
}
