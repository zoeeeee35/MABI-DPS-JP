//go:build native && windows

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var transferableSettings = []string{"game-overlay.json", "barrier-settings.json", "update-settings.json", "capture-start.json"}

func readTransferSetting(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 64<<10 {
		return nil, fmt.Errorf("設定ファイルの形式またはサイズが不正です: %s", filepath.Base(path))
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, (64<<10)+1))
	if err == nil && len(data) > 64<<10 {
		return nil, fmt.Errorf("設定ファイルが大きすぎます: %s", filepath.Base(path))
	}
	return data, err
}

func importPreviousSettings(source, destination string) (int, string, error) {
	if info, err := os.Stat(filepath.Join(source, "data")); err == nil && info.IsDir() {
		source = filepath.Join(source, "data")
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return 0, "", err
	}
	if destinationInfo, err := os.Stat(destination); err == nil && os.SameFile(sourceInfo, destinationInfo) {
		return 0, "", fmt.Errorf("現在と異なるフォルダーを選択してください")
	}
	type settingFile struct {
		name          string
		new, previous []byte
	}
	var files []settingFile
	for _, name := range transferableSettings {
		data, err := readTransferSetting(filepath.Join(source, name))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return 0, "", err
		}
		var fields map[string]json.RawMessage
		var typed any
		switch name {
		case "game-overlay.json":
			typed = new(gameOverlayPreferences)
		case "barrier-settings.json":
			typed = new(BarrierSettings)
		case "update-settings.json":
			typed = new(updatePreferences)
		case "capture-start.json":
			typed = new(autoCaptureSettings)
		}
		if json.Unmarshal(data, &fields) != nil || len(fields) == 0 || json.Unmarshal(data, typed) != nil {
			return 0, "", fmt.Errorf("読み込めない設定ファイルがあります: %s", name)
		}
		if settings, ok := typed.(*BarrierSettings); ok && settings.SoundPath != "" && settings.SoundPath != defaultBarrierSound {
			if _, err := localBarrierSound(settings.SoundPath); err != nil {
				for key := range fields {
					if strings.EqualFold(key, "soundPath") {
						delete(fields, key)
					}
				}
				value, _ := json.Marshal(defaultBarrierSound)
				fields["soundPath"] = value
				data, err = json.Marshal(fields)
				if err != nil {
					return 0, "", err
				}
			}
		}
		previous, err := readTransferSetting(filepath.Join(destination, name))
		if err != nil && !os.IsNotExist(err) {
			return 0, "", err
		}
		files = append(files, settingFile{name: name, new: data, previous: previous})
	}
	if len(files) == 0 {
		return 0, "", fmt.Errorf("旧版の設定が見つかりません。旧版のEXEがあるフォルダーか、その中のdataフォルダーを選択してください")
	}
	if err := os.MkdirAll(destination, 0700); err != nil {
		return 0, "", err
	}
	backup := ""
	for _, file := range files {
		if file.previous == nil {
			continue
		}
		if backup == "" {
			backup, err = os.MkdirTemp(destination, "settings-backup-")
			if err != nil {
				return 0, "", err
			}
		}
		if err := os.WriteFile(filepath.Join(backup, file.name), file.previous, 0600); err != nil {
			return 0, backup, err
		}
	}
	for i, file := range files {
		if err := replaceTransferSetting(destination, file.name, file.new); err != nil {
			var rollback []string
			for _, done := range files[:i+1] {
				if done.previous == nil {
					if err := os.Remove(filepath.Join(destination, done.name)); err != nil && !os.IsNotExist(err) {
						rollback = append(rollback, done.name)
					}
				} else if err := replaceTransferSetting(destination, done.name, done.previous); err != nil {
					rollback = append(rollback, done.name)
				}
			}
			if len(rollback) > 0 {
				return 0, backup, fmt.Errorf("引き継ぎに失敗し、元に戻せなかった設定: %s: %w", strings.Join(rollback, "、"), err)
			}
			return 0, backup, err
		}
	}
	return len(files), backup, nil
}

func replaceTransferSetting(dir, name string, data []byte) error {
	f, err := os.CreateTemp(dir, ".settings-import-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	target := filepath.Join(dir, name)
	if err := os.Rename(f.Name(), target); err != nil {
		// Some Windows directories allow writes but deny renames.
		if info, statErr := os.Lstat(target); os.IsNotExist(statErr) || (statErr == nil && info.Mode().IsRegular()) {
			return os.WriteFile(target, data, 0600)
		}
		return err
	}
	return nil
}

func selectPreviousSettingsFolder() string {
	shell := syscall.NewLazyDLL("shell32.dll")
	var display [260]uint16
	browse := struct {
		Owner, Root        uintptr
		DisplayName, Title *uint16
		Flags              uint32
		Callback, Data     uintptr
		Image              int32
	}{Owner: mainWindow, DisplayName: &display[0], Title: u16("旧版のEXEがあるフォルダー、または data フォルダーを選択"), Flags: 1}
	pidl, _, _ := shell.NewProc("SHBrowseForFolderW").Call(uintptr(unsafe.Pointer(&browse)))
	if pidl == 0 {
		return ""
	}
	defer syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemFree").Call(pidl)
	var path [260]uint16
	ok, _, _ := shell.NewProc("SHGetPathFromIDListW").Call(pidl, uintptr(unsafe.Pointer(&path[0])))
	if ok == 0 {
		return ""
	}
	return syscall.UTF16ToString(path[:])
}

func nativeImportPreviousSettings() {
	source := selectPreviousSettingsFolder()
	if source == "" {
		return
	}
	message := "このフォルダーから設定を引き継ぎますか？ 現在の設定はバックアップされます。\n\n" + source
	answer, _, _ := pMessageBox.Call(mainWindow, uintptr(unsafe.Pointer(u16(message))), uintptr(unsafe.Pointer(u16("設定の引き継ぎ"))), 0x124)
	if answer != 6 {
		return
	}
	count, backup, err := importPreviousSettings(source, filepath.Dir(historyDir))
	if err != nil {
		message = "設定を引き継げませんでした。\n" + err.Error()
		if backup != "" {
			message += "\nバックアップ: " + backup
		}
		alert(mainWindow, message)
		return
	}
	loadGameOverlay()
	if config, err := readBarrierSettings(barrierSettingsPath); err == nil {
		barrierConfig = config
		barrierScheduler.Tick(time.Now(), barrierConfig)
		if !barrierConfig.Enabled || !barrierConfig.Overlay {
			barrierBannerActive = false
			show(barrierBannerWindow, false)
		}
	}
	if session != nil {
		session.Idle = idle
		_ = session.recompute()
	}
	updateOverlayStyle()
	updateDisplay()
	message = fmt.Sprintf("%d件の設定ファイルを引き継ぎました。", count)
	if backup != "" {
		message += "\nバックアップ: " + backup
	}
	pMessageBox.Call(mainWindow, uintptr(unsafe.Pointer(u16(message))), uintptr(unsafe.Pointer(u16("設定の引き継ぎ"))), 0x40)
}

func promptFirstRunSettings(dataDir string) {
	marker := filepath.Join(dataDir, ".settings-setup-done")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		return
	}
	message := "前のバージョンの設定を引き継ぎますか？\n\n「はい」を選ぶと旧版のフォルダーを指定できます。後から「設定」→「その他」でも引き継げます。"
	answer, _, _ := pMessageBox.Call(mainWindow, uintptr(unsafe.Pointer(u16(message))), uintptr(unsafe.Pointer(u16("初回設定"))), 0x124)
	if answer == 6 {
		source := selectPreviousSettingsFolder()
		if source != "" {
			if _, backup, err := importPreviousSettings(source, dataDir); err != nil {
				message = "設定を引き継げませんでした。\n" + err.Error()
				if backup != "" {
					message += "\nバックアップ: " + backup
				}
				alert(mainWindow, message)
				return
			}
		}
	} else if answer != 7 {
		return
	}
	if err := os.MkdirAll(dataDir, 0700); err == nil {
		_ = os.WriteFile(marker, nil, 0600)
	}
}
