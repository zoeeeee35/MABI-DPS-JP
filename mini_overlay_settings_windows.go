//go:build native && windows

package main

import (
	"fmt"
	"strconv"
	"syscall"
	"unsafe"
)

func nativeMiniCardSettings(dc uintptr, left, right, y int32, title string, prefs *miniCardPreferences, index int) {
	nbox(dc, rect{left, y, right, y + 99}, 0x281C15)
	ntxt(dc, title, rect{left + 12, y + 8, right - 140, y + 35}, 0xEEEAE3, 15)
	nbutton(dc, rect{right - 133, y + 5, right - 9, y + 37}, "位置を戻す", "mini-reset-position", false, index)
	colorLabel := "数字色 "
	if index == 2 {
		colorLabel = "文字色 "
	}
	buttons := []struct{ label, action string }{
		{fmt.Sprintf("サイズ %d%%", prefs.Size), "mini-size"},
		{"文字 " + miniFontLabels[prefs.Font], "mini-font"},
		{fmt.Sprintf("背景の不透明度 %d%%", miniOpacities[prefs.BackgroundOpacity]), "mini-opacity"},
		{colorLabel + miniTextColorLabels[prefs.TextColor], "mini-text-color"},
		{"背景色 " + miniBackgroundLabels[prefs.BackgroundColor], "mini-background"},
	}
	available := right - left - 24
	for i, button := range buttons {
		x := left + 12 + available*int32(i)/5
		end := left + 12 + available*int32(i+1)/5 - 6
		nbutton(dc, rect{x, y + 49, end, y + 87}, button.label, button.action, false, index)
	}
}

func updateMiniCardSetting(action string, index int) {
	if action == "mini-reset-position" && index == 2 {
		gameOverlay.NoticePosition = nil
		saveGameOverlay()
		syncGameOverlay()
		return
	}
	prefs := &gameOverlay.miniCardPreferences
	defaultY := int32(580)
	if index == 1 {
		prefs, defaultY = &gameOverlay.Barrier, 460
	} else if index == 2 {
		prefs = &gameOverlay.Notice
	}
	switch action {
	case "mini-size":
		openMiniSizeEditor(prefs)
		return
	case "mini-font":
		prefs.Font = (prefs.Font + 1) % len(miniFonts)
	case "mini-opacity":
		prefs.BackgroundOpacity = (prefs.BackgroundOpacity + 1) % len(miniOpacities)
	case "mini-text-color":
		prefs.TextColor = (prefs.TextColor + 1) % len(miniTextColors)
	case "mini-background":
		prefs.BackgroundColor = (prefs.BackgroundColor + 1) % len(miniBackgroundColors)
	case "mini-reset-position":
		prefs.X, prefs.Y = 940, defaultY
	}
	saveGameOverlay()
	syncGameOverlay()
}

var miniSizeWindow, miniSizeEdit uintptr
var numericApply func(int)
var numericMin, numericMax int
var miniSizeCallback = syscall.NewCallback(miniSizeProc)

func openMiniSizeEditor(prefs *miniCardPreferences) {
	if prefs != nil {
		openNumericSetting("ミニ表示のサイズ", "サイズ（50〜250％）", prefs.Size, miniSizeMin, miniSizeMax, func(value int) {
			prefs.Size = value
			saveGameOverlay()
			syncGameOverlay()
			updateDisplay()
		})
		return
	}
	openNumericSetting("戦闘終了までの待機時間", "無ダメージの待機時間（1〜60秒）", int(idle/1000), 1, 60, func(value int) {
		gameOverlay.IdleSeconds = int64(value)
		idle = int64(value) * 1000
		if session != nil {
			session.Idle = idle
			_ = session.recompute()
		}
		saveGameOverlay()
		syncGameOverlay()
		updateDisplay()
	})
}

func openNumericSetting(title, prompt string, value, minimum, maximum int, apply func(int)) {
	if miniSizeWindow != 0 {
		pDestroy.Call(miniSizeWindow)
	}
	_ = registerClass("MabiDPSMiniSize", miniSizeCallback, 6)
	bounds := overlayScreenBounds(mainWindow)
	miniSizeWindow = createWindow("MabiDPSMiniSize", title, 0, 0x00ca0000, bounds.Left+45, bounds.Top+80, 340, 168, mainWindow, 0)
	if miniSizeWindow == 0 {
		return
	}
	numericApply, numericMin, numericMax = apply, minimum, maximum
	label := createWindow("STATIC", prompt, 0, wsChild|wsVisible, 16, 14, 280, 24, miniSizeWindow, 0)
	miniSizeEdit = createWindow("EDIT", strconv.Itoa(value), 0x200, wsChild|wsVisible|wsTabStop|0x2000|0x80, 16, 42, 120, 30, miniSizeWindow, 101)
	send(miniSizeEdit, 0xc5, 3, 0)
	button := createWindow("BUTTON", "適用", 0, wsChild|wsVisible|wsTabStop, 150, 42, 76, 30, miniSizeWindow, 102)
	cancel := createWindow("BUTTON", "閉じる", 0, wsChild|wsVisible|wsTabStop, 232, 42, 76, 30, miniSizeWindow, 103)
	for _, control := range []uintptr{label, miniSizeEdit, button, cancel} {
		send(control, 0x30, nf(14), 1)
	}
	show(miniSizeWindow, true)
	user32.NewProc("SetFocus").Call(miniSizeEdit)
	send(miniSizeEdit, 0xb1, 0, ^uintptr(0))
}

func miniSizeProc(window uintptr, msg uint32, w, l uintptr) uintptr {
	switch msg {
	case wmCommand:
		switch w & 0xffff {
		case 102:
			var buf [8]uint16
			send(miniSizeEdit, 0xd, uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
			value, err := strconv.Atoi(syscall.UTF16ToString(buf[:]))
			if err != nil || value < numericMin || value > numericMax {
				alert(window, fmt.Sprintf("%d〜%dの整数を入力してください。", numericMin, numericMax))
				return 0
			}
			apply := numericApply
			pDestroy.Call(window)
			if apply != nil {
				apply(value)
			}
		case 103:
			pDestroy.Call(window)
		}
		return 0
	case wmDestroy:
		miniSizeWindow, miniSizeEdit, numericApply = 0, 0, nil
		return 0
	}
	result, _, _ := pDefWindow.Call(window, uintptr(msg), w, l)
	return result
}
