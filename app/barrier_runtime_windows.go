//go:build native && windows

package main

import (
	"context"
	"fmt"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

var barrierConfig = defaultBarrierSettings()
var barrierScheduler BarrierScheduler
var barrierSettingsPath string
var barrierStatus = "日本時間の毎時00分を基準にした予定通知です。"
var barrierBannerWindow uintptr
var barrierBannerEvent BarrierEvent
var barrierBannerAt time.Time
var barrierLastSecond int64 = -1
var barrierHeaderWasVisible bool
var barrierBannerActive bool
var barrierBannerLastAlpha uint8 = 255
var barrierBannerPaintSecond int64 = -1
var barrierBannerLastRect rect
var barrierBannerPresented bool

func initBarrierRuntime() {
	barrierSettingsPath = filepath.Join(filepath.Dir(historyDir), "barrier-settings.json")
	c, e := readBarrierSettings(barrierSettingsPath)
	barrierConfig = c
	if e != nil {
		barrierStatus = "通知設定を読み込めません。初期値で起動しました。"
	} else if c.SoundPath != "" && c.SoundPath != defaultBarrierSound {
		if _, err := localBarrierSound(c.SoundPath); err != nil {
			barrierConfig.SoundPath = defaultBarrierSound
			barrierStatus = "通知音を開けないため、標準音を使います。"
		}
	}
	barrierScheduler.Tick(time.Now(), barrierConfig)
}
func saveBarrierConfig() {
	barrierConfig = normalizeBarrierSettings(barrierConfig)
	if !barrierConfig.Enabled || !barrierConfig.WindowsToast {
		stopBarrierToast()
	}
	if e := writeBarrierSettings(barrierSettingsPath, barrierConfig); e != nil {
		barrierStatus = "通知設定を保存できません"
	}
	for _, event := range barrierScheduler.Tick(time.Now(), barrierConfig) {
		fireBarrierEvent(event, barrierConfig)
	}
	if !barrierConfig.Enabled || !barrierConfig.Overlay {
		barrierBannerActive = false
		show(barrierBannerWindow, false)
	}
	if !barrierConfig.Enabled || !barrierConfig.Sound {
		go stopBarrierAudio()
	}
	invalidate(overlayWindow)
	invalidate(mainWindow)
}
func fireBarrierEvent(e BarrierEvent, c BarrierSettings) {
	if c.Overlay {
		barrierBannerEvent = e
		barrierBannerAt = time.Now()
		barrierBannerActive = true
		barrierBannerPaintSecond = -1
		updateBarrierBanner(barrierBannerAt)
		if gameOverlay.Enabled {
			syncGameOverlay()
		}
	}
	if c.WindowsToast {
		go func() {
			if e := sendBarrierToast(barrierCtx, e); e != nil {
				if e == context.Canceled {
					return
				}
				barrierReport(e.Error())
			} else {
				barrierReport("Windows通知を送信しました（表示はWindows設定に従います）。")
			}
		}()
	}
	if c.Sound {
		go func() {
			if e := playBarrierAudio(c); e != nil {
				barrierReport(e.Error())
			}
		}()
	}
}
func tickBarrierRuntime(now time.Time) {
	for _, event := range barrierScheduler.Tick(now, barrierConfig) {
		fireBarrierEvent(event, barrierConfig)
	}
	select {
	case text := <-barrierResults:
		barrierStatus = text
		if nativePage == "結界通知" {
			invalidate(mainWindow)
		}
	default:
	}
	if now.Unix() != barrierLastSecond {
		barrierLastSecond = now.Unix()
		_, visible := barrierCountdown(now, barrierConfig)
		if visible || barrierHeaderWasVisible {
			invalidate(overlayWindow)
		}
		barrierHeaderWasVisible = visible
	}
	if barrierBannerActive {
		updateBarrierBanner(now)
	}
}
func ensureBarrierBanner() {
	if barrierBannerWindow != 0 {
		return
	}
	brush, _, _ := gdi32.NewProc("GetStockObject").Call(0)
	if e := registerClass("MabiDPSBarrierBanner", syscall.NewCallback(barrierBannerProc), brush); e != nil {
		barrierStatus = e.Error()
		return
	}
	barrierBannerWindow = createWindow("MabiDPSBarrierBanner", "MABI DPS JP 結界通知", exTopmost|exToolwindow|exLayered|exNoActivate|exTransparent, 0x80000000, 0, 0, 340, 78, 0, 0)
}
func updateBarrierBanner(now time.Time) {
	end := barrierBannerEnd(barrierConfig, barrierBannerEvent, barrierBannerAt)
	age := now.Sub(barrierBannerAt)
	if !now.Before(end) {
		barrierBannerActive = false
		barrierBannerPresented = false
		if barrierBannerWindow != 0 {
			user32.NewProc("KillTimer").Call(barrierBannerWindow, 2)
			show(barrierBannerWindow, false)
		}
		return
	}
	if gameOverlay.Enabled {
		// Mini overlays already update on the main window timer.
		if barrierBannerWindow != 0 {
			user32.NewProc("KillTimer").Call(barrierBannerWindow, 2)
			if barrierBannerPresented {
				show(barrierBannerWindow, false)
				barrierBannerPresented = false
			}
		}
		return
	}
	ensureBarrierBanner()
	if barrierBannerWindow == 0 {
		return
	}
	anchor := overlayScreenBounds(overlayWindow)
	monitor, _, _ := user32.NewProc("MonitorFromWindow").Call(overlayWindow, 2)
	info := struct {
		Size          uint32
		Monitor, Work rect
		Flags         uint32
	}{}
	info.Size = uint32(unsafe.Sizeof(info))
	if ok, _, _ := user32.NewProc("GetMonitorInfoW").Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
		w, _, _ := pMetrics.Call(0)
		h, _, _ := pMetrics.Call(1)
		info.Work = rect{0, 0, int32(w), int32(h)}
	}
	placed := barrierBannerPlacement(anchor, info.Work)
	move := !barrierBannerPresented || placed != barrierBannerLastRect
	alpha := barrierFadeDuration(age, end.Sub(barrierBannerAt))
	repaint := move || alpha != barrierBannerLastAlpha
	if repaint {
		user32.NewProc("SetLayeredWindowAttributes").Call(barrierBannerWindow, 0, uintptr(alpha), 2)
		barrierBannerLastAlpha = alpha
	}
	if move {
		if !barrierBannerPresented {
			pSetTimer.Call(barrierBannerWindow, 2, 25, 0)
		}
		pSetWindowPos.Call(barrierBannerWindow, ^uintptr(0), signed(placed.Left), signed(placed.Top), uintptr(placed.Right-placed.Left), uintptr(placed.Bottom-placed.Top), 0x0010|0x0040)
		barrierBannerLastRect = placed
		barrierBannerPresented = true
	}
	if now.Unix() != barrierBannerPaintSecond {
		barrierBannerPaintSecond = now.Unix()
		repaint = true
	}
	if repaint {
		invalidate(barrierBannerWindow)
	}
}
func paintBarrierBanner(window, dc uintptr) {
	var r rect
	pGetClient.Call(window, uintptr(unsafe.Pointer(&r)))
	save, _, _ := gdi32.NewProc("SaveDC").Call(dc)
	defer gdi32.NewProc("RestoreDC").Call(dc, save)
	gdi32.NewProc("SetMapMode").Call(dc, 8)
	gdi32.NewProc("SetWindowExtEx").Call(dc, 360, 78, 0)
	gdi32.NewProc("SetViewportExtEx").Call(dc, uintptr(r.Right), uintptr(r.Bottom), 0)
	fill(dc, rect{0, 0, 360, 78}, 0x211911)
	fantasyCard(dc, rect{3, 3, 357, 75}, 0x413326, 0x2B2119, 28)
	fill(dc, rect{9, 16, 13, 62}, 0x4DB5F5)
	skinIcon(dc, "bell", 26, 24, 26, 0xDDD7CA)
	ntxt(dc, "召喚の結界", rect{68, 12, 341, 38}, 0xF5F2EB, 17)
	if barrierBannerEvent.Kind == "start" {
		ntxt(dc, "結界が始まりました", rect{68, 40, 341, 66}, 0x70C6FF, 15)
	} else {
		remaining := max(0, int((barrierBannerEvent.Target.Sub(time.Now())+time.Second-1)/time.Second))
		ntxt(dc, fmt.Sprintf("開始まで  %02d:%02d", remaining/60, remaining%60), rect{68, 39, 341, 67}, 0x70C6FF, 18)
	}
}
func barrierBannerProc(window uintptr, msg uint32, w, l uintptr) uintptr {
	switch msg {
	case wmTimer:
		if barrierBannerActive {
			updateBarrierBanner(time.Now())
		} else {
			user32.NewProc("KillTimer").Call(window, 2)
		}
		return 0
	case 0x0317, 0x0318:
		paintBuffered(window, w, paintBarrierBanner)
		return 0
	case wmPaint:
		var ps paintStruct
		dc, _, _ := pBeginPaint.Call(window, uintptr(unsafe.Pointer(&ps)))
		paintBuffered(window, dc, paintBarrierBanner)
		pEndPaint.Call(window, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmErase:
		return 1
	case wmMouseActivate:
		return 3
	case 0x0084:
		return ^uintptr(0)
	case wmDestroy:
		releasePaintBuffer(window)
		return 0
	}
	v, _, _ := pDefWindow.Call(window, uintptr(msg), w, l)
	return v
}
func selectBarrierSound() {
	buffer := make([]uint16, 32768)
	filter := []uint16{}
	for _, part := range []string{"通知音 (*.wav;*.mp3)", "*.wav;*.mp3", ""} {
		v, _ := syscall.UTF16FromString(part)
		filter = append(filter, v...)
	}
	ofn := openFileName{Owner: mainWindow, Filter: &filter[0], File: &buffer[0], MaxFile: uint32(len(buffer)), FilterIndex: 1, Flags: 0x00080000 | 0x00001000 | 0x00000008}
	ofn.Size = uint32(unsafe.Sizeof(ofn))
	if result, _, _ := pGetOpen.Call(uintptr(unsafe.Pointer(&ofn))); result != 0 {
		path, err := localBarrierSound(syscall.UTF16ToString(buffer))
		if err != nil {
			barrierStatus = err.Error()
			return
		}
		barrierConfig.SoundPath = path
		saveBarrierConfig()
	}
}
func barrierSettingClick(action string, index int) bool {
	switch action {
	case "barrier-master":
		barrierConfig.Enabled = !barrierConfig.Enabled
	case "barrier-start":
		barrierConfig.AtStart = !barrierConfig.AtStart
	case "barrier-windows":
		barrierConfig.WindowsToast = !barrierConfig.WindowsToast
	case "barrier-sound":
		barrierConfig.Sound = !barrierConfig.Sound
	case "barrier-overlay":
		barrierConfig.Overlay = !barrierConfig.Overlay
	case "barrier-always":
		barrierConfig.AlwaysCountdown = !barrierConfig.AlwaysCountdown
	case "barrier-keep-before":
		barrierConfig.KeepBeforeBanner = !barrierConfig.KeepBeforeBanner
	case "barrier-banner-seconds":
		openNumericSetting("開始バナーの表示時間", "表示時間（1〜30秒）", barrierConfig.BannerSeconds, 1, 30, func(value int) {
			barrierConfig.BannerSeconds = value
			saveBarrierConfig()
		})
		return true
	case "barrier-lead":
		if index >= 0 && index < len(barrierLeadChoices) {
			barrierConfig.LeadMinutes = barrierLeadChoices[index]
		}
	case "barrier-browse":
		selectBarrierSound()
		return true
	case "barrier-default-sound":
		barrierConfig.SoundPath = defaultBarrierSound
	case "barrier-play":
		c := barrierConfig
		go func() {
			if e := playBarrierAudio(c); e != nil {
				barrierReport(e.Error())
			} else {
				barrierReport("通知音を試聴しています。")
			}
		}()
		return true
	case "barrier-test-before", "barrier-test-start":
		kind := "before"
		if action == "barrier-test-start" {
			kind = "start"
		}
		c := barrierConfig
		c.WindowsToast = false
		c.Sound = false
		c.Overlay = true
		previewTime := 4*time.Minute + 32*time.Second
		if kind == "before" && c.KeepBeforeBanner {
			previewTime = 12 * time.Second
		}
		fireBarrierEvent(BarrierEvent{kind, time.Now().Add(previewTime), c.LeadMinutes}, c)
		return true
	case "barrier-test-windows":
		c := barrierConfig
		c.Overlay = false
		c.Sound = false
		c.WindowsToast = true
		fireBarrierEvent(BarrierEvent{"before", nextBarrierTime(time.Now()), c.LeadMinutes}, c)
		return true
	default:
		return false
	}
	saveBarrierConfig()
	return true
}
func nativeBarrierPanel(dc uintptr, left, right, y int32) {
	fantasyCard(dc, rect{left, y, right, y + 371}, 0x30251B, 0x241B14, 20)
	ntxt(dc, "召喚の結界通知 / 毎時00分（日本時間）の予定", rect{left + 18, y + 14, right - 18, y + 42}, 0xEEEAE3, 16)
	nativeToggleButton(dc, rect{left + 18, y + 52, left + 260, y + 88}, "結界通知", "barrier-master", barrierConfig.Enabled, 0x527FB0)
	ntxt(dc, "事前通知", rect{left + 18, y + 101, left + 120, y + 129}, 0xC8BBAB, 13)
	width := (right - left - 150) / 6
	for i, m := range barrierLeadChoices {
		x := left + 130 + int32(i)*width
		nbutton(dc, rect{x, y + 96, x + width - 7, y + 132}, fmt.Sprintf("%d分前", m), "barrier-lead", m == barrierConfig.LeadMinutes, i)
	}
	half := (right - left - 48) / 2
	for i, v := range []struct {
		label, action string
		on            bool
	}{{"Windows通知", "barrier-windows", barrierConfig.WindowsToast}, {"開始時通知", "barrier-start", barrierConfig.AtStart}, {"通知音", "barrier-sound", barrierConfig.Sound}, {"オーバーレイ", "barrier-overlay", barrierConfig.Overlay}} {
		x := left + 18 + int32(i%2)*(half+12)
		yy := y + 148 + int32(i/2)*48
		nativeToggleButton(dc, rect{x, yy, x + half, yy + 36}, v.label, v.action, v.on, 0x527FB0)
	}
	nativeToggleButton(dc, rect{left + 18, y + 247, left + 315, y + 283}, "常時カウントダウン", "barrier-always", barrierConfig.AlwaysCountdown, 0x527FB0)
	ntxt(dc, "OFF：事前通知の時刻から表示", rect{left + 335, y + 256, right - 18, y + 283}, 0xB4A394, 12)
	nativeToggleButton(dc, rect{left + 18, y + 295, left + 348, y + 331}, "事前バナーを開始まで表示", "barrier-keep-before", barrierConfig.KeepBeforeBanner, 0x527FB0)
	nbutton(dc, rect{left + 360, y + 295, left + 628, y + 331}, fmt.Sprintf("開始バナーの表示時間  %d秒", barrierConfig.BannerSeconds), "barrier-banner-seconds", false, 0)
	ntxt(dc, "日本時間の毎時00分を基準にした予定通知です。", rect{left + 18, y + 337, right - 18, y + 364}, 0xB6A597, 12)
	nbutton(dc, rect{left + 18, y + 382, left + 280, y + 420}, settingsDisclosure("通知音と動作テスト", nativeNoticeDetails), "settings-notice-details", nativeNoticeDetails, 0)
	if !nativeNoticeDetails {
		return
	}
	y += 142
	fantasyCard(dc, rect{left, y + 291, right, y + 550}, 0x30251B, 0x241B14, 20)
	nbutton(dc, rect{left + 18, y + 301, left + 157, y + 338}, "音声を選択", "barrier-browse", false, 0)
	nbutton(dc, rect{left + 169, y + 301, left + 284, y + 338}, "試聴", "barrier-play", false, 0)
	nbutton(dc, rect{left + 296, y + 301, left + 431, y + 338}, "標準音へ戻す", "barrier-default-sound", false, 0)
	path := barrierConfig.SoundPath
	if path == "" {
		path = defaultBarrierSound
	}
	ntxt(dc, path, rect{left + 449, y + 309, right - 18, y + 337}, 0xC4B6A5, 12)
	ntxt(dc, fmt.Sprintf("音量 %d%%", barrierConfig.Volume), rect{left + 18, y + 357, left + 145, y + 387}, 0xE9E1D5, 14)
	track := rect{left + 155, y + 365, right - 35, y + 374}
	fill(dc, track, 0x44382C)
	fill(dc, rect{track.Left, track.Top, track.Left + (track.Right-track.Left)*int32(barrierConfig.Volume)/100, track.Bottom}, 0x55B6EE)
	hit := rect{track.Left, max(nativeBodyTop, y+347), track.Right, min(nativeBottom, y+391)}
	if hit.Bottom > hit.Top {
		nativeHits = append(nativeHits, nativeHit{hit, "barrier-volume", 0})
	}
	for i, v := range []struct{ label, action string }{{"事前バナーを試す", "barrier-test-before"}, {"開始バナーを試す", "barrier-test-start"}, {"Windows通知を試す", "barrier-test-windows"}} {
		x := left + 18 + int32(i)*((right-left-36)/3)
		nbutton(dc, rect{x, y + 405, x + (right-left-36)/3 - 10, y + 441}, v.label, v.action, false, 0)
	}
	ntxt(dc, barrierStatus, rect{left + 18, y + 454, right - 18, y + 485}, 0xB5D7BE, 12)
	ntxt(dc, "予定通知です。実際のサーバー開催を検知する機能ではありません。", rect{left + 18, y + 487, right - 18, y + 515}, 0xB6A597, 12)
	ntxt(dc, "通知音はWindows通知と二重再生しません。設定は自動保存。", rect{left + 18, y + 515, right - 18, y + 543}, 0xB6A597, 12)
}

func barrierBannerPlacement(anchor, work rect) rect {
	width := min(max(int32(300), anchor.Right-anchor.Left), max(int32(1), work.Right-work.Left))
	height := max(int32(65), width*78/360)
	x := max(work.Left, min(anchor.Left, work.Right-width))
	y := anchor.Top - height - 8
	if y < work.Top {
		y = anchor.Bottom + 8
	}
	if y+height > work.Bottom {
		y = work.Top + 8
		x = max(work.Left, work.Right-width-8)
	}
	return rect{x, y, x + width, y + height}
}
