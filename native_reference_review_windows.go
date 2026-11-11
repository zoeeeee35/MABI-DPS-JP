//go:build native && windows

package main

import (
	"math"
	"path/filepath"
	"unsafe"
)

// Exercise only local navigation and selection; never start capture or recording.
func nativeReferenceReview(dir string) map[string]any {
	if !nativeModern {
		return map[string]any{"mode": "classic"}
	}
	oldPage, oldScroll, oldSettings := nativePage, nativeScroll, nativeSettingsTab
	oldHistory, oldDrawer := historyIndex, nativeHistoryOpen
	oldAdaptersOpen := nativeAdaptersOpen
	oldTarget, oldTargetContext := selectedDamageTarget, selectedDamageTargetContext
	defer func() { selectedDamageTarget, selectedDamageTargetContext = oldTarget, oldTargetContext }()
	oldExpanded := unknownEffectsExpanded
	defer func() { unknownEffectsExpanded = oldExpanded }()
	oldSort, oldDirection := skillSortColumn, skillSortDescending
	oldMinutes, oldTimer, oldMeasure := soloTimerMinutes, soloTimerEnd, soloMeasurement
	oldPlaying := false
	if session != nil {
		oldPlaying = session.Playing
	}
	selection := send(controls[idCaptureAdapters], 0x0147, 0, 0)
	defer func() {
		nativePage, nativeScroll, nativeSettingsTab = oldPage, oldScroll, oldSettings
		historyIndex, nativeHistoryOpen = oldHistory, oldDrawer
		nativeAdaptersOpen = oldAdaptersOpen
		skillSortColumn, skillSortDescending = oldSort, oldDirection
		soloTimerMinutes, soloTimerEnd, soloMeasurement = oldMinutes, oldTimer, oldMeasure
		if session != nil {
			session.Playing = oldPlaying
		}
		send(controls[idCaptureAdapters], 0x014e, selection, 0)
	}()
	click := func(action string) bool {
		for _, h := range nativeHits {
			if h.Action == action {
				nativeModernClick((h.R.Left+h.R.Right)/2, (h.R.Top+h.R.Bottom)/2)
				return true
			}
		}
		return false
	}
	nativePage = "分析"
	nativeScroll = 0
	_ = captureOwnSurface(filepath.Join(dir, "player-main.png"), mainWindow, true)
	scroll := nativeMaxScroll > 0
	settings := click("settings") && nativePage == "設定"
	_ = captureOwnSurface(filepath.Join(dir, "capture-settings.png"), mainWindow, true)
	details := click("settings-section-advanced") && nativeSettingsTab == settingsAdvanced
	// Scroll to the diagnostic actions.
	nativeScroll = 1000000 // The next paint computes and clamps the new maximum.
	_ = captureOwnSurface(filepath.Join(dir, "advanced-settings.png"), mainWindow, true)
	flagReview := click("flag-review") && nativePage == "判定詳細"
	_ = captureOwnSurface(filepath.Join(dir, "flags-details.png"), mainWindow, true)
	hasAction := func(action string) bool {
		for _, h := range nativeHits {
			if h.Action == action {
				return true
			}
		}
		return false
	}
	overlayPage := click("page:オーバーレイ") && nativePage == "オーバーレイ"
	_ = captureOwnSurface(filepath.Join(dir, "overlay-settings.png"), mainWindow, true)
	overlayPage = overlayPage && !hasAction("settings-section")
	noticePage := click("page:結界通知") && nativePage == "結界通知"
	_ = captureOwnSurface(filepath.Join(dir, "notice-settings.png"), mainWindow, true)
	noticePage = noticePage && !hasAction("settings-section")
	settingsOtherPage := click("settings")
	_ = captureOwnSurface(filepath.Join(dir, "capture-settings.png"), mainWindow, true)
	settingsOtherPage = settingsOtherPage && !hasAction("settings-import") && click("settings-section-other")
	_ = captureOwnSurface(filepath.Join(dir, "other-settings.png"), mainWindow, true)
	settingsOtherPage = settingsOtherPage && nativeSettingsTab == settingsOther && hasAction("settings-import")
	nativePage = "分析"
	nativeScroll = 0
	nativeAdaptersOpen = false
	_ = captureOwnSurface(filepath.Join(dir, "adapter-base.png"), mainWindow, true)
	dropdownOpened := click("adapters") && nativeAdaptersOpen
	_ = captureOwnSurface(filepath.Join(dir, "adapter-selection.png"), mainWindow, true)
	selected := false
	if len(captureAdapterList) > 0 {
		selected = click("adapter-select") && send(controls[idCaptureAdapters], 0x0147, 0, 0) == 0 && !nativeAdaptersOpen
	}
	nativeAdaptersOpen = false
	nativePage = "分析"
	nativeScroll = 0
	nativeHistoryOpen = false
	_ = captureOwnSurface(filepath.Join(dir, "drawer-base.png"), mainWindow, true)
	drawerToggle := click("history-toggle") && nativeHistoryOpen
	_ = captureOwnSurface(filepath.Join(dir, "history-drawer.png"), mainWindow, true)
	drawerSelection := false
	if len(history) > 0 {
		drawerSelection = click("history") && !nativeHistoryOpen && historyIndex >= 0 && nativePage == "分析"
	}
	_ = captureOwnSurface(filepath.Join(dir, "history-selected.png"), mainWindow, true)
	historyIndex = oldHistory
	nativeHistoryOpen = false
	nativePage = "履歴"
	nativeScroll = 0
	_ = captureOwnSurface(filepath.Join(dir, "history-list.png"), mainWindow, true)
	if len(history) > 0 {
		openHistoryLocation(0)
		_ = captureOwnSurface(filepath.Join(dir, "history-location.png"), historyNoteWindow, false)
		pDestroy.Call(historyNoteWindow)
	}
	oldShown, oldClick := overlayShown, clickThrough
	showToggle := click("overlay") && overlayShown != oldShown
	showToggle = click("overlay") && overlayShown == oldShown && showToggle
	passToggle := click("click") && clickThrough != oldClick
	passToggle = click("click") && clickThrough == oldClick && passToggle
	var overlayRect rect
	pGetClient.Call(overlayWindow, uintptr(unsafe.Pointer(&overlayRect)))
	timerSelectionOnly := true
	for minute := 1; minute <= 3; minute++ {
		x := int32(120+(minute-1)*46) * overlayRect.Right / 360
		y := int32(350-overlayResourceGap) * overlayRect.Bottom / fantasyOverlayHeight
		soloOverlayClick(overlayWindow, x, y)
		timerSelectionOnly = timerSelectionOnly && soloTimerMinutes == minute && soloTimerEnd.IsZero() && soloMeasurement == nil
	}
	soloTimerMinutes, soloTimerEnd, soloMeasurement = oldMinutes, oldTimer, oldMeasure
	nativePage = "スキル"
	nativeScroll = 0
	nativeHistoryOpen = false
	nativeAdaptersOpen = false
	skillSortColumn = 3
	skillSortDescending = true
	_ = captureOwnSurface(filepath.Join(dir, "skills-sort-base.png"), mainWindow, true)
	sortClicks := true
	for _, column := range []int{9, 9, 5, 5} {
		found := false
		for _, h := range nativeHits {
			if h.Action == "skill-sort" && h.Index == column {
				nativeModernClick((h.R.Left+h.R.Right)/2, (h.R.Top+h.R.Bottom)/2)
				found = true
				break
			}
		}
		sortClicks = sortClicks && found && skillSortColumn == column
		_ = captureOwnSurface(filepath.Join(dir, "skills-sort.png"), mainWindow, true)
	}
	sortClicks = sortClicks && !skillSortDescending
	unknownToggle := false
	unknownEffectsExpanded = false
	_ = captureOwnSurface(filepath.Join(dir, "unknown-collapsed.png"), mainWindow, true)
	if click("unknown-effects") {
		unknownToggle = unknownEffectsExpanded
		_ = captureOwnSurface(filepath.Join(dir, "unknown-expanded.png"), mainWindow, true)
		unknownToggle = click("unknown-effects") && !unknownEffectsExpanded && unknownToggle
	}
	targetSelected, targetCleared, targetDropdown := false, false, false
	targetBefore, targetAfter := int64(0), int64(0)
	nativePage = "スキル"
	nativeScroll = 0
	selectedDamageTarget = ""
	nativeSkillTargetsOpen = false
	_ = captureOwnSurface(filepath.Join(dir, "target-field.png"), mainWindow, true)
	targetCount := len(nativeTargetRows)
	if b := displayBattle(); b != nil {
		targetBefore = b.Damage
	}
	if click("skill-target-toggle") {
		targetDropdown = nativeSkillTargetsOpen
		_ = captureOwnSurface(filepath.Join(dir, "target-list.png"), mainWindow, true)
		if click("target-select") {
			if b := displayBattle(); b != nil {
				targetAfter = b.Damage
				targetSelected = currentTargetFilter() != ""
				for _, h := range b.DamageEvents {
					targetSelected = targetSelected && h.TargetID == currentTargetFilter()
				}
			}
			_ = captureOwnSurface(filepath.Join(dir, "target-skills.png"), mainWindow, true)
			_ = captureOwnSurface(filepath.Join(dir, "target-overlay.png"), overlayWindow, false)
		}
	}
	pieHover := false
	pieHoverName := ""
	nativeSkillTargetsOpen = false
	nativePage = "分析"
	nativeScroll = 0
	nativePieMouseInside = false
	_ = captureOwnSurface(filepath.Join(dir, "skill-pie.png"), mainWindow, true)
	if len(nativePieSlices) > 0 {
		pick := 0
		for i, s := range nativePieSlices {
			if len(s.Name) > len(nativePieSlices[pick].Name) && s.Share >= 0.005 {
				pick = i
			}
		}
		s := nativePieSlices[pick]
		a := (s.Start+s.End)*math.Pi - math.Pi/2
		x, y := int32(nativePieCX+nativePieRadius*0.7*math.Cos(a)), int32(nativePieCY+nativePieRadius*0.7*math.Sin(a))
		nativePieMove(mainWindow, x, y)
		pieHover = nativePieHoverKey == s.Key
		pieHoverName = s.Name
		_ = captureOwnSurface(filepath.Join(dir, "skill-pie-tooltip.png"), mainWindow, true)
		nativePieMouseInside = false
		nativePieHoverKey = ""
		_ = captureOwnSurface(filepath.Join(dir, "skill-pie-no-tooltip.png"), mainWindow, true)
	}
	nativePage = "スキル"
	nativeScroll = 0
	_ = captureOwnSurface(filepath.Join(dir, "target-before-clear.png"), mainWindow, true)
	if click("skill-target-toggle") {
		_ = captureOwnSurface(filepath.Join(dir, "target-clear-dropdown.png"), mainWindow, true)
		if click("target-all") {
			if b := displayBattle(); b != nil {
				targetCleared = currentTargetFilter() == "" && b.Damage == targetBefore
			}
		}
	}
	nativeSkillTargetsOpen = false
	// The same selector is embedded in Analysis after the charts and ranking.
	nativePage = "分析"
	nativeScroll = 900
	_ = captureOwnSurface(filepath.Join(dir, "analysis-target-field.png"), mainWindow, true)
	analysisOpened, analysisSelected, analysisCleared := false, false, false
	if click("skill-target-toggle") {
		_ = captureOwnSurface(filepath.Join(dir, "analysis-target-dropdown.png"), mainWindow, true)
		for _, h := range nativeHits {
			if h.Action == "target-all" {
				analysisOpened = true
			}
		}
		if click("target-select") {
			analysisSelected = currentTargetFilter() != ""
			if view := displayBattle(); view != nil {
				for _, hit := range view.DamageEvents {
					if hit.TargetID != currentTargetFilter() {
						analysisSelected = false
					}
				}
			} else {
				analysisSelected = false
			}
			_ = captureOwnSurface(filepath.Join(dir, "analysis-target-selected.png"), mainWindow, true)
			if click("skill-target-toggle") {
				_ = captureOwnSurface(filepath.Join(dir, "analysis-target-clear-list.png"), mainWindow, true)
				if click("target-all") {
					analysisCleared = currentTargetFilter() == ""
					if view := displayBattle(); view != nil {
						analysisCleared = analysisCleared && view.Damage == targetBefore
					}
				}
			}
		}
	}
	nativeSkillTargetsOpen = false
	return map[string]any{"barrierUI": barrierUIReview(dir), "analysisTargetDropdown": analysisOpened, "analysisTargetSelected": analysisSelected, "analysisTargetCleared": analysisCleared, "targetDropdown": targetDropdown, "skillPieHover": pieHover, "skillPieHoverName": pieHoverName, "targetSelected": targetSelected, "targetCleared": targetCleared, "targetCount": targetCount, "targetBeforeDamage": targetBefore, "targetAfterDamage": targetAfter, "unknownEffectToggle": unknownToggle, "effectCSV": effectNamesStatus, "translationCSV": skillTranslationCSVStatus, "translationExample": japaneseSkillName("Arbalist_GustingBolt_01"), "newTranslationExample": japaneseSkillName("HighArcher_ArrowRevolver"), "skillHeaderSortToggle": sortClicks, "overlayShowToggle": showToggle, "clickThroughToggle": passToggle, "adapterDropdownOpen": dropdownOpened, "timerSelectionDoesNotStart": timerSelectionOnly, "mainFontFace": nativeFontFace(), "bufferedPaintFrames": nativeBufferedFrames, "historyDrawerToggle": drawerToggle, "historyDrawerSelection": drawerSelection, "titleBar": nativeTitleBarState, "flagDetailsNavigation": flagReview, "integratedCaptureSettings": captureEmbedded, "legacyCaptureWindowVisible": nativeCaptureVisible(), "adapterCount": len(captureAdapterList), "scrollAvailable": scroll, "settingsNavigation": settings, "overlayPage": overlayPage, "noticePage": noticePage, "settingsOtherPage": settingsOtherPage, "detailsToggle": details, "adapterSelection": selected}
}
func nativeCaptureVisible() bool {
	v, _, _ := user32.NewProc("IsWindowVisible").Call(captureWindow)
	return v != 0
}
