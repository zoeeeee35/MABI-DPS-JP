package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const updateRepository = "zoeeeee35/MABI-DPS-JP"
const updateAPI = "https://api.github.com/repos/" + updateRepository + "/releases/latest"
const maxUpdateZIP = 64 << 20

type updateAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}
type updateRelease struct {
	Tag        string        `json:"tag_name"`
	Body       string        `json:"body"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []updateAsset `json:"assets"`
}
type updateOffer struct {
	Version, Notes string
	Asset          updateAsset
}
type updatePlan struct {
	Parent                   uint32
	Version, Digest, DataDir string
}

func versionNumbers(s string) ([3]uint64, error) {
	var out [3]uint64
	if !regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`).MatchString(s) {
		return out, fmt.Errorf("バージョン形式が不正です")
	}
	for i, p := range strings.Split(s, ".") {
		n, e := strconv.ParseUint(p, 10, 32)
		if e != nil {
			return out, e
		}
		out[i] = n
	}
	return out, nil
}
func newerVersion(a, b string) bool {
	x, e := versionNumbers(a)
	if e != nil {
		return false
	}
	y, e := versionNumbers(b)
	if e != nil {
		return false
	}
	for i := range x {
		if x[i] != y[i] {
			return x[i] > y[i]
		}
	}
	return false
}
func validUpdateURL(raw string, download bool) bool {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" {
		return false
	}
	if !download {
		return u.Host == "api.github.com" && u.Path == "/repos/"+updateRepository+"/releases/latest" && u.RawQuery == ""
	}
	return (u.Host == "github.com" && strings.HasPrefix(u.Path, "/"+updateRepository+"/releases/download/")) || u.Host == "release-assets.githubusercontent.com" || u.Host == "objects.githubusercontent.com"
}
func updateHTTPClient() *http.Client {
	return &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 || !validUpdateURL(req.URL.String(), true) {
			return fmt.Errorf("許可されていない更新先への転送です")
		}
		return nil
	}}
}
func fetchUpdate(ctx context.Context, client *http.Client, current string) (*updateOffer, error) {
	req, e := http.NewRequestWithContext(ctx, "GET", updateAPI, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "MABI-DPS-JP-update-check")
	res, e := client.Do(req)
	if e != nil {
		return nil, fmt.Errorf("GitHubに接続できません")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub更新確認 HTTP %d（公開Releaseを確認してください）", res.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
	if e != nil || len(b) > 2<<20 {
		return nil, fmt.Errorf("更新情報が大きすぎるか読み取れません")
	}
	var rel updateRelease
	if json.Unmarshal(b, &rel) != nil {
		return nil, fmt.Errorf("更新情報が不正です")
	}
	v := strings.TrimPrefix(rel.Tag, "v")
	if _, e = versionNumbers(v); e != nil {
		return nil, e
	}
	if rel.Draft || rel.Prerelease || !newerVersion(v, current) {
		return nil, nil
	}
	wanted := "MABI-DPS-JP-v" + v + "-Windows.zip"
	var offer *updateOffer
	// Prefer the versioned distribution over a downloaded Actions wrapper ZIP.
	sort.SliceStable(rel.Assets, func(i, j int) bool { return rel.Assets[i].Name == wanted && rel.Assets[j].Name != wanted })
	for _, a := range rel.Assets {
		if a.Name != wanted && a.Name != "MABI-DPS-JP-Windows.zip" {
			continue
		}
		if offer != nil {
			if offer.Asset.Name == a.Name {
				return nil, fmt.Errorf("更新ファイル名が重複しています")
			}
			continue
		}
		if a.Size <= 0 || a.Size > maxUpdateZIP || !validUpdateURL(a.URL, true) || !strings.HasPrefix(a.URL, "https://github.com/"+updateRepository+"/releases/download/"+rel.Tag+"/") {
			return nil, fmt.Errorf("更新ファイルの取得先・サイズが不正です")
		}
		digest := strings.TrimPrefix(a.Digest, "sha256:")
		decoded, err := hex.DecodeString(digest)
		if err != nil || len(decoded) != 32 || !strings.HasPrefix(a.Digest, "sha256:") {
			return nil, fmt.Errorf("GitHubのSHA-256がないため自動更新できません")
		}
		offer = &updateOffer{Version: v, Notes: rel.Body, Asset: a}
	}
	if offer == nil {
		return nil, fmt.Errorf("最新版に対応するWindows配布ZIPがありません")
	}
	return offer, nil
}

// Only application-owned distribution files may be installed. User data and
// downloaded programs/scripts other than the single application are excluded.
func updateFileAllowed(name string) bool {
	switch name {
	case "MabiDPS-JP.exe", "識別子翻訳.csv", "VERSION", "TRANSLATIONS_VERSION", "release-manifest.json", "スキル翻訳.csv", "効果ID翻訳.csv", "使い方.txt", "LICENSE", "THIRD-PARTY.md", "sample-combat.json", "効果音クレジット.txt", "Paperlogy-OFL.txt", "RoundedMplus-OFL.txt":
		return true
	}
	ext := strings.ToLower(path.Ext(name))
	for _, prefix := range []string{"config/", "licenses/", "sounds/"} {
		if strings.HasPrefix(name, prefix) {
			if prefix == "sounds/" {
				return ext == ".mp3" || ext == ".wav" || ext == ".txt"
			}
			return ext == ".json" || ext == ".txt" || ext == ".md"
		}
	}
	return false
}
func safeUpdateName(name string) bool {
	if name == "" || len(name) > 240 || strings.ContainsAny(name, "\\:\x00\r\n<>|?*\t") || strings.HasPrefix(name, "/") || path.Clean(name) != name {
		return false
	}
	for _, p := range strings.Split(name, "/") {
		if p == "" || p == "." || p == ".." || strings.TrimRight(p, " .") != p {
			return false
		}
		base := strings.ToUpper(strings.SplitN(p, ".", 2)[0])
		if regexp.MustCompile(`^(CON|PRN|AUX|NUL|COM[0-9]|LPT[0-9])$`).MatchString(base) {
			return false
		}
	}
	return updateFileAllowed(name)
}
func inspectUpdateZIP(filename, version, digest string) (map[string][]byte, error) {
	b, e := readBoundedFile(filename, maxUpdateZIP)
	if e != nil {
		return nil, e
	}
	if !strings.EqualFold(dataSHA256(b), strings.TrimPrefix(digest, "sha256:")) {
		return nil, fmt.Errorf("更新ZIPのSHA-256が一致しません")
	}
	return unpackUpdateZIP(b, version, true)
}
func unpackUpdateZIP(b []byte, version string, allowWrapper bool) (map[string][]byte, error) {
	r, e := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if e != nil {
		return nil, e
	}
	if len(r.File) > 200 {
		return nil, fmt.Errorf("ZIP内のファイル数が上限を超えています")
	}
	files := map[string][]byte{}
	seen := map[string]bool{}
	var total uint64
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name := f.Name
		// Accept either a flat package or one release-named wrapper directory.
		name = strings.TrimPrefix(name, "MABI-DPS-JP-v"+version+"-Windows/")
		wrapperFile := allowWrapper && (name == "MABI-DPS-JP-v"+version+"-Windows.zip" || name == "SHA256SUMS.txt")
		if (!safeUpdateName(name) && !wrapperFile) || f.Mode()&os.ModeType != 0 {
			return nil, fmt.Errorf("ZIP内に許可されていないファイルがあります")
		}
		if seen[strings.ToLower(name)] {
			return nil, fmt.Errorf("ZIP内のパスが重複しています")
		}
		seen[strings.ToLower(name)] = true
		total += f.UncompressedSize64
		if f.UncompressedSize64 > 96<<20 || total > 192<<20 {
			return nil, fmt.Errorf("ZIP展開サイズが上限を超えています")
		}
		in, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(io.LimitReader(in, (96<<20)+1))
		in.Close()
		if err != nil || len(content) > 96<<20 {
			return nil, fmt.Errorf("ZIP展開に失敗しました")
		}
		files[name] = content
	}
	if inner, ok := files["MABI-DPS-JP-v"+version+"-Windows.zip"]; ok && allowWrapper {
		if len(files) > 2 || (len(files) == 2 && files["SHA256SUMS.txt"] == nil) || len(inner) > maxUpdateZIP {
			return nil, fmt.Errorf("Actions ZIPの構成が不正です")
		}
		return unpackUpdateZIP(inner, version, false)
	}
	if stringVersion := strings.TrimSpace(string(files["VERSION"])); stringVersion != version {
		return nil, fmt.Errorf("ZIP内のバージョンが一致しません")
	}
	exe := files["MabiDPS-JP.exe"]
	if len(exe) < 2 || string(exe[:2]) != "MZ" {
		return nil, fmt.Errorf("更新EXEがありません")
	}
	var manifest struct {
		Version string `json:"version"`
		Hash    string `json:"executableSHA256"`
	}
	if json.Unmarshal(files["release-manifest.json"], &manifest) != nil || manifest.Version != version || !strings.EqualFold(manifest.Hash, dataSHA256(exe)) {
		return nil, fmt.Errorf("更新EXEの検証に失敗しました")
	}
	return files, nil
}
func stageUpdate(ctx context.Context, client *http.Client, root string, offer updateOffer, parent uint32) (string, error) {
	if !validUpdateURL(offer.Asset.URL, true) || offer.Asset.Size <= 0 || offer.Asset.Size > maxUpdateZIP {
		return "", fmt.Errorf("取得条件が不正です")
	}
	updates := filepath.Join(root, ".updates")
	if e := os.MkdirAll(updates, 0700); e != nil {
		return "", e
	}
	if e := checkUpdatePath(root, updates); e != nil {
		return "", e
	}
	stage, e := os.MkdirTemp(updates, "pending-")
	if e != nil {
		return "", e
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(stage)
		}
	}()
	req, e := http.NewRequestWithContext(ctx, "GET", offer.Asset.URL, nil)
	if e != nil {
		return "", e
	}
	req.Header.Set("User-Agent", "MABI-DPS-JP-update-download")
	res, e := client.Do(req)
	if e != nil {
		return "", fmt.Errorf("更新ZIPを取得できません")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fmt.Errorf("ZIP取得 HTTP %d", res.StatusCode)
	}
	f, e := os.OpenFile(filepath.Join(stage, "package.zip"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return "", e
	}
	n, err := io.Copy(f, io.LimitReader(res.Body, maxUpdateZIP+1))
	closeErr := f.Close()
	if err != nil || closeErr != nil || n != offer.Asset.Size {
		return "", fmt.Errorf("更新ZIPのサイズが一致しません")
	}
	if _, e = inspectUpdateZIP(filepath.Join(stage, "package.zip"), offer.Version, offer.Asset.Digest); e != nil {
		return "", e
	}
	plan, _ := json.Marshal(updatePlan{Parent: parent, Version: offer.Version, Digest: offer.Asset.Digest})
	if e = os.WriteFile(filepath.Join(stage, "plan.json"), plan, 0600); e != nil {
		return "", e
	}
	helper, e := readBoundedFile(filepath.Join(root, "MabiDPS-JP.exe"), 96<<20)
	if e != nil {
		return "", e
	}
	if e = os.WriteFile(filepath.Join(stage, "MabiDPS-JP-update.exe"), helper, 0700); e != nil {
		return "", e
	}
	ok = true
	return stage, nil
}
func preserveUpdateFile(name string) bool {
	return strings.HasSuffix(name, ".csv") || strings.HasPrefix(name, "config/") || strings.HasPrefix(name, "sounds/")
}

// Reject links and paths outside the installation before opening any target.
func checkUpdatePath(root, dest string) error {
	root, e := filepath.Abs(root)
	if e != nil {
		return e
	}
	dest, e = filepath.Abs(dest)
	if e != nil {
		return e
	}
	rel, e := filepath.Rel(root, dest)
	if e != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("更新先がアプリ外です")
	}
	current := root
	parts := append([]string{""}, strings.Split(rel, string(os.PathSeparator))...)
	for _, p := range parts {
		if p != "" {
			current = filepath.Join(current, p)
		}
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("リンク先には更新できません")
		}
	}
	return nil
}

// Roll back replaced files on a write/rename error. Backups also remain in the
// transaction directory for recovery after unexpected interruption/power loss.
func installUpdateFiles(root, stage string, files map[string][]byte) error {
	keys := make([]string, 0, len(files))
	for k := range files {
		if !safeUpdateName(k) {
			return fmt.Errorf("更新パスが不正です")
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, name := range keys {
		if e := checkUpdatePath(root, filepath.Join(root, filepath.FromSlash(name))); e != nil {
			return e
		}
	}
	type changedFile struct {
		name   string
		hadOld bool
	}
	changed := []changedFile{}
	rollback := func() error {
		var errs []string
		for i := len(changed) - 1; i >= 0; i-- {
			c := changed[i]
			dst := filepath.Join(root, filepath.FromSlash(c.name))
			if e := os.Remove(dst); e != nil && !os.IsNotExist(e) {
				errs = append(errs, c.name)
				continue
			}
			if c.hadOld {
				if e := os.Rename(filepath.Join(stage, "backup", filepath.FromSlash(c.name)), dst); e != nil {
					errs = append(errs, c.name)
				}
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("復元できないファイルがあります。更新フォルダーのbackupを保持してください")
		}
		return nil
	}
	fail := func(e error) error {
		if r := rollback(); r != nil {
			return fmt.Errorf("%v / %v", e, r)
		}
		return e
	}
	for _, name := range keys {
		dst := filepath.Join(root, filepath.FromSlash(name))
		info, e := os.Lstat(dst)
		exists := e == nil
		if e != nil && !os.IsNotExist(e) {
			return fail(e)
		}
		if exists && !info.Mode().IsRegular() {
			return fail(fmt.Errorf("更新先が通常ファイルではありません"))
		}
		if exists && preserveUpdateFile(name) {
			continue
		}
		if e = os.MkdirAll(filepath.Dir(dst), 0700); e != nil {
			return fail(e)
		}
		backup := filepath.Join(stage, "backup", filepath.FromSlash(name))
		if exists {
			if e = os.MkdirAll(filepath.Dir(backup), 0700); e != nil {
				return fail(e)
			}
			if e = os.Rename(dst, backup); e != nil {
				return fail(e)
			}
		}
		changed = append(changed, changedFile{name, exists})
		if e = os.WriteFile(dst, files[name], 0600); e != nil {
			return fail(e)
		}
	}
	return nil
}
