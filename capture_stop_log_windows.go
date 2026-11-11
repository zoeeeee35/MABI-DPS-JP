//go:build native && windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Metadata only: no packet bytes, names, IP addresses, or adapter identifiers.
func recordCaptureStop(reason string) {
	if historyDir == "" {
		return
	}
	data := map[string]any{"at": time.Now().UTC().Format(time.RFC3339), "reason": reason}
	if captureInput != nil {
		data["segments"] = captureInput.Rotations
		data["lastSegmentReason"] = captureInput.LastRotation
		data["registeredDamageActors"] = len(captureInput.actors)
		if captureInput.Session != nil {
			data["retainedDamageEvents"] = len(captureInput.Session.Log.Events)
		}
	}
	b, e := json.MarshalIndent(data, "", "  ")
	if e != nil {
		return
	}
	dir := filepath.Dir(historyDir)
	if os.MkdirAll(dir, 0700) == nil {
		_ = os.WriteFile(filepath.Join(dir, "capture-last-stop.json"), b, 0600)
	}
}
