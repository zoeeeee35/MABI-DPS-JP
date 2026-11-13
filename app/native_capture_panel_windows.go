//go:build native && windows

package main

import (
	"fmt"
	"time"
)

func nativeAdapterLabel() string {
	i := int(send(controls[idCaptureAdapters], 0x0147, 0, 0))
	if i >= 0 && i < len(captureAdapterList) {
		return captureAdapterList[i].Label
	}
	return "ネットワークアダプターを選択してください"
}

var nativeAnalysisSettings bool

func nativeCapturePanel(dc uintptr, left, right, y int32) {
	nbox(dc, rect{left, y, right, y + 166}, 0x281B12)
	ntxt(dc, "戦闘の計測", rect{left + 18, y + 14, right - 16, y + 40}, 0xE9E3D9, 16)
	nativeToggleButton(dc, rect{left + 18, y + 58, left + 330, y + 96}, "自分IDの自動推定", "solo-autotoggle", soloAutoEnabled, 0xA95C7C)
	nbutton(dc, rect{left + 18, y + 112, left + 290, y + 150}, fmt.Sprintf("戦闘終了の待機時間  %d秒", idle/1000), "idle-edit", false, 0)
	ntxt(dc, "最後のダメージからの待機時間。計測時間には含めません。", rect{left + 304, y + 121, right - 16, y + 150}, 0xAF997D, 12)
}

func openNativeCapturePage() {
	nativePage, nativeScroll = "分析", 0
	nativeAnalysisSettings = true
	nativeAdaptersOpen = false
}

func nativeCapturePage(dc uintptr, left, right, y int32) {
	ntxt(dc, "分析 / 取得設定", rect{left + 14, y + 8, right - 180, y + 40}, 0xE9E3D9, 17)
	nbutton(dc, rect{right - 160, y + 4, right - 12, y + 42}, "分析に戻る", "page:分析", false, 0)
	y += 58
	nbox(dc, rect{left, y, right, y + 315}, 0x281B12)
	ntxt(dc, "ネットワークアダプター", rect{left + 18, y + 14, right - 16, y + 42}, 0xE9E3D9, 15)
	nativeAdapterField(dc, rect{left + 18, y + 52, right - 18, y + 90})
	active := captureCtl.running()
	label := "キャプチャ開始"
	if active {
		label = "キャプチャ停止"
	}
	nbutton(dc, rect{left + 18, y + 108, left + 260, y + 146}, label, "capture", active, 0)
	nativeNetworkMeter(dc, rect{left + 276, y + 108, left + 461, y + 146})
	s := captureCtl.snapshot()
	ntxt(dc, stageText(s.Stage)+" / "+s.Detail, rect{left + 18, y + 160, right - 18, y + 194}, 0xAACF8A, 13)
	if captureSetupError != "" {
		ntxt(dc, captureSetupError, rect{left + 18, y + 194, right - 18, y + 228}, 0xB5A1F5, 12)
	}
	nativeToggleButton(dc, rect{left + 18, y + 235, left + 330, y + 273}, "キャプチャの自動開始", "capture-auto", autoCapturePrefs.Enabled, 0xA95C7C)
	ntxt(dc, autoCaptureStatus, rect{left + 18, y + 279, right - 18, y + 308}, 0xAF997D, 12)
	nativeCapturePanel(dc, left, right, y+331)
}
func nativeAdvancedPanel(dc uintptr, left, right, y int32) {
	s := captureCtl.snapshot()
	if rawCaptureTools {
		nbox(dc, rect{left, y, right, y + 184}, 0x281B12)
		ntxt(dc, "敵味方判定用のローカル記録", rect{left + 18, y + 14, right - 16, y + 40}, 0xE9E3D9, 15)
		ntxt(dc, "停止するまで保存 / 64MiB上限。ログイン・アカウント操作中は記録しないでください。", rect{left + 18, y + 43, right - 18, y + 67}, 0xAEBDE4, 12)
		nbutton(dc, rect{left + 18, y + 78, left + 238, y + 113}, "記録を開始", "record-start", s.Recording.Active, 0)
		nbutton(dc, rect{left + 248, y + 78, left + 448, y + 113}, "停止して保存", "record-stop", false, 0)
		nbutton(dc, rect{left + 458, y + 78, left + 658, y + 113}, "PCAPNGを開く", "replay", false, 0)
		status := "停止"
		if s.Recording.Active {
			status = fmt.Sprintf("記録中 %d秒 / %d件 / %.2f MiB", int(time.Since(s.Recording.Start).Seconds()), s.Recording.Packets, float64(s.Recording.Bytes)/(1<<20))
		}
		ntxt(dc, status, rect{left + 18, y + 123, right - 18, y + 147}, 0xADCBA6, 12)
		ntxt(dc, s.Recording.Path, rect{left + 18, y + 149, right - 18, y + 174}, 0xA18D77, 11)
		y += 201
	}
	nbox(dc, rect{left, y, right, y + 90}, 0x281B12)
	ntxt(dc, appWindowTitle(), rect{left + 18, y + 10, right - 18, y + 35}, 0xE9E3D9, 15)
	identifierNamesOnce.Do(loadIdentifierNames)
	revision := identifierNamesStatus
	ntxt(dc, revision+" / ビルド日時（UTC）: "+buildUTC, rect{left + 18, y + 43, right - 18, y + 76}, 0xADCBA6, 12)
	y += 106
	nbox(dc, rect{left, y, right, y + 225}, 0x281B12)
	ntxt(dc, "診断とログ", rect{left + 18, y + 10, right - 18, y + 36}, 0xE9E3D9, 15)
	ntxt(dc, trafficStatus(s, ""), rect{left + 18, y + 40, right - 18, y + 67}, 0xC4AC91, 12)
	ntxt(dc, fmt.Sprintf("JP9受信 %d / 0x5243候補 %d / 集計 %d / 欠落 %d", s.JP.InFrames, s.JP.Count, s.Aggregated, s.ReassemblyGaps), rect{left + 18, y + 67, right - 18, y + 93}, 0xBDA88E, 12)
	nbutton(dc, rect{left + 18, y + 95, left + 218, y + 133}, "診断JSONを保存", "diagnostic", false, 0)
	nbutton(dc, rect{left + 230, y + 95, left + 450, y + 133}, "旧詳細設定画面を開く", "legacy-settings", false, 0)
	nbutton(dc, rect{left + 462, y + 95, left + 662, y + 133}, "動作ログを開く", "app-logs", false, 0)
	flagReviewX := left + 18
	if rawCaptureTools {
		flagReviewX = left + 310
		nbutton(dc, rect{left + 18, y + 148, left + 298, y + 186}, "この記録フォルダーを削除", "record-delete", false, 0)
	}
	nbutton(dc, rect{flagReviewX, y + 148, flagReviewX + 280, y + 186}, "判定フラグの検証・CSV", "flag-review", false, 0)
	nbutton(dc, rect{left + 18, y + 192, left + 298, y + 224}, "自分IDを修正", "self-correction", false, 0)
	y += 245
	nbox(dc, rect{left, y, right, y + 110}, 0x281B12)
	ntxt(dc, "ゲーム操作", rect{left + 18, y + 10, right - 18, y + 37}, 0xE9E3D9, 15)
	nativeToggleButton(dc, rect{left + 18, y + 42, left + 390, y + 80}, "ゲーム内にカーソルを固定", "cursor-lock", gameOverlay.CursorLock, 0xA95C7C)
	ntxt(dc, "ゲームが前面の間のみ。Alt押下中・他の画面への切り替えで解除します。", rect{left + 18, y + 84, right - 18, y + 108}, 0xAF997D, 12)
}
