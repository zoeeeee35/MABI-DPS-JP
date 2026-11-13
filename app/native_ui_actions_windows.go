//go:build native && windows

package main

import (
	"encoding/json"
	"os"
	"syscall"
	"unsafe"
)

func ensureNativeCapture() {
	if captureWindow == 0 {
		captureEmbedded = true
		openCaptureSettings()
	}
}
func nativeSavePNG() {
	buffer := make([]uint16, 32768)
	filter := []uint16{}
	for _, s := range []string{"PNG画像 (*.png)", "*.png", ""} {
		v, _ := syscall.UTF16FromString(s)
		filter = append(filter, v...)
	}
	ofn := openFileName{Owner: mainWindow, Filter: &filter[0], File: &buffer[0], MaxFile: uint32(len(buffer)), FilterIndex: 1, Flags: 0x00080000 | 0x00000002 | 0x00000008, DefaultExt: u16("png")}
	ofn.Size = uint32(unsafe.Sizeof(ofn))
	ok, _, _ := pGetSave.Call(uintptr(unsafe.Pointer(&ofn)))
	if ok != 0 {
		if e := captureOwnSurface(syscall.UTF16ToString(buffer), mainWindow, true); e != nil {
			alert(mainWindow, e.Error())
		}
	}
}
func nativeSaveDiagnostic() {
	path, e := dialogPath(mainWindow, true)
	if e != nil {
		alert(mainWindow, e.Error())
		return
	}
	if path == "" {
		return
	}
	data, _ := json.MarshalIndent(map[string]any{"stats": captureCtl.snapshot(), "measurements": trafficMeasurements, "liveVerified": false}, "", "  ")
	if e = os.WriteFile(path, data, 0600); e != nil {
		alert(mainWindow, e.Error())
	}
}
