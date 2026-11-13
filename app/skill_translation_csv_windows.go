//go:build native && windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

func nativeOpenTranslationCSV() { nativeOpenNamedCSV(skillTranslationCSVName) }
func nativeOpenEffectCSV()      { nativeOpenNamedCSV(effectTranslationCSVName) }
func nativeOpenNamedCSV(name string) {
	exe, e := os.Executable()
	if e != nil {
		alert(mainWindow, "EXEの場所を確認できません")
		return
	}
	path := filepath.Join(filepath.Dir(exe), name)
	if _, e = os.Stat(path); e != nil {
		alert(mainWindow, "EXEと同じフォルダーに"+name+"を置いてください")
		return
	}
	verb, _ := syscall.UTF16PtrFromString("open")
	file, _ := syscall.UTF16PtrFromString(path)
	r, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW").Call(mainWindow, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), 0, 0, 1)
	if r <= 32 {
		alert(mainWindow, "CSVを開けません。エクスプローラーからCSV UTF-8対応の表計算ソフトで開いてください")
	}
}

func nativeOpenApplicationLogs() {
	if appDiagnostics == nil {
		alert(mainWindow, "動作ログを作成できていません。保存先の書き込み権限を確認してください")
		return
	}
	verb, _ := syscall.UTF16PtrFromString("open")
	file, _ := syscall.UTF16PtrFromString(appDiagnostics.dir)
	r, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("ShellExecuteW").Call(mainWindow, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(file)), 0, 0, 1)
	if r <= 32 {
		alert(mainWindow, "ログフォルダーを開けません："+appDiagnostics.dir)
	}
}
