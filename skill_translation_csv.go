package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const skillTranslationCSVName = "スキル翻訳.csv"
const maxSkillTranslationCSV = 4 << 20

type SkillTranslationCSVStatus struct {
	SHA256     string `json:"sha256,omitempty"`
	Present    bool   `json:"present"`
	Loaded     int    `json:"loaded"`
	Unverified int    `json:"unverified"`
	Error      string `json:"error,omitempty"`
}

var skillTranslationCSVStatus SkillTranslationCSVStatus

func parseSkillTranslationCSV(data []byte) (map[string]string, int, error) {
	if len(data) > maxSkillTranslationCSV {
		return nil, 0, fmt.Errorf("4MiB以内のCSVを指定してください")
	}
	if !utf8.Valid(data) {
		return nil, 0, fmt.Errorf("文字コードがUTF-8ではありません。CSV UTF-8で保存してください")
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	reader := csv.NewReader(bytes.NewReader(data))
	reader.TrimLeadingSpace = true
	header, e := reader.Read()
	if e != nil {
		return nil, 0, fmt.Errorf("見出しを読み込めません")
	}
	codeCol, labelCol, stateCol := -1, -1, -1
	for i, h := range header {
		switch strings.TrimSpace(h) {
		case "内部名（変更しない）", "内部名", "InternalName":
			if codeCol >= 0 {
				return nil, 0, fmt.Errorf("内部名の列が重複しています")
			}
			codeCol = i
		case "日本語訳（記入欄）", "日本語訳", "JapaneseName":
			if labelCol >= 0 {
				return nil, 0, fmt.Errorf("日本語訳の列が重複しています")
			}
			labelCol = i
		case "確認状態":
			stateCol = i
		}
	}
	if codeCol < 0 || labelCol < 0 {
		return nil, 0, fmt.Errorf("内部名と日本語訳の列が必要です")
	}
	aliases := map[string]string{}
	pending := map[string]bool{}
	for line := 2; ; line++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("%d行付近の列数・引用符が不正です", line)
		}
		if line > 10001 {
			return nil, 0, fmt.Errorf("翻訳は10,000行以内にしてください")
		}
		code, label := strings.TrimSpace(row[codeCol]), strings.TrimSpace(row[labelCol])
		if code == "" && label == "" {
			continue
		}
		if code == "" || len(code) > 512 {
			return nil, 0, fmt.Errorf("%d行目の内部名が空欄か長すぎます", line)
		}
		for _, ch := range code {
			if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '_') {
				return nil, 0, fmt.Errorf("%d行目の内部名に使用できない文字があります", line)
			}
		}
		if len(label) > 512 || strings.ContainsAny(label, "\r\n\x00") {
			return nil, 0, fmt.Errorf("%d行目の訳が長すぎるか改行を含みます", line)
		}
		if old, exists := aliases[code]; exists && old != label {
			return nil, 0, fmt.Errorf("%d行目で同じ内部名に異なる訳があります", line)
		}
		aliases[code] = label
		if label != "" && stateCol >= 0 && strings.TrimSpace(row[stateCol]) != "確認済み" {
			pending[code] = true
		}
	}
	return aliases, len(pending), nil
}

func readSkillTranslationCSV(dir string) (map[string]string, SkillTranslationCSVStatus) {
	f, e := os.Open(filepath.Join(dir, skillTranslationCSVName))
	if os.IsNotExist(e) {
		return nil, SkillTranslationCSVStatus{}
	}
	status := SkillTranslationCSVStatus{Present: true}
	if e != nil {
		status.Error = "CSVを開けません"
		return nil, status
	}
	defer f.Close()
	data, e := io.ReadAll(io.LimitReader(f, maxSkillTranslationCSV+1))
	if e != nil {
		status.Error = "CSVを読み取れません"
		return nil, status
	}
	status.SHA256 = dataSHA256(data)
	aliases, pending, e := parseSkillTranslationCSV(data)
	if e != nil {
		status.Error = e.Error()
		return nil, status
	}
	for _, label := range aliases {
		if label != "" {
			status.Loaded++
		}
	}
	status.Unverified = pending
	return aliases, status
}
func skillTranslationStatusText() string {
	skillNamesOnce.Do(loadSkillDisplayNames)
	s := skillTranslationCSVStatus
	if s.Error != "" {
		return skillTranslationCSVName + "：読込失敗（従来辞書を使用） / " + s.Error
	}
	if !s.Present {
		return skillTranslationCSVName + "：未配置（従来辞書を使用）"
	}
	if s.Unverified > 0 {
		return fmt.Sprintf("%s：%d件読込 / 対応未確認の表示名 %d件", skillTranslationCSVName, s.Loaded, s.Unverified)
	}
	return fmt.Sprintf("%s：%d件読込", skillTranslationCSVName, s.Loaded)
}
