package main

import "testing"

func TestAutomaticCaptureAdapterRequiresGameConnection(t *testing.T) {
	for _, tc := range []struct {
		devices    []CaptureDevice
		remembered string
		want       int
	}{
		{[]CaptureDevice{{Name: "one"}}, "one", -1},
		{[]CaptureDevice{{Name: "one", Suggested: true}}, "", 0},
		{[]CaptureDevice{{Name: "NPF_Loopback", Suggested: true}, {Name: "physical", Suggested: true}}, "", 1},
		{[]CaptureDevice{{Name: "a", Suggested: true}, {Name: "b", Suggested: true}}, "", -1},
		{[]CaptureDevice{{Name: "a", Suggested: true}, {Name: "b", Suggested: true}}, "b", 1},
		{[]CaptureDevice{{Name: "old"}, {Name: "new", Suggested: true}}, "old", 1},
		{nil, "", -1},
	} {
		if got := autoCaptureDevice(tc.devices, tc.remembered); got != tc.want {
			t.Fatal(tc, got)
		}
	}
}
func TestAutomaticCaptureHonorsManualStopAndDisabledState(t *testing.T) {
	for _, tc := range []struct{ enabled, paused, running, busy, want bool }{
		{true, false, false, false, true}, {true, true, false, false, false}, {false, false, false, false, false}, {true, false, true, false, false}, {true, false, false, true, false},
	} {
		if got := autoCaptureCanScan(tc.enabled, tc.paused, tc.running, tc.busy); got != tc.want {
			t.Fatal(tc)
		}
	}
}
