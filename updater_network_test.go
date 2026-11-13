package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Opt-in only. Downloads public release metadata and ZIP, never executes it.
func TestUpdaterPublicReleaseDownload(t *testing.T) {
	if os.Getenv("MABI_UPDATE_NETWORK_TEST") != "1" {
		t.Skip("network integration test disabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	client := updateHTTPClient()
	current, e := fetchUpdate(ctx, client, appVersion())
	if e != nil {
		t.Fatal(e)
	}
	if current != nil {
		t.Log("newer release", current.Version)
	}
	offer, e := fetchUpdate(ctx, client, "0.0.0")
	if e != nil || offer == nil {
		t.Fatal(offer, e)
	}
	root := t.TempDir()
	if e = os.WriteFile(filepath.Join(root, "MabiDPS-JP.exe"), []byte("MZ-local-stand-in"), 0600); e != nil {
		t.Fatal(e)
	}
	stage, e := stageUpdate(ctx, client, root, *offer, 123)
	if e != nil {
		t.Fatal(e)
	}
	files, e := inspectUpdateZIP(filepath.Join(stage, "package.zip"), offer.Version, offer.Asset.Digest)
	if e != nil {
		t.Fatal(e)
	}
	t.Logf("public v%s / %s / %d bytes / %d validated files; no downloaded program executed", offer.Version, offer.Asset.Name, offer.Asset.Size, len(files))
}
