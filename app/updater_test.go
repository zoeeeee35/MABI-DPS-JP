package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type updateTransport func(*http.Request) (*http.Response, error)

func (f updateTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func updateFixture(t *testing.T, extra map[string][]byte) ([]byte, string) {
	t.Helper()
	exe := []byte("MZ-test-executable")
	manifest, _ := json.Marshal(map[string]string{"version": "1.0.2", "executableSHA256": dataSHA256(exe)})
	files := map[string][]byte{"MabiDPS-JP.exe": exe, "VERSION": []byte("1.0.2"), "release-manifest.json": manifest, "スキル翻訳.csv": []byte("new translations"), "config/ja.json": []byte("new configuration")}
	for k, v := range extra {
		files[k] = v
	}
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for name, content := range files {
		f, e := w.Create(name)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.Write(content); e != nil {
			t.Fatal(e)
		}
	}
	if e := w.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes(), "sha256:" + dataSHA256(b.Bytes())
}
func TestUpdaterVersionsAndURLRestrictions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{{"1.10.0", "1.9.9", true}, {"1.0.1", "1.0.1", false}, {"1.0.0", "1.0.1", false}, {"1.0.2-beta", "1.0.1", false}, {"01.2.3", "1.0.1", false}} {
		if newerVersion(tc.a, tc.b) != tc.want {
			t.Fatal(tc)
		}
	}
	for _, raw := range []string{"http://github.com/zoeeeee35/MABI-DPS-JP/releases/download/v1.0.2/a.zip", "https://github.com.attacker.test/a.zip", "https://attacker@github.com/zoeeeee35/MABI-DPS-JP/releases/download/a", "https://github.com/other/repo/releases/download/a", "file:///C:/a.zip", "https://github.com:8443/zoeeeee35/MABI-DPS-JP/releases/download/a"} {
		if validUpdateURL(raw, true) {
			t.Fatal(raw)
		}
	}
}
func TestUpdaterRejectsPaths(t *testing.T) {
	for _, p := range []string{"../MabiDPS-JP.exe", "/MabiDPS-JP.exe", "config/../MabiDPS-JP.exe", "config\\ja.json", "config/CON.json", "config/a.json:secret", "data/history.json", "Updater.exe", "config/x.ps1", "config/x .json.", "sounds/x.exe"} {
		if safeUpdateName(p) {
			t.Fatal(p)
		}
	}
}
func TestUpdaterReleaseValidationAndNoUpdate(t *testing.T) {
	asset := updateAsset{Name: "MABI-DPS-JP-v1.0.2-Windows.zip", URL: "https://github.com/" + updateRepository + "/releases/download/v1.0.2/MABI-DPS-JP-v1.0.2-Windows.zip", Size: 10, Digest: "sha256:" + strings.Repeat("a", 64)}
	rel := updateRelease{Tag: "v1.0.2", Assets: []updateAsset{asset}}
	client := &http.Client{Transport: updateTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != updateAPI || r.Header.Get("Authorization") != "" {
			t.Fatal("unexpected request")
		}
		b, _ := json.Marshal(rel)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(b))}, nil
	})}
	offer, e := fetchUpdate(context.Background(), client, "1.0.1")
	if e != nil || offer == nil || offer.Version != "1.0.2" {
		t.Fatal(offer, e)
	}
	if offer, e = fetchUpdate(context.Background(), client, "1.0.2"); e != nil || offer != nil {
		t.Fatal(offer, e)
	}
	rel.Assets[0].Digest = ""
	if _, e = fetchUpdate(context.Background(), client, "1.0.1"); e == nil {
		t.Fatal("missing digest accepted")
	}
	rel.Prerelease = true
	if offer, e = fetchUpdate(context.Background(), client, "1.0.1"); e != nil || offer != nil {
		t.Fatal("prerelease accepted")
	}
}
func TestUpdaterZIPIntegrityAndUnexpectedFiles(t *testing.T) {
	for _, tc := range []struct {
		extra   map[string][]byte
		badHash bool
		wantErr bool
	}{
		{nil, false, false}, {nil, true, true}, {map[string][]byte{"../escape.txt": []byte("bad")}, false, true}, {map[string][]byte{"data/history.json": []byte("bad")}, false, true}, {map[string][]byte{"VERSION": []byte("0.0.1")}, false, true}, {map[string][]byte{"MabiDPS-JP.exe": []byte("MZ-tampered")}, false, true},
	} {
		b, digest := updateFixture(t, tc.extra)
		p := filepath.Join(t.TempDir(), "update.zip")
		if e := os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
		if tc.badHash {
			digest = "sha256:" + strings.Repeat("0", 64)
		}
		_, e := inspectUpdateZIP(p, "1.0.2", digest)
		if (e != nil) != tc.wantErr {
			t.Fatal("ZIP validation", tc, e)
		}
	}
}

func TestUpdaterActionsWrapperZIP(t *testing.T) {
	inner, _ := updateFixture(t, nil)
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	f, e := w.Create("MABI-DPS-JP-v1.0.2-Windows.zip")
	if e != nil {
		t.Fatal(e)
	}
	_, _ = f.Write(inner)
	f, e = w.Create("SHA256SUMS.txt")
	if e != nil {
		t.Fatal(e)
	}
	_, _ = f.Write([]byte("checksum"))
	if e = w.Close(); e != nil {
		t.Fatal(e)
	}
	files, e := unpackUpdateZIP(b.Bytes(), "1.0.2", true)
	if e != nil || len(files["MabiDPS-JP.exe"]) == 0 {
		t.Fatal(e)
	}
	if _, e = unpackUpdateZIP(b.Bytes(), "1.0.2", false); e == nil {
		t.Fatal("nested wrapper accepted")
	}
}
func TestUpdaterPreservesUserFilesAndRollback(t *testing.T) {
	root := t.TempDir()
	stage := t.TempDir()
	for name, content := range map[string]string{"MabiDPS-JP.exe": "old exe", "スキル翻訳.csv": "user translation", "config/ja.json": "user config", "data/history/x.json": "private history"} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
			t.Fatal(e)
		}
		if e := os.WriteFile(p, []byte(content), 0600); e != nil {
			t.Fatal(e)
		}
	}
	files := map[string][]byte{"MabiDPS-JP.exe": []byte("new exe"), "スキル翻訳.csv": []byte("new csv"), "config/ja.json": []byte("new config")}
	if e := installUpdateFiles(root, stage, files); e != nil {
		t.Fatal(e)
	}
	for name, want := range map[string]string{"MabiDPS-JP.exe": "new exe", "スキル翻訳.csv": "user translation", "config/ja.json": "user config", "data/history/x.json": "private history"} {
		b, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if e != nil || string(b) != want {
			t.Fatal(name, string(b), e)
		}
	}
	if e := os.Mkdir(filepath.Join(root, "VERSION"), 0700); e != nil {
		t.Fatal(e)
	}
	e := installUpdateFiles(root, t.TempDir(), map[string][]byte{"MabiDPS-JP.exe": []byte("broken update"), "VERSION": []byte("1.0.2")})
	if e == nil {
		t.Fatal("failure not returned")
	}
	b, _ := os.ReadFile(filepath.Join(root, "MabiDPS-JP.exe"))
	if string(b) != "new exe" {
		t.Fatal("rollback lost old exe")
	}
}
func TestUpdaterDownloadStageAndSizeMismatch(t *testing.T) {
	b, digest := updateFixture(t, nil)
	root := t.TempDir()
	if e := os.WriteFile(filepath.Join(root, "MabiDPS-JP.exe"), []byte("MZ-old"), 0600); e != nil {
		t.Fatal(e)
	}
	offer := updateOffer{Version: "1.0.2", Asset: updateAsset{URL: "https://github.com/" + updateRepository + "/releases/download/v1.0.2/MABI-DPS-JP-v1.0.2-Windows.zip", Size: int64(len(b)), Digest: digest}}
	client := &http.Client{Transport: updateTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(b))}, nil
	})}
	stage, e := stageUpdate(context.Background(), client, root, offer, 123)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(stage, "plan.json")); e != nil {
		t.Fatal(e)
	}
	offer.Asset.Size++
	if _, e = stageUpdate(context.Background(), client, root, offer, 123); e == nil {
		t.Fatal("wrong download size accepted")
	}
	entries, e := os.ReadDir(filepath.Join(root, ".updates"))
	if e != nil || len(entries) != 1 {
		t.Fatal("failed staging not cleaned", e)
	}
}
