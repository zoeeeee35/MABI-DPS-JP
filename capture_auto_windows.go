//go:build native && windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

var autoCapturePrefs = autoCaptureSettings{Enabled: true, AutoSelf: true}
var autoCapturePrefsPath string
var autoCaptureReady, autoCapturePaused, autoCaptureScanning bool
var autoCaptureGeneration uint64
var autoCaptureLastScan time.Time
var autoCaptureStatus = "ゲーム接続を待っています"
var enumerateAutoCaptureDevices = captureDevices
var startAutoCapture = startOrStopCapture

type autoCaptureResult struct {
	devices    []CaptureDevice
	err        error
	generation uint64
}

var autoCaptureResults = make(chan autoCaptureResult, 1)

func initAutoCapture(dataDir string, disabled bool) {
	autoCapturePrefsPath = filepath.Join(dataDir, "capture-start.json")
	if b, e := readBoundedFile(autoCapturePrefsPath, 16<<10); e == nil {
		prefs := autoCapturePrefs
		if json.Unmarshal(b, &prefs) == nil {
			autoCapturePrefs = prefs
		}
	}
	soloAutoEnabled = autoCapturePrefs.AutoSelf
	autoCaptureReady = !disabled
}
func saveAutoCapturePrefs() {
	if autoCapturePrefsPath == "" {
		return
	}
	if e := os.MkdirAll(filepath.Dir(autoCapturePrefsPath), 0700); e != nil {
		return
	}
	b, _ := json.MarshalIndent(autoCapturePrefs, "", "  ")
	if e := os.WriteFile(autoCapturePrefsPath, b, 0600); e != nil {
		captureSetupError = "自動取得の設定を保存できません"
	}
}
func pauseAutoCapture() {
	autoCapturePaused = true
	autoCaptureGeneration++
	autoCaptureStatus = "手動停止中（この起動中は自動再開しません）"
}
func toggleAutoCapture() {
	autoCapturePrefs.Enabled = !autoCapturePrefs.Enabled
	autoCapturePaused = false
	autoCaptureGeneration++
	autoCaptureLastScan = time.Time{}
	if autoCapturePrefs.Enabled {
		autoCaptureStatus = "ゲーム接続を待っています"
	} else {
		autoCaptureStatus = "自動開始OFF（手動開始できます）"
	}
	saveAutoCapturePrefs()
}
func rememberCaptureAdapter(index int) {
	if index >= 0 && index < len(captureAdapterList) {
		autoCaptureGeneration++
		autoCapturePrefs.AdapterName = captureAdapterList[index].Name
		saveAutoCapturePrefs()
	}
}
func populateCaptureAdapters(devices []CaptureDevice) {
	captureAdapterList = devices
	send(controls[idCaptureAdapters], 0x014b, 0, 0)
	for _, d := range devices {
		sendString(controls[idCaptureAdapters], 0x0143, d.Label)
	}
	index := autoCaptureDevice(devices, autoCapturePrefs.AdapterName)
	send(controls[idCaptureAdapters], 0x014e, uintptr(index), 0)
}
func tickAutoCapture(now time.Time) {
	if !autoCaptureReady {
		return
	}
	select {
	case result := <-autoCaptureResults:
		autoCaptureScanning = false
		if result.generation != autoCaptureGeneration || !autoCaptureCanScan(autoCapturePrefs.Enabled, autoCapturePaused, captureCtl.running() || captureActive, false) {
			return
		}
		if result.err != nil {
			autoCaptureStatus = result.err.Error()
			invalidate(mainWindow)
			return
		}
		populateCaptureAdapters(result.devices)
		index := autoCaptureDevice(result.devices, autoCapturePrefs.AdapterName)
		if index < 0 {
			autoCaptureStatus = "ゲーム接続待ち（候補が複数ならアダプターを指定してください）"
			invalidate(mainWindow)
			return
		}
		// The normal start path initializes TCP/JP9 and the auto-self state.
		startAutoCapture()
		if captureActive {
			autoCaptureStatus = "ゲーム接続を検出し、自動で開始しました"
		} else {
			autoCaptureStatus = captureSetupError
		}
		updateCaptureSettings()
		invalidate(mainWindow)
	default:
	}
	if !autoCaptureCanScan(autoCapturePrefs.Enabled, autoCapturePaused, captureCtl.running() || captureActive, autoCaptureScanning) {
		return
	}
	interval := 5 * time.Second
	stats := captureCtl.snapshot()
	if stats.Stage == "driver_missing" || stats.Stage == "permission_denied" || stats.Stage == "adapter_unavailable" {
		interval = 30 * time.Second
		autoCaptureStatus = stats.Detail
	}
	if now.Sub(autoCaptureLastScan) < interval {
		return
	}
	autoCaptureLastScan = now
	autoCaptureScanning = true
	generation := autoCaptureGeneration
	go func() {
		devices, e := enumerateAutoCaptureDevices()
		autoCaptureResults <- autoCaptureResult{devices, e, generation}
	}()
}
