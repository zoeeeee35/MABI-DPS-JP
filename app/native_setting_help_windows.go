//go:build native && windows

package main

var nativeSettingHover string

var nativeSettingHelp = map[string]string{
	"mini-adjust":              "設定画面を前面にしてもミニ表示を残します。カードを個別にドラッグして位置を調整できます。",
	"settings-display-details": "DPS・結界カードのサイズ、文字、背景色などを調整します。",
	"settings-notice-details":  "通知音の選択、音量調整、バナーやWindows通知のテストを開きます。",
	"barrier-keep-before":      "オンにすると事前バナーを結界の予定開始時刻まで表示します。オフなら8秒で消えます。",
	"barrier-banner-seconds":   "開始時刻に出るバナーの表示時間を1〜30秒で設定します。",
	"mini-hide-without-hud":    "ゲーム左上のキャラクター枠とHPバーが見えなくなると、ミニ表示を隠します。バッグやメニューを閉じると再表示します。計測は続きます。",
	"click":                    "オンにすると、オーバーレイ上のクリックが背後のゲームに届きます。ゲーム内ミニ表示でカードを動かすときは位置調整を使います。",
	"overlay":                  "オーバーレイを表示・非表示にします。表示を消しても計測は続きます。",
	"overlay-regular":          "DPSやダメージの詳細を独立したオーバーレイに表示します。",
	"overlay-game":             "ゲームのウィンドウ内に、DPSと結界の小さなカードを表示します。",
	"scope-current":            "いまの戦闘、または直近に終了した戦闘の結果を表示します。",
	"scope-all":                "キャプチャ開始からの戦闘をまとめて表示します。",
	"idle-edit":                "最後にダメージを取得してから、戦闘終了とみなすまでの秒数です。待機時間はDPSの計測時間に含めません。",
	"mini-size":                "このカードの大きさを50〜250％で設定します。",
	"mini-font":                "このカードに使う文字の書体を切り替えます。",
	"mini-opacity":             "カード背景の不透明度を切り替えます。数値が低いほど背後のゲーム画面が見えます。",
	"mini-text-color":          "このカードの数値に使う色を切り替えます。",
	"mini-background":          "このカードの背景色を切り替えます。",
	"mini-reset-position":      "このカードの位置を初期位置に戻します。",
	"barrier-master":           "召喚の結界の予定時刻に合わせた通知を切り替えます。",
	"barrier-lead":             "結界開始の何分前に通知するかを設定します。",
	"barrier-windows":          "結界の予定をWindowsの通知として表示します。",
	"barrier-start":            "予定時刻の事前通知に加え、開始時刻にも通知します。",
	"barrier-overlay":          "結界の通知バナーやカウントダウンをオーバーレイに表示します。",
	"barrier-always":           "オンなら次の開始までの残り時間を常時表示します。オフなら事前通知の時刻から表示します。",
	"barrier-sound":            "結界の通知時に効果音を鳴らします。",
}

func nativeSettingHelpAt(x, y int32) string {
	if nativeAdaptersOpen || nativeHistoryOpen || nativeSkillTargetsOpen {
		return ""
	}
	for i := len(nativeHits) - 1; i >= 0; i-- {
		h := nativeHits[i]
		if x >= h.R.Left && x < h.R.Right && y >= h.R.Top && y < h.R.Bottom {
			return nativeSettingHelp[h.Action]
		}
	}
	return ""
}

func nativeSettingTooltip(dc uintptr, width, height int32) {
	if nativeSettingHover == "" || nativeSettingHover != nativeSettingHelpAt(nativePieMouseX, nativePieMouseY) {
		return
	}
	popupWidth := min(int32(410), width-28)
	old, _, _ := pSelect.Call(dc, nf(14))
	lines := nativeWrapTooltip(dc, nativeSettingHover, popupWidth-28)
	pSelect.Call(dc, old)
	popupHeight := 20 + int32(len(lines))*22
	x := min(nativePieMouseX+16, width-popupWidth-12)
	y := min(nativePieMouseY+20, height-popupHeight-12)
	if y <= nativePieMouseY+6 {
		y = max(12, nativePieMouseY-popupHeight-10)
	}
	nbox(dc, rect{x + 3, y + 3, x + popupWidth + 3, y + popupHeight + 3}, 0x100C09)
	nbox(dc, rect{x, y, x + popupWidth, y + popupHeight}, 0x493C2B)
	for i, line := range lines {
		top := y + 10 + int32(i)*22
		ntxt(dc, line, rect{x + 14, top, x + popupWidth - 14, top + 22}, 0xFFFFFF, 14)
	}
}
