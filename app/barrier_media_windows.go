//go:build native && windows

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const barrierAppID = "MABI.DPS.JP"

type barrierCOMObject struct{ Vtbl *[32]uintptr }

var barrierAudioMu sync.Mutex
var barrierToastMu sync.Mutex
var barrierToastProcess *os.Process
var barrierToastGeneration uint64
var barrierResults = make(chan string, 16)
var barrierCtx, barrierCancel = context.WithCancel(context.Background())

func barrierReport(s string) {
	select {
	case barrierResults <- s:
	default:
	}
}
func mciCommand(s string) error {
	result, _, _ := syscall.NewLazyDLL("winmm.dll").NewProc("mciSendStringW").Call(uintptr(unsafe.Pointer(u16(s))), 0, 0, 0)
	if result != 0 {
		return fmt.Errorf("音声を再生できません（Windows音声エラー %d）", result)
	}
	return nil
}
func stopBarrierMCI() {
	barrierAudioMu.Lock()
	defer barrierAudioMu.Unlock()
	_ = mciCommand("close mabidpsbarrier")
}
func playBarrierMCI(c BarrierSettings) error {
	barrierAudioMu.Lock()
	defer barrierAudioMu.Unlock()
	absolute, e := localBarrierSound(c.SoundPath)
	if e != nil {
		return e
	}
	_ = mciCommand("close mabidpsbarrier")
	if e = mciCommand("open \"" + absolute + "\" type mpegvideo alias mabidpsbarrier"); e != nil {
		return e
	}
	if e = mciCommand(fmt.Sprintf("setaudio mabidpsbarrier volume to %d", max(0, min(100, c.Volume))*10)); e != nil {
		_ = mciCommand("close mabidpsbarrier")
		return e
	}
	return mciCommand("play mabidpsbarrier")
}

// Own Start-menu identity, not another application's notification identity.
// IShellLinkW + IPropertyStore set PKEY_AppUserModel_ID before IPersistFile.Save.
func createBarrierShortcut(path, exe string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	keep := []*uint16{}
	textPointer := func(s string) uintptr { p := u16(s); keep = append(keep, p); return uintptr(unsafe.Pointer(p)) }
	defer func() { runtime.KeepAlive(keep) }()
	ole := syscall.NewLazyDLL("ole32.dll")
	hr, _, _ := ole.NewProc("CoInitializeEx").Call(0, 2)
	if int32(hr) < 0 {
		return fmt.Errorf("通知IDの初期化失敗 (%X)", hr)
	}
	defer ole.NewProc("CoUninitialize").Call()
	guid := func(s string) [16]byte {
		var g [16]byte
		ole.NewProc("CLSIDFromString").Call(uintptr(unsafe.Pointer(u16(s))), uintptr(unsafe.Pointer(&g)))
		return g
	}
	cls := guid("{00021401-0000-0000-C000-000000000046}")
	iid := guid("{000214F9-0000-0000-C000-000000000046}")
	var link *barrierCOMObject
	hr, _, _ = ole.NewProc("CoCreateInstance").Call(uintptr(unsafe.Pointer(&cls)), 0, 1, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&link)))
	if int32(hr) < 0 {
		return fmt.Errorf("通知ショートカット作成失敗 (%X)", hr)
	}
	call := func(object *barrierCOMObject, index int, args ...uintptr) uintptr {
		method := object.Vtbl[index]
		all := append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)
		r, _, _ := syscall.SyscallN(method, all...)
		return r
	}
	defer call(link, 2)
	if hr = call(link, 20, textPointer(exe)); int32(hr) < 0 {
		return fmt.Errorf("通知EXE設定失敗")
	}
	hr = call(link, 17, textPointer(exe), 0)
	if int32(hr) < 0 {
		return fmt.Errorf("通知アイコン設定失敗")
	}
	storeIID := guid("{886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99}")
	var store *barrierCOMObject
	if hr = call(link, 0, uintptr(unsafe.Pointer(&storeIID)), uintptr(unsafe.Pointer(&store))); int32(hr) < 0 {
		return fmt.Errorf("通知プロパティを設定できません")
	}
	defer call(store, 2)
	key := struct {
		GUID [16]byte
		PID  uint32
	}{guid("{9F4C2855-9F79-4B39-A8D0-E1D42DE1D5F3}"), 5}
	value := struct {
		VT             uint16
		Reserved       [3]uint16
		Value, Padding uintptr
	}{VT: 31, Value: textPointer(barrierAppID)}
	if hr = call(store, 6, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&value))); int32(hr) < 0 {
		return fmt.Errorf("通知IDの設定失敗")
	}
	if hr = call(store, 7); int32(hr) < 0 {
		return fmt.Errorf("通知IDの保存失敗")
	}
	persistIID := guid("{0000010B-0000-0000-C000-000000000046}")
	var persist *barrierCOMObject
	if hr = call(link, 0, uintptr(unsafe.Pointer(&persistIID)), uintptr(unsafe.Pointer(&persist))); int32(hr) < 0 {
		return fmt.Errorf("通知リンクを保存できません")
	}
	defer call(persist, 2)
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	if hr = call(persist, 6, textPointer(path), 1); int32(hr) < 0 {
		return fmt.Errorf("通知リンクの保存失敗 (%X)", hr)
	}
	runtime.KeepAlive(value)
	return nil
}
func encodedPowerShell(script string) string {
	chars := utf16.Encode([]rune(script))
	b := make([]byte, len(chars)*2)
	for i, v := range chars {
		binary.LittleEndian.PutUint16(b[i*2:], v)
	}
	return base64.StdEncoding.EncodeToString(b)
}
func xmlBarrierText(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&apos;").Replace(s)
}
func stopBarrierToast() {
	barrierToastMu.Lock()
	defer barrierToastMu.Unlock()
	barrierToastGeneration++
	if barrierToastProcess != nil {
		_ = barrierToastProcess.Kill()
		barrierToastProcess = nil
	}
}
func sendBarrierToast(ctx context.Context, event BarrierEvent) error {
	barrierToastMu.Lock()
	generation := barrierToastGeneration
	barrierToastMu.Unlock()
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	shortcut := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs", "MABI DPS JP.lnk")
	if !barrierShortcutCurrent(shortcut, exe) {
		if e = createBarrierShortcut(shortcut, exe); e != nil {
			return e
		}
	}
	text := strings.ReplaceAll(xmlBarrierText(event.Text()), "\n", "</text><text>")
	xml := "<toast duration=\"short\"><visual><binding template=\"ToastGeneric\"><text>MabiDPS-JP｜召喚の結界</text><text>" + text + "</text></binding></visual><audio silent=\"true\"/></toast>"
	// Data is base64 encoded; no shell interpolation of paths or custom file names.
	payload := base64.StdEncoding.EncodeToString([]byte(xml))
	script := `$ErrorActionPreference='Stop';[Windows.UI.Notifications.ToastNotificationManager,Windows.UI.Notifications,ContentType=WindowsRuntime]|Out-Null;[Windows.UI.Notifications.ToastNotification,Windows.UI.Notifications,ContentType=WindowsRuntime]|Out-Null;[Windows.Data.Xml.Dom.XmlDocument,Windows.Data.Xml.Dom.XmlDocument,ContentType=WindowsRuntime]|Out-Null;$doc=New-Object Windows.Data.Xml.Dom.XmlDocument;$doc.LoadXml([Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('` + payload + `')));$notifier=[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('` + barrierAppID + `');$setting=[int]$notifier.Setting;if($setting -ne 0){if($setting -ge 1 -and $setting -le 4){exit (20+$setting)};exit 25};$toast=[Windows.UI.Notifications.ToastNotification]::new($doc);$toast.ExpirationTime=[DateTimeOffset]::Now.AddMinutes(1);$notifier.Show($toast)`
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodedPowerShell(script))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	barrierToastMu.Lock()
	if generation != barrierToastGeneration {
		barrierToastMu.Unlock()
		return context.Canceled
	}
	if barrierToastProcess != nil {
		_ = barrierToastProcess.Kill()
	}
	e = cmd.Start()
	if e == nil {
		barrierToastProcess = cmd.Process
	}
	barrierToastMu.Unlock()
	if e == nil {
		e = cmd.Wait()
	}
	barrierToastMu.Lock()
	canceled := generation != barrierToastGeneration
	if barrierToastProcess == cmd.Process {
		barrierToastProcess = nil
	}
	barrierToastMu.Unlock()
	if canceled {
		return context.Canceled
	}
	if e != nil {
		if exit, ok := e.(*exec.ExitError); ok {
			return barrierToastExitError(exit.ExitCode())
		}
		return fmt.Errorf("Windows通知を送信できません（通知設定・PowerShellを確認してください）")
	}
	return nil
}

func barrierShortcutCurrent(path, exe string) bool {
	data, e := os.ReadFile(path)
	if e != nil || len(data) > 1<<20 {
		return false
	}
	wide := func(s string) []byte {
		u := utf16.Encode([]rune(s))
		b := make([]byte, len(u)*2)
		for i, v := range u {
			binary.LittleEndian.PutUint16(b[i*2:], v)
		}
		return b
	}
	return bytes.Contains(data, wide(barrierAppID)) && (bytes.Contains(data, wide(exe)) || bytes.Contains(data, []byte(exe)))
}

func barrierToastExitError(code int) error {
	switch code {
	case 21:
		return fmt.Errorf("MABI DPS JP個別の通知が無効です。Windowsの通知アプリ一覧でMABI DPS JPを確認してください（状態1）")
	case 22:
		return fmt.Errorf("現在のWindowsユーザーの通知が無効です（状態2）。通知全体の設定を確認してください")
	case 23:
		return fmt.Errorf("Windowsの管理ポリシーで通知が禁止されています（状態3）")
	case 24:
		return fmt.Errorf("アプリの通知登録がWindowsに認識されていません（状態4）。通知全体のON/OFFとは別の問題です")
	case 25:
		return fmt.Errorf("Windowsから未知の通知状態が返されました。通知登録を確認してください")
	default:
		return fmt.Errorf("Windows通知処理でエラーが発生しました（終了コード%d）。通知登録・PowerShellを確認してください", code)
	}
}

func barrierSoundFile(path string) string {
	exe, err := os.Executable()
	if err != nil {
		return path
	}
	return barrierSoundFileAt(filepath.Dir(exe), path)
}
func barrierSoundFileAt(base, path string) string {
	if path == "" {
		path = defaultBarrierSound
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(base, filepath.FromSlash(path))
}

func localBarrierSound(path string) (string, error) {
	invalid := fmt.Errorf("通知音にはローカルドライブ上のWAVまたはMP3を選択してください")
	// Validate before any file access. Windows can resolve UNC and device paths,
	// mapped drives, or reparse points to a network share.
	volume := filepath.VolumeName(path)
	if (volume != "" && (len(volume) != 2 || !filepath.IsAbs(path))) ||
		strings.HasPrefix(path, `\`) || strings.HasPrefix(path, "/") ||
		strings.ContainsAny(path[len(volume):], ":\x00\"\r\n") {
		return "", invalid
	}
	absolute := filepath.Clean(barrierSoundFile(path))
	volume = filepath.VolumeName(absolute)
	ext := strings.ToLower(filepath.Ext(absolute))
	if len(volume) != 2 || !filepath.IsAbs(absolute) || (ext != ".wav" && ext != ".mp3") {
		return "", invalid
	}
	root := volume + `\`
	drive := windows.GetDriveType(syscall.StringToUTF16Ptr(root))
	switch drive {
	case windows.DRIVE_FIXED, windows.DRIVE_REMOVABLE, windows.DRIVE_CDROM, windows.DRIVE_RAMDISK:
	default:
		return "", invalid
	}
	current := root
	for _, component := range strings.Split(absolute[len(root):], `\`) {
		if strings.TrimRight(component, ". ") != component || !filepath.IsLocal(component) {
			return "", invalid
		}
		current = filepath.Join(current, component)
		name, err := windows.UTF16PtrFromString(current)
		if err != nil {
			return "", invalid
		}
		attributes, err := windows.GetFileAttributes(name)
		if err != nil {
			return "", fmt.Errorf("通知音ファイルが見つかりません")
		}
		if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || (current == absolute && attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0) {
			return "", invalid
		}
	}
	return absolute, nil
}
