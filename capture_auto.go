package main

import "strings"

type CaptureDevice struct {
	Name      string
	Label     string
	Suggested bool
}

type autoCaptureSettings struct {
	Enabled     bool   `json:"enabled"`
	AutoSelf    bool   `json:"autoSelf"`
	AdapterName string `json:"adapterName,omitempty"`
}

// Do not guess a device from its position in the adapter list. A suggestion
// means an address matched a connection owned by the game process.
func autoCaptureDevice(devices []CaptureDevice, remembered string) int {
	for i, d := range devices {
		if d.Suggested && d.Name == remembered && remembered != "" {
			return i
		}
	}
	physical := []int{}
	all := []int{}
	for i, d := range devices {
		if !d.Suggested {
			continue
		}
		all = append(all, i)
		if !strings.Contains(strings.ToLower(d.Name+" "+d.Label), "loopback") {
			physical = append(physical, i)
		}
	}
	if len(physical) == 1 {
		return physical[0]
	}
	if len(all) == 1 {
		return all[0]
	}
	return -1
}

func autoCaptureCanScan(enabled, paused, running, scanInFlight bool) bool {
	return enabled && !paused && !running && !scanInFlight
}
