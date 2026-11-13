//go:build native && windows

package main

import (
	"fmt"
	"path/filepath"
)

// Render the actual application surface during GUI self-test and check the
// controls, so a missing page dispatch cannot pass as a successful build.
func reviewCaptureSettingsSurface(dir string) string {
	oldModern, oldPage, oldChild, oldScroll, oldTab := nativeModern, nativePage, nativeAnalysisSettings, nativeScroll, nativeSettingsTab
	defer func() {
		nativeModern, nativePage, nativeAnalysisSettings, nativeScroll, nativeSettingsTab = oldModern, oldPage, oldChild, oldScroll, oldTab
	}()
	nativeModern = true
	pSetWindowPos.Call(mainWindow, 0, 0, 0, 1180, 970, 0x0016)
	openNativeCapturePage()
	if nativePage != "分析" || !nativeAnalysisSettings {
		return "取得設定が分析の子画面ではありません"
	}
	if err := captureOwnSurface(filepath.Join(dir, "analysis-capture-settings.png"), mainWindow, true); err != nil {
		return err.Error()
	}
	seen := map[string]bool{}
	for _, hit := range nativeHits {
		seen[hit.Action] = true
	}
	for _, action := range []string{"capture", "capture-auto", "solo-autotoggle", "idle-edit", "page:分析"} {
		if !seen[action] {
			return fmt.Sprintf("取得設定に%sが描画されていません", action)
		}
	}
	if seen["cursor-lock"] || seen["settings-section"] {
		return "取得設定に設定タブが混在しています"
	}
	// Adapter/capture controls must exist once in the child page, and never
	// remain duplicated in the main header or ordinary analysis page.
	counts := map[string]int{}
	for _, hit := range nativeHits {
		counts[hit.Action]++
	}
	if counts["adapters"] != 1 || counts["capture"] != 1 {
		return "取得設定のキャプチャ操作が重複しています"
	}
	nativeAnalysisSettings = false
	nativePage, nativeScroll = "分析", 0
	if err := captureOwnSurface(filepath.Join(dir, "analysis-main.png"), mainWindow, true); err != nil {
		return err.Error()
	}
	for _, hit := range nativeHits {
		if hit.Action == "adapters" || hit.Action == "capture" {
			return "分析上段に取得設定が残っています"
		}
	}
	openNativeSettings(settingsAdvanced)
	if err := captureOwnSurface(filepath.Join(dir, "advanced-cursor-setting.png"), mainWindow, true); err != nil {
		return err.Error()
	}
	seen = map[string]bool{}
	for _, hit := range nativeHits {
		seen[hit.Action] = true
	}
	if !seen["cursor-lock"] || seen["settings-section"] {
		return "詳細設定の項目配置が不正です"
	}
	return ""
}
