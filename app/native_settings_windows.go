//go:build native && windows

package main

import "fmt"

type settingsTab int

const (
	settingsMeasure settingsTab = iota
	settingsDisplay
	settingsNotice
	settingsAdvanced
	settingsOther
)

var nativeSettingsTab settingsTab
var nativeDisplayDetails, nativeNoticeDetails bool

func openNativeSettings(tab settingsTab) {
	if tab == settingsMeasure {
		tab = settingsAdvanced
	}
	page := "設定"
	switch tab {
	case settingsDisplay:
		page = "オーバーレイ"
	case settingsNotice:
		page = "結界通知"
	}
	nativePage, nativeSettingsTab, nativeScroll = page, tab, 0
}

func nativeSettingsBottom() int32 {
	const sectionTabsHeight = 52
	switch nativeSettingsTab {
	case settingsDisplay:
		if gameOverlay.Enabled && nativeDisplayDetails {
			return 865 - sectionTabsHeight
		}
		return 570 - sectionTabsHeight
	case settingsNotice:
		if nativeNoticeDetails {
			return 975 - sectionTabsHeight
		}
		return 705 - sectionTabsHeight
	case settingsAdvanced:
		if rawCaptureTools {
			return 935
		}
		return 735
	case settingsOther:
		return 680
	case settingsMeasure:
		return 640
	default:
		return 525
	}
}

func nativeSettingsPanel(dc uintptr, left, right, y int32, mode int) {
	if nativePage == "設定" {
		tabs := [...]struct {
			label, action string
			tab           settingsTab
		}{

			{"詳細", "settings-section-advanced", settingsAdvanced},
			{"その他", "settings-section-other", settingsOther},
		}
		width := min(int32(170), (right-left-24)/2)
		for i, item := range tabs {
			x := left + int32(i)*(width+6)
			nbutton(dc, rect{x, y, x + width, y + 36}, item.label, item.action, nativeSettingsTab == item.tab, int(item.tab))
		}
		y += 52
	}
	switch nativeSettingsTab {
	case settingsDisplay:
		nativeDisplayPanel(dc, left, right, y, mode)
	case settingsNotice:
		nativeBarrierPanel(dc, left, right, y)
	case settingsAdvanced:
		nativeAdvancedPanel(dc, left, right, y)
	case settingsOther:
		nbox(dc, rect{left, y, right, y + 116}, 0x281B12)
		ntxt(dc, "設定の引き継ぎ", rect{left + 18, y + 12, right - 16, y + 40}, 0xE9E3D9, 16)
		nbutton(dc, rect{left + 18, y + 54, left + 290, y + 90}, "旧版から設定を引き継ぐ", "settings-import", false, 0)
		ntxt(dc, "旧版のフォルダーから表示・計測・通知の設定を取り込みます。", rect{left + 304, y + 61, right - 16, y + 88}, 0xAF997D, 12)
		y += 132
		nbox(dc, rect{left, y, right, y + 212}, 0x281B12)
		ntxt(dc, "アプリの更新", rect{left + 18, y + 12, right - 16, y + 40}, 0xE9E3D9, 16)
		nativeToggleButton(dc, rect{left + 18, y + 50, left + 330, y + 86}, "起動時に更新を確認", "update-auto", updatePrefs.CheckOnStartup, 0xA95C7C)
		nbutton(dc, rect{left + 18, y + 98, left + 220, y + 134}, "最新版を確認", "update-check", false, 0)
		if pendingUpdate != nil && !updateBusy {
			nbutton(dc, rect{left + 234, y + 98, left + 438, y + 134}, "更新する v"+pendingUpdate.Version, "update-install", true, 0)
		}
		ntxt(dc, updateStatus, rect{left + 18, y + 145, right - 16, y + 172}, 0xE9E3D9, 13)
		ntxt(dc, "自動確認は1日1回。適用前に確認し、設定・履歴・編集済みCSVを保持します。", rect{left + 18, y + 177, right - 16, y + 204}, 0xAF997D, 12)
	default:
		nativeAdvancedPanel(dc, left, right, y)
	}
}

func nativeDisplayPanel(dc uintptr, left, right, y int32, mode int) {
	height := int32(215)
	if gameOverlay.Enabled {
		height = 248
	}
	nbox(dc, rect{left, y, right, y + height}, 0x281B12)
	ntxt(dc, "オーバーレイの表示と操作", rect{left + 18, y + 12, right - 18, y + 40}, 0xE9E3D9, 16)
	drawNativeChoice(dc, rect{left + 18, y + 49, left + 238, y + 87}, [2]string{"通常表示", "ミニ表示"}, [2]string{"overlay-regular", "overlay-game"}, mode)
	nativeToggleButton(dc, rect{left + 254, y + 49, left + 410, y + 87}, "表示", "overlay", overlayShown, 0xA95C7C)
	nativeToggleButton(dc, rect{left + 426, y + 49, left + 640, y + 87}, "クリック透過", "click", overlayPassesClicks(), 0xA95C7C)
	if gameOverlay.Enabled {
		nativeToggleButton(dc, rect{left + 18, y + 105, left + 314, y + 144}, "位置調整 / 常に表示", "mini-adjust", gameOverlayAdjust, 0xA95C7C)
		ntxt(dc, "オンにすると、ゲーム上のカードをドラッグして動かせます。", rect{left + 329, y + 114, right - 18, y + 142}, 0xC0AC90, 12)
		nativeToggleButton(dc, rect{left + 18, y + 154, left + 453, y + 193}, "ゲームメニュー中はミニ表示を隠す", "mini-hide-without-hud", gameOverlay.HideWithoutHUD, 0xA95C7C)
		nbutton(dc, rect{left + 18, y + 202, left + 268, y + 239}, settingsDisclosure("カードの見た目と位置", nativeDisplayDetails), "settings-display-details", nativeDisplayDetails, 0)
		if !nativeDisplayDetails {
			return
		}
		nativeMiniCardSettings(dc, left, right, y+264, "DPSカード", &gameOverlay.miniCardPreferences, 0)
		nativeMiniCardSettings(dc, left, right, y+374, "結界カウントダウン", &gameOverlay.Barrier, 1)
		nativeMiniCardSettings(dc, left, right, y+484, "結界バナー（事前・開始共通）", &gameOverlay.Notice, 2)
		return
	}
	ntxt(dc, fmt.Sprintf("背景の不透明度  %d%%", int(opacity)*100/255), rect{left + 18, y + 118, right - 18, y + 147}, 0xC0AC90, 13)
	track := rect{left + 18, y + 159, right - 18, y + 176}
	fill(dc, track, 0x503B2B)
	thumb := track.Left + int32(float64(int(opacity)-60)/195*float64(track.Right-track.Left))
	fill(dc, rect{track.Left, track.Top, thumb, track.Bottom}, 0x35ACEC)
	hit := rect{track.Left, max(nativeBodyTop, track.Top-10), track.Right, min(nativeBottom, track.Bottom+10)}
	if hit.Bottom > hit.Top {
		nativeHits = append(nativeHits, nativeHit{hit, "opacity-slider", 0})
	}
}

func settingsDisclosure(label string, expanded bool) string {
	if expanded {
		return "▾  " + label
	}
	return "▸  " + label
}
