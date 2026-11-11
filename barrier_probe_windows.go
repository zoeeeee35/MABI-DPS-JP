//go:build native && windows

package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func runBarrierProbe(kind, report, sound string) {
	result := map[string]any{"kind": kind, "verifiedAt": time.Now().UTC()}
	var err error
	switch kind {
	case "shortcut":
		exe, e := os.Executable()
		if e != nil {
			err = e
		} else {
			err = createBarrierShortcut(filepath.Join(filepath.Dir(report), "MABI DPS JP.lnk"), exe)
		}
	case "toast":
		err = sendBarrierToast(context.Background(), BarrierEvent{"before", nextBarrierTime(time.Now()), 5})
		result["visualDisplayVerified"] = false
	case "sound":
		c := defaultBarrierSettings()
		c.SoundPath = sound
		c.Volume = 10
		err = playBarrierAudio(c)
		if err == nil {
			time.Sleep(500 * time.Millisecond)
			stopBarrierAudio()
		}
		result["audibleOutputVerified"] = false
	default:
		result["error"] = "unknown probe"
	}
	if err != nil {
		result["error"] = err.Error()
	} else {
		result["apiSuccess"] = true
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	_ = os.WriteFile(report, data, 0600)
}
