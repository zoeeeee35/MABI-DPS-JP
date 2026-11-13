//go:build native && windows

package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const miniWidth int32 = 252
const miniDPSHeight int32 = 82
const miniBarrierWidth int32 = 72
const miniBarrierHeight int32 = 54

type miniCardPreferences struct {
	X, Y                                                      int32 // Fraction of available travel, in thousandths.
	Size, Font, BackgroundOpacity, TextColor, BackgroundColor int
}

type gameOverlayPreferences struct {
	Enabled             bool
	HideWithoutHUD      bool
	CursorLock          bool  `json:",omitempty"`
	IdleSeconds         int64 `json:",omitempty"`
	miniCardPreferences       // Preserve the position saved by the previous version.
	Barrier             miniCardPreferences
	Notice              miniCardPreferences
	NoticePosition      *point `json:",omitempty"`
}

const miniSizeMin, miniSizeMax = 50, 250

var miniFonts = [...]string{"Yu Gothic UI", "Meiryo", "Rounded Mplus 1c Bold"}
var miniFontLabels = [...]string{"游ゴシック", "メイリオ", "丸ゴシック"}
var miniOpacities = [...]int{0, 25, 50, 75, 100}
var miniTextColors = [...]uint32{0x70C9FF, 0xFFFFFF, 0xFFD698, 0xA9EBA0, 0xF8BBEE}
var miniTextColorLabels = [...]string{"金", "白", "水色", "緑", "紫"}
var miniBackgroundColors = [...]uint32{0x2E2419, 0x151515, 0x382445, 0x2B3C1B}
var miniBackgroundLabels = [...]string{"濃紺", "黒", "紫", "緑"}

func defaultMiniCard(x, y int32) miniCardPreferences {
	return miniCardPreferences{X: x, Y: y, Size: 100, BackgroundOpacity: 3}
}

func defaultMiniNotice() miniCardPreferences {
	prefs := defaultMiniCard(500, 0)
	prefs.TextColor = 1
	return prefs
}

func (p *miniCardPreferences) normalize() {
	p.X, p.Y = min(1000, max(0, p.X)), min(1000, max(0, p.Y))
	// Older releases stored the index of one of four presets.
	if p.Size >= 0 && p.Size <= 3 {
		p.Size = 75 + 25*p.Size
	}
	p.Size = min(miniSizeMax, max(miniSizeMin, p.Size))
	p.Font = min(len(miniFonts)-1, max(0, p.Font))
	p.BackgroundOpacity = min(len(miniOpacities)-1, max(0, p.BackgroundOpacity))
	p.TextColor = min(len(miniTextColors)-1, max(0, p.TextColor))
	p.BackgroundColor = min(len(miniBackgroundColors)-1, max(0, p.BackgroundColor))
}

var (
	gameOverlay                            = gameOverlayPreferences{miniCardPreferences: defaultMiniCard(940, 580), Barrier: defaultMiniCard(940, 460), Notice: defaultMiniNotice()}
	gameOverlayAdjust, gameOverlayDragging bool
	gameOverlaySyncing                     bool
	gameOverlayForeground                  uintptr
	gameOverlayForegroundPID               uint32
	gameOverlayGame                        bool
	gameOverlayTarget                      uintptr
	gameOverlayTargetPID                   uint32
	gameOverlayLastSearch                  time.Time
	miniDPS, miniBarrier, miniNotice       miniOverlayWindow
)

func gameOverlayPath() string {
	return filepath.Join(filepath.Dir(historyDir), "game-overlay.json")
}

func overlayScreenBounds(window uintptr) rect {
	var bounds rect
	user32.NewProc("GetWindowRect").Call(window, uintptr(unsafe.Pointer(&bounds)))
	return bounds
}

func loadGameOverlay() {
	data, err := readBoundedFile(gameOverlayPath(), 64<<10)
	if err != nil {
		return
	}
	prefs := gameOverlay
	prefs.Notice = defaultMiniNotice()
	var legacy struct{ Barrier *miniCardPreferences }
	_ = json.Unmarshal(data, &legacy)
	if json.Unmarshal(data, &prefs) == nil {
		prefs.miniCardPreferences.normalize()
		if legacy.Barrier == nil {
			prefs.Barrier.X, prefs.Barrier.Y = prefs.X, max(0, prefs.Y-120)
			if prefs.Y < 120 {
				prefs.Barrier.Y = prefs.Y + 180
			}
		}
		prefs.Barrier.normalize()
		prefs.Notice.normalize()
		if pos := prefs.NoticePosition; pos != nil {
			pos.X, pos.Y = min(1000, max(0, pos.X)), min(1000, max(0, pos.Y))
		}
		gameOverlay = prefs
		if prefs.IdleSeconds >= 1 && prefs.IdleSeconds <= 60 {
			idle = prefs.IdleSeconds * 1000
		}
	}
}

func saveGameOverlay() {
	data, err := json.Marshal(gameOverlay)
	if err == nil {
		err = os.WriteFile(gameOverlayPath(), data, 0600)
	}
	if err != nil {
		errorText = "ゲーム内表示の設定を保存できません。"
		invalidate(mainWindow)
	}
}

func setGameOverlay(enabled bool) {
	if gameOverlay.Enabled == enabled {
		return
	}
	gameOverlay.Enabled = enabled
	gameOverlayAdjust = false
	saveGameOverlay()
	updateOverlayStyle()
	updateDisplay()
}

func overlayPassesClicks() bool {
	if gameOverlay.Enabled {
		return !gameOverlayAdjust
	}
	return clickThrough
}
func gameWindowPID(window uintptr) uint32 {
	var pid uint32
	user32.NewProc("GetWindowThreadProcessId").Call(window, uintptr(unsafe.Pointer(&pid)))
	return pid
}

func isGameProcess(pid uint32) bool {
	if pid == 0 || pid == uint32(os.Getpid()) {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	var path [1024]uint16
	length := uint32(len(path))
	if windows.QueryFullProcessImageName(handle, 0, &path[0], &length) != nil {
		return false
	}
	exe := filepath.Base(windows.UTF16ToString(path[:length]))
	profile, _ := profileFor("jp")
	for _, name := range profile.ProcessNames {
		if strings.EqualFold(exe, name) {
			return true
		}
	}
	return false
}

func gameOverlayClient() (rect, bool) {
	window, _, _ := user32.NewProc("GetForegroundWindow").Call()
	pid := gameWindowPID(window)
	if window != gameOverlayForeground || pid != gameOverlayForegroundPID {
		gameOverlayForeground, gameOverlayForegroundPID = window, pid
		gameOverlayGame = isGameProcess(pid)
	}
	if gameOverlayGame {
		if client, ok := gameClientBounds(window); ok {
			gameOverlayTarget, gameOverlayTargetPID = window, pid
			return client, true
		}
	}
	if !gameOverlayAdjust {
		return rect{}, false
	}
	if gameOverlayTarget != 0 && gameWindowPID(gameOverlayTarget) == gameOverlayTargetPID {
		if client, ok := gameClientBounds(gameOverlayTarget); ok {
			return client, true
		}
	}
	gameOverlayTarget, gameOverlayTargetPID = 0, 0
	if time.Since(gameOverlayLastSearch) >= time.Second {
		gameOverlayLastSearch = time.Now()
		// EnumWindows runs synchronously on the UI thread.
		_ = windows.EnumWindows(findGameOverlayWindow, nil)
	}
	return gameClientBounds(gameOverlayTarget)
}

var findGameOverlayWindow = syscall.NewCallback(func(window, _ uintptr) uintptr {
	if _, ok := gameClientBounds(window); ok {
		if pid := gameWindowPID(window); isGameProcess(pid) {
			gameOverlayTarget, gameOverlayTargetPID = window, pid
			return 0
		}
	}
	return 1
})

func gameClientBounds(window uintptr) (rect, bool) {
	if window == 0 {
		return rect{}, false
	}
	visible, _, _ := user32.NewProc("IsWindowVisible").Call(window)
	minimized, _, _ := user32.NewProc("IsIconic").Call(window)
	if visible == 0 || minimized != 0 {
		return rect{}, false
	}
	var client rect
	if ok, _, _ := pGetClient.Call(window, uintptr(unsafe.Pointer(&client))); ok == 0 {
		return rect{}, false
	}
	if client.Right < 160 || client.Bottom < 64 {
		return rect{}, false
	}
	var origin point
	if ok, _, _ := user32.NewProc("ClientToScreen").Call(window, uintptr(unsafe.Pointer(&origin))); ok == 0 {
		return rect{}, false
	}
	return rect{origin.X, origin.Y, origin.X + client.Right, origin.Y + client.Bottom}, true
}

func miniOverlayBounds(client rect, prefs miniCardPreferences, baseWidth, baseHeight int32) rect {
	scale := math.Min(float64(prefs.Size)/100, math.Min(float64(client.Right-client.Left)/float64(baseWidth), float64(client.Bottom-client.Top)/float64(baseHeight)))
	width, height := int32(float64(baseWidth)*scale), int32(float64(baseHeight)*scale)
	x := client.Left + (client.Right-client.Left-width)*prefs.X/1000
	y := client.Top + (client.Bottom-client.Top-height)*prefs.Y/1000
	return rect{x, y, x + width, y + height}
}

func syncGameOverlay() {
	if gameOverlayDragging || gameOverlaySyncing {
		return
	}
	gameOverlaySyncing = true
	defer func() { gameOverlaySyncing = false }()
	if !gameOverlay.Enabled || !overlayShown {
		miniDPS.hide()
		miniBarrier.hide()
		miniNotice.hide()
		return
	}
	client, available := gameOverlayClient()
	if !available || hideMiniForMissingHUD(time.Now(), client) {
		miniDPS.hide()
		miniBarrier.hide()
		miniNotice.hide()
		return
	}
	dps, elapsed := "—", "00:00.0"
	if battle := displayBattle(); battle != nil {
		dps = formatNum(float64(battle.Damage) / math.Max(1, battle.Duration))
		elapsed = duration(battle.Duration)
	}
	now := time.Now()
	if seconds := soloPreparationRemaining(now); seconds > 0 {
		dps, elapsed = "—", miniStartTime(seconds)
	}
	miniDPS.present(miniOverlayBounds(client, gameOverlay.miniCardPreferences, miniWidth, miniDPSHeight), gameOverlay.miniCardPreferences, miniOverlayContent{title: "DPS", value: dps, elapsed: elapsed})
	countdown, visible := barrierCountdown(now, barrierConfig)
	if visible && countdown != barrierStartedLabel {
		countdown = miniBarrierTime(nextBarrierTime(now).Sub(now))
	}
	if gameOverlayAdjust && !visible {
		countdown, visible = "—", true
	}
	noticeActive := barrierBannerActive && now.Before(barrierBannerEnd(barrierConfig, barrierBannerEvent, barrierBannerAt))
	if noticeActive || gameOverlayAdjust {
		label := "結界バナーの位置"
		if noticeActive {
			label = barrierStartedLabel
			if barrierBannerEvent.Kind == "before" {
				label = "結界まで " + miniBarrierTime(barrierBannerEvent.Target.Sub(now))
			}
		}
		prefs := gameOverlay.Notice
		if pos := gameOverlay.NoticePosition; pos != nil {
			prefs.X, prefs.Y = pos.X, pos.Y
		}
		bounds := miniOverlayBounds(client, prefs, miniNoticeWidth, miniNoticeHeight)
		if gameOverlay.NoticePosition == nil {
			offset := min(int32(24), client.Bottom-bounds.Bottom)
			bounds.Top, bounds.Bottom = bounds.Top+offset, bounds.Bottom+offset
		}
		miniNotice.present(bounds, prefs, miniOverlayContent{value: label, notice: true})
	} else {
		miniNotice.hide()
	}
	if visible {
		miniBarrier.present(miniOverlayBounds(client, gameOverlay.Barrier, miniBarrierWidth, miniBarrierHeight), gameOverlay.Barrier, miniOverlayContent{value: countdown})
	} else {
		miniBarrier.hide()
	}
}

func dragGameOverlay(window uintptr) {
	client, ok := gameOverlayClient()
	if !ok {
		return
	}
	gameOverlayDragging = true
	pReleaseCapture.Call()
	pSendMessage.Call(window, 0x00a1, 2, 0)
	gameOverlayDragging = false
	bounds := overlayScreenBounds(window)
	prefs := &gameOverlay.miniCardPreferences
	switch window {
	case miniNotice.handle:
		prefs = &miniNotice.prefs
		miniNotice.bounds = rect{}
	case miniBarrier.handle:
		prefs = &gameOverlay.Barrier
		miniBarrier.bounds = rect{}
	default:
		miniDPS.bounds = rect{}
	}
	xTravel := client.Right - client.Left - (bounds.Right - bounds.Left)
	yTravel := client.Bottom - client.Top - (bounds.Bottom - bounds.Top)
	if xTravel > 0 {
		prefs.X = min(1000, max(0, ((bounds.Left-client.Left)*1000+xTravel/2)/xTravel))
	}
	if yTravel > 0 {
		prefs.Y = min(1000, max(0, ((bounds.Top-client.Top)*1000+yTravel/2)/yTravel))
	}
	if window == miniNotice.handle {
		gameOverlay.NoticePosition = &point{prefs.X, prefs.Y}
	}
	saveGameOverlay()
	syncGameOverlay()
}
