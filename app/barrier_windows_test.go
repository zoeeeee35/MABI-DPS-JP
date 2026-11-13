//go:build native && windows

package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBarrierNativeSoundPathValidation(t *testing.T) {
	c := defaultBarrierSettings()
	c.SoundPath = filepath.Join(t.TempDir(), "absent.mp3")
	if playBarrierAudio(c) == nil {
		t.Fatal("missing sound accepted")
	}
	c.SoundPath = "bad.exe"
	if playBarrierAudio(c) == nil {
		t.Fatal("bad type")
	}
}
func TestBarrierShortcutLocalPersistence(t *testing.T) {
	if os.Getenv("MABI_TEST_SHORTCUT") == "" {
		t.Skip("explicit local COM verification")
	}
	path := filepath.Join(t.TempDir(), "MABI DPS JP.lnk")
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	if e = createBarrierShortcut(path, exe); e != nil {
		t.Fatal(e)
	}
	if !barrierShortcutCurrent(path, exe) {
		t.Fatal("shortcut identity not persisted")
	}
	data, e := os.ReadFile(path)
	if e != nil || len(data) < 76 || binary.LittleEndian.Uint32(data) != 76 {
		t.Fatal("invalid shortcut", e)
	}
}
func TestBarrierToastEscaping(t *testing.T) {
	if got := xmlBarrierText("<x>&\"'"); got != "&lt;x&gt;&amp;&quot;&apos;" {
		t.Fatal(got)
	}
	if strings.Contains(encodedPowerShell("日本語"), "日本語") {
		t.Fatal("not encoded")
	}
}
func TestBarrierFadeAlwaysBounded(t *testing.T) {
	for i := -500; i < 9000; i += 17 {
		v := barrierFade(time.Duration(i)*time.Millisecond, 8)
		if v > 245 {
			t.Fatal(v)
		}
	}
}

func TestBarrierMultiMonitorPlacement(t *testing.T) {
	for _, work := range []rect{{0, 0, 1920, 1080}, {-1920, 0, 0, 1080}, {0, -1440, 2560, 0}} {
		anchor := rect{work.Left + 70, work.Top + 20, work.Left + 430, work.Top + 420}
		r := barrierBannerPlacement(anchor, work)
		if r.Left < work.Left || r.Right > work.Right || r.Top < work.Top || r.Bottom > work.Bottom {
			t.Fatal(work, r)
		}
		if r.Top < anchor.Bottom && r.Bottom > anchor.Top && r.Left < anchor.Right && r.Right > anchor.Left {
			t.Fatal("banner overlaps DPS", r, anchor)
		}
	}
}

func TestBarrierToastDistinctDisabledReasons(t *testing.T) {
	for code, want := range map[int]string{21: "個別", 22: "ユーザー", 23: "管理ポリシー", 24: "登録", 25: "未知", 1: "終了コード1"} {
		if got := barrierToastExitError(code).Error(); !strings.Contains(got, want) {
			t.Fatalf("%d: %s", code, got)
		}
	}
}

func TestBarrierBundledSoundPortablePath(t *testing.T) {
	base := `C:\MABI DPS JP`
	want := filepath.Join(base, filepath.FromSlash(defaultBarrierSound))
	if got := barrierSoundFileAt(base, ""); got != want {
		t.Fatal(got, want)
	}
	if got := barrierSoundFileAt(base, defaultBarrierSound); got != want {
		t.Fatal(got, want)
	}
	custom := `D:\音声\custom.mp3`
	if got := barrierSoundFileAt(base, custom); got != custom {
		t.Fatal(got)
	}
	if defaultBarrierSettings().SoundPath != defaultBarrierSound {
		t.Fatal("default audio not configured")
	}
}

func TestLocalBarrierSoundBlocksRemoteAndDevicePaths(t *testing.T) {
	for _, path := range []string{
		`\\example.invalid\share\alert.mp3`, `//example.invalid/share/alert.mp3`,
		`\\?\UNC\example.invalid\share\alert.mp3`, `\\.\C:\alert.mp3`,
		`C:alert.mp3`, `https://example.invalid/alert.mp3`, `alert.mp3:other.wav`,
	} {
		if _, err := localBarrierSound(path); err == nil || !strings.Contains(err.Error(), "ローカルドライブ") {
			t.Errorf("expected path rejection for %q, got %v", path, err)
		}
	}
	path := filepath.Join(t.TempDir(), "通知音.mp3")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := localBarrierSound(path); err != nil || got != path {
		t.Fatalf("local file rejected: %q, %v", got, err)
	}
}

func TestSoloPreparationUsesLiveOrigin(t *testing.T) {
	old := soloMeasurement
	defer func() { soloMeasurement = old }()
	now := time.Now()
	soloMeasurement = &soloTimedRun{Live: &LiveInput{origin: now}, Start: 5000, End: 65000}
	for i := 0; i <= 5; i++ {
		if got := soloPreparationRemaining(now.Add(time.Duration(i) * time.Second)); got != 5-i {
			t.Fatal(i, got)
		}
	}
	soloMeasurement = nil
	if soloPreparationRemaining(now) != 0 {
		t.Fatal("countdown survived cancellation")
	}
}

func TestMiniOverlayIndependentBounds(t *testing.T) {
	client := rect{-1200, 40, 0, 840}
	dps := defaultMiniCard(1000, 1000)
	barrier := defaultMiniCard(0, 0)
	barrier.Size = 125
	dpsRect := miniOverlayBounds(client, dps, miniWidth, miniDPSHeight)
	timerRect := miniOverlayBounds(client, barrier, miniBarrierWidth, miniBarrierHeight)
	if dpsRect.Right != client.Right || dpsRect.Bottom != client.Bottom ||
		timerRect.Left != client.Left || timerRect.Top != client.Top ||
		timerRect.Right-timerRect.Left != 90 {
		t.Fatalf("DPS=%+v barrier=%+v", dpsRect, timerRect)
	}
}

func TestMiniOverlayBackgroundDoesNotDimText(t *testing.T) {
	for _, opacity := range []uint32{0, 64, 128, 255} {
		if pixel := miniBlendPixel(0x2E2419, 0x70C9FF, opacity, 255); pixel != 0xFFFFC970 {
			t.Fatalf("text changed at background alpha %d: %08x", opacity, pixel)
		}
	}
	if pixel := miniBlendPixel(0xFFFFFF, 0, 0, 0); pixel != 0 {
		t.Fatalf("transparent background is %08x", pixel)
	}
}

func TestMiniOverlayLayeredWindowRender(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	window := createWindow("STATIC", "", exLayered|exNoActivate, 0x80000000, 0, 0, 252, 82, 0, 0)
	if window == 0 {
		t.Fatal("could not create layered test window")
	}
	defer pDestroy.Call(window)
	prefs := defaultMiniCard(0, 0)
	prefs.BackgroundOpacity = 0
	for _, tc := range []struct {
		bounds  rect
		content miniOverlayContent
	}{
		{rect{20, 20, 272, 102}, miniOverlayContent{title: "DPS", value: "13,852", elapsed: "01:47.4"}},
		{rect{30, 30, 102, 84}, miniOverlayContent{value: barrierStartedLabel}},
		{rect{40, 40, 230, 74}, miniOverlayContent{value: "結界まで 5分前", notice: true}},
	} {
		if !renderMiniOverlay(window, tc.bounds, prefs, tc.content) {
			t.Fatal("layered rendering failed", tc)
		}
		if got := overlayScreenBounds(window); got != tc.bounds {
			t.Fatalf("got %+v, want %+v", got, tc.bounds)
		}
	}
}
