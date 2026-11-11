//go:build native && windows

package main

import "golang.org/x/sys/windows"

// Keep the named object alive for the lifetime of the app. Windows releases
// the handle even if the process exits unexpectedly.
func openAppInstance(name string) (windows.Handle, error) {
	key, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}
	handle, err := windows.CreateMutex(nil, false, key)
	if err != nil && handle != 0 {
		_ = windows.CloseHandle(handle)
		return 0, err
	}
	return handle, err
}
