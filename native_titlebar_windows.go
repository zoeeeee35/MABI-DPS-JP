//go:build native && windows

package main

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

var nativeTitleBarState = map[string]any{}

func applyNativeDarkTitle(window uintptr) {
	dll := windows.NewLazySystemDLL("dwmapi.dll")
	set := dll.NewProc("DwmSetWindowAttribute")
	if set.Find() != nil {
		return
	}
	dark := int32(1)
	hr, _, _ := set.Call(window, 20, uintptr(unsafe.Pointer(&dark)), 4)
	if int32(hr) != 0 {
		hr, _, _ = set.Call(window, 19, uintptr(unsafe.Pointer(&dark)), 4)
	}
	nativeTitleBarState["darkModeResult"] = int32(hr)
	caption, text := uint32(0x211510), uint32(0xF5F5F5)
	hr, _, _ = set.Call(window, 35, uintptr(unsafe.Pointer(&caption)), 4)
	nativeTitleBarState["captionResult"] = int32(hr)
	hr, _, _ = set.Call(window, 36, uintptr(unsafe.Pointer(&text)), 4)
	nativeTitleBarState["textResult"] = int32(hr)
}
