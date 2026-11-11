//go:build native && windows

package main

import (
	"fmt"
	"path/filepath"
	"time"
	"unsafe"
)

// QA uses only local rendering/configuration. It never sends Windows toasts or plays audio.
func barrierUIReview(dir string) map[string]any {
	oldConfig, oldPane, oldPage, oldScroll := barrierConfig, nativeSettingsTab, nativePage, nativeScroll
	oldNoticeDetails := nativeNoticeDetails
	oldStatus := barrierStatus
	oldScope, oldHistory := displayCaptureTotal, historyIndex
	defer func() { displayCaptureTotal, historyIndex = oldScope, oldHistory }()
	defer func() {
		barrierConfig, nativeSettingsTab, nativePage, nativeScroll = oldConfig, oldPane, oldPage, oldScroll
		nativeNoticeDetails = oldNoticeDetails
		barrierStatus = oldStatus
		barrierBannerActive = false
		show(barrierBannerWindow, false)
		_ = writeBarrierSettings(barrierSettingsPath, oldConfig)
	}()
	barrierConfig = defaultBarrierSettings()
	barrierConfig.Enabled = true
	barrierConfig.WindowsToast = false
	barrierConfig.Sound = false
	barrierConfig.AlwaysCountdown = true
	openNativeSettings(settingsNotice)
	nativeNoticeDetails = false
	nativeScroll = 0
	_ = captureOwnSurface(filepath.Join(dir, "barrier-settings.png"), mainWindow, true)
	click := func(action string) bool {
		for _, h := range nativeHits {
			if h.Action == action {
				nativeModernClick((h.R.Left+h.R.Right)/2, (h.R.Top+h.R.Bottom)/2)
				return true
			}
		}
		return false
	}
	hasAction := func(action string) bool {
		for _, h := range nativeHits {
			if h.Action == action {
				return true
			}
		}
		return false
	}
	settingsUnified := nativePage == "結界通知" && !hasAction("settings-section")
	displayCaptureTotal, historyIndex = false, -1
	scopeExplicit := click("scope-all") && displayCaptureTotal && historyIndex < 0
	_ = captureOwnSurface(filepath.Join(dir, "scope-all.png"), mainWindow, true)
	scopeExplicit = scopeExplicit && click("scope-current") && !displayCaptureTotal
	_ = captureOwnSurface(filepath.Join(dir, "scope-current.png"), mainWindow, true)
	openNativeSettings(settingsDisplay)
	_ = captureOwnSurface(filepath.Join(dir, "overlay-display-settings.png"), mainWindow, true)
	overlaySettings := nativePage == "オーバーレイ" && !hasAction("settings-section")
	overlaySettings = overlaySettings && click("settings") && nativePage == "設定" && nativeSettingsTab == settingsMeasure
	_ = captureOwnSurface(filepath.Join(dir, "capture-settings.png"), mainWindow, true)
	overlaySettings = overlaySettings && !hasAction("settings-import") && click("settings-section-other") && nativeSettingsTab == settingsOther
	_ = captureOwnSurface(filepath.Join(dir, "other-settings.png"), mainWindow, true)
	overlaySettings = overlaySettings && hasAction("settings-import") && click("page:結界通知") && nativePage == "結界通知"
	_ = captureOwnSurface(filepath.Join(dir, "overlay-barrier-settings.png"), mainWindow, true)
	headerToggle := false
	for _, h := range nativeHits {
		if h.Action == "barrier-master" && h.R.Top == 101 {
			headerToggle = true
		}
	}
	before := barrierConfig.Enabled
	toggle := click("barrier-master") && barrierConfig.Enabled != before
	persisted, e := readBarrierSettings(barrierSettingsPath)
	saved := e == nil && persisted.Enabled == barrierConfig.Enabled
	barrierConfig.Enabled = true
	barrierConfig.WindowsToast = false
	barrierConfig.Sound = false
	barrierConfig.AlwaysCountdown = true
	oldRun, oldEnd := soloMeasurement, soloTimerEnd
	countdownRenders := map[string]bool{}
	for _, n := range []int{5, 4, 3, 2, 1} {
		origin := time.Now()
		soloMeasurement = &soloTimedRun{Live: &LiveInput{origin: origin}, Start: int64(n * 1000), End: int64(n*1000 + 60000)}
		soloTimerEnd = origin.Add(time.Duration(n)*time.Second + time.Minute)
		name := fmt.Sprintf("overlay-countdown-%d.png", n)
		countdownRenders[name] = captureOwnSurface(filepath.Join(dir, name), overlayWindow, false) == nil
	}
	soloMeasurement, soloTimerEnd = oldRun, oldEnd
	var original rect
	user32.NewProc("GetWindowRect").Call(overlayWindow, uintptr(unsafe.Pointer(&original)))
	oldSized := overlayUserSized
	overlayUserSized = true
	defer func() {
		pSetWindowPos.Call(overlayWindow, 0, signed(original.Left), signed(original.Top), uintptr(original.Right-original.Left), uintptr(original.Bottom-original.Top), 0x0014)
		overlayUserSized = oldSized
	}()
	renders := map[string]bool{}
	for _, s := range []struct {
		name string
		w, h int32
	}{{"overlay-fantasy-100.png", 360, 400}, {"overlay-fantasy-125.png", 450, 500}, {"overlay-fantasy-150.png", 540, 600}} {
		pSetWindowPos.Call(overlayWindow, 0, 0, 0, uintptr(s.w), uintptr(s.h), 0x0016)
		renders[s.name] = captureOwnSurface(filepath.Join(dir, s.name), overlayWindow, false) == nil
	}
	pSetWindowPos.Call(overlayWindow, 0, 0, 0, 360, 400, 0x0016)
	foreground, _, _ := user32.NewProc("GetForegroundWindow").Call()
	barrierBannerEvent = BarrierEvent{"before", time.Now().Add(4*time.Minute + 32*time.Second), 5}
	barrierBannerAt = time.Now().Add(-time.Second)
	barrierBannerActive = true
	ensureBarrierBanner()
	updateBarrierBanner(time.Now())
	bannerRendered := captureOwnSurface(filepath.Join(dir, "barrier-before.png"), barrierBannerWindow, false) == nil
	after, _, _ := user32.NewProc("GetForegroundWindow").Call()
	focusPreserved := foreground == after
	style, _, _ := pGetLong.Call(barrierBannerWindow, exIndex())
	noActivate := style&exNoActivate != 0
	through := style&exTransparent != 0
	barrierBannerEvent.Kind = "start"
	_ = captureOwnSurface(filepath.Join(dir, "barrier-start.png"), barrierBannerWindow, false)
	barrierBannerAt = time.Now().Add(-9 * time.Second)
	updateBarrierBanner(time.Now())
	visible, _, _ := user32.NewProc("IsWindowVisible").Call(barrierBannerWindow)
	dc, _, _ := user32.NewProc("GetDC").Call(overlayWindow)
	process, _, _ := kernel32.NewProc("GetCurrentProcess").Call()
	beforeGDI, _, _ := user32.NewProc("GetGuiResources").Call(process, 0)
	began := time.Now()
	for i := 0; i < 32; i++ {
		paintBuffered(overlayWindow, dc, paintOverlayContent)
	}
	renderMS := float64(time.Since(began).Microseconds()) / 32000
	afterGDI, _, _ := user32.NewProc("GetGuiResources").Call(process, 0)
	user32.NewProc("ReleaseDC").Call(overlayWindow, dc)
	return map[string]any{"gdiBefore": beforeGDI, "gdiAfter": afterGDI, "gdiStable": afterGDI <= beforeGDI+1, "averageRenderMS": renderMS, "countdownRenders": countdownRenders, "settingsUnified": settingsUnified, "scopeExplicitSelection": scopeExplicit, "masterToggle": toggle, "headerTogglePresent": headerToggle, "overlaySettingsAccessible": overlaySettings, "settingsPersisted": saved, "bannerRendered": bannerRendered, "bannerClickThrough": through, "bannerNoActivate": noActivate, "focusPreserved": focusPreserved, "bannerAutoHidden": visible == 0, "layoutSizes": renders, "liveServerEventDetected": false, "toastSent": false, "audioPlayed": false}
}
