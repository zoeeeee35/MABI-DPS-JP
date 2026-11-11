//go:build native && windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportPreviousSettings(t *testing.T) {
	source, destination := t.TempDir(), t.TempDir()
	data := filepath.Join(source, "data")
	if err := os.Mkdir(data, 0700); err != nil {
		t.Fatal(err)
	}
	write := func(dir, name, contents string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0600); err != nil {
			t.Fatal(err)
		}
	}
	const overlay = "game-overlay.json"
	write(destination, overlay, `{"Enabled":true}`)
	write(data, overlay, `{"Enabled":false,"IdleSeconds":15}`)
	write(data, "barrier-settings.json", `{"enabled":false,"soundPath":"\\\\example.invalid\\share\\alert.mp3"}`)
	write(data, "capture-last-stop.json", `{"ignored":true}`)
	count, backup, err := importPreviousSettings(source, destination)
	if err != nil || count != 2 || backup == "" {
		t.Fatalf("count=%d backup=%q error=%v", count, backup, err)
	}
	for _, tc := range []struct{ dir, want string }{
		{destination, `{"Enabled":false,"IdleSeconds":15}`},
		{backup, `{"Enabled":true}`},
	} {
		got, err := os.ReadFile(filepath.Join(tc.dir, overlay))
		if err != nil || string(got) != tc.want {
			t.Fatalf("%s: %s, %v", tc.dir, got, err)
		}
	}
	if _, err := os.Stat(filepath.Join(destination, "capture-last-stop.json")); !os.IsNotExist(err) {
		t.Fatalf("unexpected capture log: %v", err)
	}
	settings, err := readBarrierSettings(filepath.Join(destination, "barrier-settings.json"))
	if err != nil || settings.SoundPath != defaultBarrierSound || settings.Enabled {
		t.Fatalf("expected the standard sound and preserved notification settings: %+v, %v", settings, err)
	}
	write(data, "barrier-settings.json", `{"enabled":"invalid"}`)
	write(data, overlay, `{"Enabled":true}`)
	if _, _, err := importPreviousSettings(data, destination); err == nil {
		t.Fatal("invalid input accepted")
	}
	got, err := os.ReadFile(filepath.Join(destination, overlay))
	if err != nil || string(got) != `{"Enabled":false,"IdleSeconds":15}` {
		t.Fatalf("invalid import changed the destination: %s, %v", got, err)
	}
}
