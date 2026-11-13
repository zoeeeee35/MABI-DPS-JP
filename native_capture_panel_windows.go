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
func nativeCapturePanel(dc uintptr, left, right, y int32) {
	s := captureCtl.snapshot()
	nbox(dc, rect{left, y, right, y + 296}, 0x281B12)
	ntxt(dc, "データ取得", rect{left + 18, y + 14, right - 16, y + 40}, 0xE9E3D9, 16)
	ntxt(dc, "ゲーム接続のあるアダプターで自動開始。上部バーから手動操作もできます。", rect{left + 18, y + 48, right - 16, y + 72}, 0xAF997D, 12)
	ntxt(dc, stageText(s.Stage)+" / "+s.Detail, rect{left + 18, y + 77, right - 18, y + 104}, 0xAACF8A, 12)
	if captureSetupError != "" {
		ntxt(dc, captureSetupError, rect{left + 18, y + 107, right - 18, y + 137}, 0xB5A1F5, 12)
	}
	nativeToggleButton(dc, rect{left + 18, y + 145, left + 330, y + 180}, "キャプチャの自動開始", "capture-auto", autoCapturePrefs.Enabled, 0xA95C7C)
	ntxt(dc, autoCaptureStatus, rect{left + 18, y + 185, right - 16, y + 208}, 0xAF997D, 12)
	nativeToggleButton(dc, rect{left + 18, y + 214, left + 330, y + 249}, "自分IDの自動推定", "solo-autotoggle", soloAutoEnabled, 0xA95C7C)
	nbutton(dc, rect{left + 18, y + 257, left + 290, y + 288}, fmt.Sprintf("戦闘終了の待機時間  %d秒", idle/1000), "idle-edit", false, 0)
	ntxt(dc, "最後のダメージからの待機時間。計測時間には含めません。", rect{left + 304, y + 264, right - 16, y + 290}, 0xAF997D, 12)
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
}
