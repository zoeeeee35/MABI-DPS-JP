//go:build native && windows

package main

import (
	"testing"
	"time"
)

func TestAutomaticCaptureStartsAfterGameAppearsAndRejectsStaleScan(t *testing.T) {
	oldPrefs, oldReady, oldPaused, oldScan, oldGen, oldLast, oldStatus := autoCapturePrefs, autoCaptureReady, autoCapturePaused, autoCaptureScanning, autoCaptureGeneration, autoCaptureLastScan, autoCaptureStatus
	oldCtl, oldActive, oldEnum, oldStart, oldChannel, oldPath, oldAdapters := captureCtl, captureActive, enumerateAutoCaptureDevices, startAutoCapture, autoCaptureResults, autoCapturePrefsPath, captureAdapterList
	defer func() {
		autoCapturePrefs, autoCaptureReady, autoCapturePaused, autoCaptureScanning, autoCaptureGeneration, autoCaptureLastScan, autoCaptureStatus = oldPrefs, oldReady, oldPaused, oldScan, oldGen, oldLast, oldStatus
		captureCtl, captureActive, enumerateAutoCaptureDevices, startAutoCapture, autoCaptureResults, autoCapturePrefsPath, captureAdapterList = oldCtl, oldActive, oldEnum, oldStart, oldChannel, oldPath, oldAdapters
	}()
	autoCapturePrefs = autoCaptureSettings{Enabled: true, AutoSelf: true}
	autoCaptureReady = true
	autoCapturePaused = false
	autoCaptureScanning = false
	autoCaptureLastScan = time.Time{}
	autoCapturePrefsPath = ""
	captureCtl = &CaptureController{}
	captureActive = false
	autoCaptureResults = make(chan autoCaptureResult, 1)
	calls, starts := 0, 0
	enumerateAutoCaptureDevices = func() ([]CaptureDevice, error) { calls++; return []CaptureDevice{{Name: "game", Suggested: true}}, nil }
	startAutoCapture = func() { starts++; captureActive = true }
	now := time.Now()
	tickAutoCapture(now)
	var result autoCaptureResult
	select {
	case result = <-autoCaptureResults:
	case <-time.After(2 * time.Second):
		t.Fatal("scan did not finish")
	}
	autoCaptureResults <- result
	tickAutoCapture(now.Add(time.Second))
	if calls != 1 || starts != 1 || !captureActive {
		t.Fatal(calls, starts, captureActive)
	}
	pauseAutoCapture()
	captureActive = false
	tickAutoCapture(now.Add(20 * time.Second))
	if calls != 1 || starts != 1 {
		t.Fatal("manual stop restarted capture")
	}
	autoCapturePaused = false
	autoCaptureResults <- result
	tickAutoCapture(now.Add(21 * time.Second))
	if starts != 1 {
		t.Fatal("stale scan started capture")
	}
}
