//go:build modernpreview && native && windows

package main

import (
	"encoding/json"
	"fmt"
	webview "github.com/jchv/go-webview2"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

var modernUI bool
var modernTestPath string
var modernHandle uintptr
var modernAdapter int
var modernDevices []CaptureDevice
var modernView webview.WebView
var modernOverlayMode = "detail"
var modernError string

type modernPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type modernHistory struct {
	Index    int     `json:"index"`
	Created  string  `json:"created"`
	Label    string  `json:"label"`
	Number   int     `json:"number"`
	Damage   int64   `json:"damage"`
	Duration float64 `json:"duration"`
}

func modernRefreshAdapters() {
	list, e := captureDevices()
	if e != nil {
		modernError = e.Error()
		modernDevices = nil
		return
	}
	modernDevices = list
	for i, v := range list {
		if v.Suggested {
			modernAdapter = i
			break
		}
	}
}
func modernGraph(b *Battle) []modernPoint {
	points := []modernPoint{}
	if b == nil {
		return points
	}
	var sum int64
	step := max(1, len(b.DamageEvents)/300)
	for i, e := range b.DamageEvents {
		sum += e.Damage
		if i%step == 0 || i == len(b.DamageEvents)-1 {
			sec := float64(e.At-b.Start) / 1000
			points = append(points, modernPoint{sec, float64(sum) / math.Max(1, sec)})
		}
	}
	if len(points) > 0 {
		points = append(points, modernPoint{b.Duration, float64(b.Damage) / math.Max(1, b.Duration)})
	}
	return points
}
func modernBattle(b *Battle) any {
	if b == nil {
		return nil
	}
	data, _ := json.Marshal(b)
	m := map[string]any{}
	_ = json.Unmarshal(data, &m)
	players := m["players"].([]any)
	for i, v := range players {
		p := v.(map[string]any)
		p["classLabel"] = mapped("class", b.Players[i].ClassID)
		for j, s := range p["skills"].([]any) {
			s.(map[string]any)["label"] = mapped("skill", b.Players[i].Skills[j].ID)
		}
	}
	return m
}
func modernSnapshot() map[string]any {
	b := displayBattle()
	stats := captureCtl.snapshot()
	adapters := []map[string]any{}
	for _, d := range modernDevices {
		adapters = append(adapters, map[string]any{"label": d.Label})
	}
	label := "アダプター未選択"
	if modernAdapter >= 0 && modernAdapter < len(modernDevices) {
		label = modernDevices[modernAdapter].Label
	}
	hs := []modernHistory{}
	for i, v := range history {
		created, _ := time.Parse(time.RFC3339, v.Archive.CreatedAt)
		source := "実通信"
		if v.Archive.Request.Log.Source == "synthetic" {
			source = "合成サンプル"
		}
		hs = append(hs, modernHistory{i, created.In(time.FixedZone("JST", 9*3600)).Format("2006/01/02 15:04:05"), source, v.Battle.Number, v.Battle.Damage, v.Battle.Duration})
	}
	source := ""
	if session != nil {
		source = session.Log.Source
	}
	if historyIndex >= 0 && historyIndex < len(history) {
		source = history[historyIndex].Archive.Request.Log.Source
	}
	roster := []Player{}
	if captureInput != nil && captureInput.Session != nil {
		roster = captureInput.Session.Log.Players
	}
	return map[string]any{"battle": modernBattle(b), "source": source, "sourceLabel": sourceLabel(), "provisional": isProvisionalDisplay(), "graph": modernGraph(b), "history": hs, "roster": roster, "running": stats.Running, "stageLabel": stageText(stats.Stage), "adapters": adapters, "adapter": modernAdapter, "adapterLabel": label, "overlay": map[string]any{"shown": overlayShown, "clickThrough": clickThrough, "opacity": opacity, "mode": modernOverlayMode}, "idle": idle, "speed": speed, "diagnostics": stats, "error": modernError, "replaying": session != nil && session.Playing, "testMode": modernTestPath != ""}
}
func modernStartCapture() error {
	if captureCtl.running() {
		stopCapture()
		return nil
	}
	if modernAdapter < 0 || modernAdapter >= len(modernDevices) {
		return fmt.Errorf("使用するネットワークアダプターを選択してください")
	}
	live, e := newLiveInput("jp", time.Now(), idle)
	if e != nil {
		return e
	}
	profile, _ := profileFor("jp")
	if e = captureCtl.start(modernDevices[modernAdapter], profile); e != nil {
		return e
	}
	enableLiveRotation(live)
	jpInput = &JPProvisionalInput{Live: live, Seen: map[string]bool{}, Roles: map[string]JPRole{}}
	captureInput = live
	captureActive = true
	jpCandidateView = true
	jpFromCapture = true
	jpDisplay = JPStats{}
	session = nil
	historyIndex = -1
	selectedPlayer = ""
	jpUpdateAt = time.Time{}
	return nil
}
func modernAction(command, value string) (string, error) {
	modernError = ""
	var e error
	switch command {
	case "capture":
		e = modernStartCapture()
	case "adapter":
		if captureCtl.running() {
			e = fmt.Errorf("取得を停止してから変更してください")
		} else {
			var index int
			index, e = strconv.Atoi(value)
			if e == nil && index >= 0 && index < len(modernDevices) {
				modernAdapter = index
			} else {
				e = fmt.Errorf("アダプターを選択してください")
			}
		}
	case "refresh":
		if !captureCtl.running() {
			modernRefreshAdapters()
		}
	case "sample":
		onCommand(idSample, 0)
	case "reset":
		onCommand(idReset, 0)
	case "overlay":
		onCommand(idOverlay, 0)
	case "click":
		onCommand(idClick, 0)
	case "opacity":
		n, err := strconv.Atoi(value)
		if err != nil {
			return "", fmt.Errorf("透明度を指定してください")
		}
		opacity = uint8(max(60, min(255, n)))
		updateOverlayStyle()
	case "mode":
		if value != "compact" && value != "detail" {
			return "", fmt.Errorf("表示モードが不正です")
		}
		modernOverlayMode = value
		overlayUserSized = false
		modernResizeOverlay()
		invalidate(overlayWindow)
	case "overlay-reset":
		screen, _, _ := pMetrics.Call(0)
		pSetWindowPos.Call(overlayWindow, 0, screen-480, 110, 460, 360, 0x0014)
		overlayUserSized = false
		modernResizeOverlay()
	case "idle":
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || (n != 3000 && n != 5000 && n != 10000 && n != 15000) {
			return "", fmt.Errorf("終了判定の秒数が不正です")
		}
		idle = n
		if session != nil {
			session.Idle = idle
			_ = session.recompute()
		}
	case "speed":
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil || (n != 1 && n != 2 && n != 4) {
			return "", fmt.Errorf("再生速度が不正です")
		}
		speed = n
		if session != nil {
			session.Speed = n
		}
	case "import":
		path, err := dialogPath(modernHandle, false)
		e = err
		if e == nil && path != "" {
			var l CombatLog
			l, e = readLog(path)
			if e == nil {
				stopCapture()
				jpFromCapture = false
				jpInput = nil
				loadSession(l, true)
			}
		}
	case "replay":
		path, err := dialogPCAPPath(modernHandle)
		e = err
		if e == nil && path != "" {
			e = loadCandidatePCAP(path)
		}
	case "history":
		i, err := strconv.Atoi(value)
		if err != nil || i < 0 || i >= len(history) {
			return "", fmt.Errorf("履歴を選択してください")
		}
		if captureActive {
			stopCapture()
		}
		jpFromCapture = false
		jpInput = nil
		jpCandidateView = false
		if session != nil {
			session.Playing = false
		}
		historyIndex = i
		selectedPlayer = ""
		updateDisplay()
	case "actor":
		i, err := strconv.Atoi(value)
		if err != nil || captureInput == nil || captureInput.Session == nil || i < 0 || i >= len(captureInput.Session.Log.Players) {
			return "", fmt.Errorf("識別子を選択してください")
		}
		e = captureInput.selectActor(i, !captureInput.Session.Log.Players[i].Party)
		updateDisplay()
	case "diagnostic":
		path, err := dialogPath(modernHandle, true)
		e = err
		if e == nil && path != "" {
			data, _ := json.MarshalIndent(map[string]any{"stats": captureCtl.snapshot(), "measurements": trafficMeasurements, "liveVerified": false}, "", "  ")
			e = os.WriteFile(path, data, 0600)
		}
	case "save":
		b := displayBattle()
		if b == nil {
			return "", fmt.Errorf("保存する戦闘がありません")
		}
		path, err := dialogPath(modernHandle, true)
		e = err
		if e == nil && path != "" {
			data, _ := json.MarshalIndent(map[string]any{"source": sourceLabel(), "battle": b, "liveVerified": false}, "", "  ")
			e = os.WriteFile(path, data, 0600)
		}
	case "record":
		if recordingDirectory == "" {
			exe, _ := os.Executable()
			root := filepath.Join(filepath.Dir(exe), "captures")
			e = os.MkdirAll(root, 0700)
			if e == nil {
				recordingDirectory, e = os.MkdirTemp(root, "session-")
			}
		}
		if e == nil {
			e = captureCtl.record(recordingDirectory, value)
		}
	case "delete-records":
		if captureCtl.running() {
			e = fmt.Errorf("キャプチャ停止後に削除してください")
		} else if recordingDirectory != "" {
			e = deleteLocalRecordings(recordingDirectory)
		}
	case "measure":
		if value != "A-idle" && value != "B-attack" && value != "C-skill" {
			return "", fmt.Errorf("測定条件が不正です")
		}
		beginTrafficMeasure(value)
		if captureSetupError != "" {
			e = fmt.Errorf("%s", captureSetupError)
		}
	default:
		e = fmt.Errorf("未対応の操作です")
	}
	if e != nil {
		modernError = e.Error()
		return "", e
	}
	modernSavePreferences()
	return "", nil
}

type modernPreferences struct {
	Mode       string `json:"overlayMode"`
	Opacity    uint8  `json:"opacity"`
	Click      bool   `json:"clickThrough"`
	Shown      bool   `json:"shown"`
	X, Y, W, H int32
	Adapter    int   `json:"adapter"`
	Idle       int64 `json:"idle"`
}

func modernPreferencesPath() string {
	return filepath.Join(filepath.Dir(historyDir), "display-settings.json")
}
func modernSavePreferences() {
	if !modernUI {
		return
	}
	var r rect
	user32.NewProc("GetWindowRect").Call(overlayWindow, uintptr(unsafe.Pointer(&r)))
	pref := modernPreferences{modernOverlayMode, opacity, clickThrough, overlayShown, r.Left, r.Top, r.Right - r.Left, r.Bottom - r.Top, modernAdapter, idle}
	data, _ := json.MarshalIndent(pref, "", "  ")
	_ = os.MkdirAll(filepath.Dir(modernPreferencesPath()), 0700)
	_ = os.WriteFile(modernPreferencesPath(), data, 0600)
}
func modernLoadPreferences() {
	data, e := os.ReadFile(modernPreferencesPath())
	if e != nil {
		return
	}
	var p modernPreferences
	if json.Unmarshal(data, &p) != nil {
		return
	}
	if p.Mode == "compact" || p.Mode == "detail" {
		modernOverlayMode = p.Mode
	}
	if p.Opacity >= 60 {
		opacity = p.Opacity
	}
	clickThrough = p.Click
	overlayShown = p.Shown
	if p.Adapter >= 0 && p.Adapter < len(modernDevices) {
		modernAdapter = p.Adapter
	}
	if p.Idle == 3000 || p.Idle == 5000 || p.Idle == 10000 || p.Idle == 15000 {
		idle = p.Idle
	}
	width, _, _ := pMetrics.Call(0)
	height, _, _ := pMetrics.Call(1)
	if p.W >= 340 && p.W < 1800 && p.H >= 170 && p.H < 1500 {
		x := max(0, min(p.X, int32(width)-100))
		y := max(0, min(p.Y, int32(height)-80))
		pSetWindowPos.Call(overlayWindow, 0, uintptr(x), uintptr(y), uintptr(p.W), uintptr(p.H), 0x0014)
		overlayUserSized = true
	}
	updateOverlayStyle()
}
func runModernUI() {
	modernUI = true
	_ = os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--disable-gpu")
	modernRefreshAdapters()
	modernLoadPreferences()
	modernView = webview.NewWithOptions(webview.WebViewOptions{AutoFocus: true, DataPath: filepath.Join(filepath.Dir(historyDir), "webview-cache"), WindowOptions: webview.WindowOptions{Title: "MABI DPS JP — 戦闘分析", Width: 1280, Height: 930, Center: true}})
	if modernView == nil {
		alert(0, "WebView2を起動できません。Microsoft WebView2 Runtimeを確認するか、-legacy-uiで旧画面を起動してください。")
		return
	}
	defer modernView.Destroy()
	defer captureCtl.stop()
	defer modernSavePreferences()
	modernHandle = uintptr(modernView.Window())
	pShowWindow.Call(modernHandle, 1)
	modernView.SetSize(860, 650, webview.HintMin)
	dark := int32(1)
	syscallDwm := windowsDwm()
	syscallDwm.Call(modernHandle, 20, uintptr(unsafe.Pointer(&dark)), 4)
	_ = modernView.Bind("appSnapshot", modernSnapshot)
	_ = modernView.Bind("appAction", modernAction)
	_ = modernView.Bind("appReport", func(report string) {
		if modernTestPath != "" {
			var js any
			if json.Unmarshal([]byte(report), &js) == nil {
				data, _ := json.MarshalIndent(map[string]any{"frontend": js, "state": modernSnapshot(), "standalone": true, "externalBrowserOpened": false}, "", "  ")
				_ = os.WriteFile(modernTestPath, data, 0600)
				_ = captureOwnOverlay(filepath.Join(filepath.Dir(modernTestPath), "modern-overlay.png"))
			}
		}
	})
	html, _ := assets.ReadFile("modern-ui/index.html")
	css, _ := assets.ReadFile("modern-ui/style.css")
	js, _ := assets.ReadFile("modern-ui/app.js")
	content := strings.ReplaceAll(string(html), "__STYLE__", string(css))
	content = strings.ReplaceAll(content, "__SCRIPT__", string(js))
	modernView.SetHtml(content)
	modernResizeOverlay()
	modernView.Run()
}
