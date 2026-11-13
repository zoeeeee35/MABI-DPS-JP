//go:build native && windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type updatePreferences struct {
	CheckOnStartup bool      `json:"checkOnStartup"`
	LastCheck      time.Time `json:"lastCheck"`
}
type updateEvent struct {
	offer       *updateOffer
	stage       string
	err         error
	downloading bool
}

var updatePrefs = updatePreferences{CheckOnStartup: true}
var updatePreferencesPath string
var updateEvents = make(chan updateEvent, 2)
var updateBusy bool
var updateExiting bool
var pendingUpdate *updateOffer
var updateStatus = "未確認"
var updateCancel context.CancelFunc

func initUpdateRuntime(dataDir string, test bool) {
	updatePreferencesPath = filepath.Join(dataDir, "update-settings.json")
	if b, e := readBoundedFile(updatePreferencesPath, 16<<10); e == nil {
		p := updatePrefs
		if json.Unmarshal(b, &p) == nil {
			updatePrefs = p
		}
	}
	if !test && updatePrefs.CheckOnStartup && time.Since(updatePrefs.LastCheck) > 24*time.Hour {
		beginUpdateCheck()
	}
}
func saveUpdatePreferences() {
	if updatePreferencesPath == "" {
		return
	}
	b, _ := json.MarshalIndent(updatePrefs, "", "  ")
	if e := os.MkdirAll(filepath.Dir(updatePreferencesPath), 0700); e == nil {
		if e = os.WriteFile(updatePreferencesPath, b, 0600); e != nil {
			updateStatus = "更新設定を保存できません"
		}
	}
}
func beginUpdateCheck() {
	if updateBusy {
		return
	}
	updateBusy = true
	updateStatus = "GitHubの最新版を確認中…"
	updatePrefs.LastCheck = time.Now()
	saveUpdatePreferences()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	updateCancel = cancel
	go func() {
		defer cancel()
		offer, e := fetchUpdate(ctx, updateHTTPClient(), appVersion())
		updateEvents <- updateEvent{offer: offer, err: e}
	}()
}
func requestUpdateInstall() {
	if updateBusy || pendingUpdate == nil {
		return
	}
	if soloMeasurement != nil {
		updateStatus = "時間指定計測を終了してから更新してください"
		invalidate(mainWindow)
		return
	}
	offer := *pendingUpdate
	message := fmt.Sprintf("v%s に更新しますか？\n\nキャプチャと計測を停止し、アプリを再起動します。\n設定・履歴・既存の翻訳CSVは保持します。\n\n", offer.Version)
	notes := []rune(offer.Notes)
	if len(notes) > 1200 {
		notes = notes[:1200]
	}
	message += string(notes)
	answer, _, _ := pMessageBox.Call(mainWindow, uintptr(unsafe.Pointer(u16(message))), uintptr(unsafe.Pointer(u16("MABI DPS JP の更新"))), 0x24|0x100)
	if answer != 6 {
		return
	}
	exe, e := os.Executable()
	if e != nil {
		alert(mainWindow, "アプリの保存先を確認できません")
		return
	}
	updateBusy = true
	updateStatus = "更新ZIPを取得・検証中…"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	updateCancel = cancel
	go func() {
		defer cancel()
		stage, e := stageUpdate(ctx, updateHTTPClient(), filepath.Dir(exe), offer, uint32(os.Getpid()))
		updateEvents <- updateEvent{stage: stage, err: e, downloading: true}
	}()
}
func pumpUpdateEvents() {
	select {
	case event := <-updateEvents:
		updateBusy = false
		if event.err != nil {
			updateStatus = "更新できません：" + event.err.Error()
			invalidate(mainWindow)
			return
		}
		if !event.downloading {
			pendingUpdate = event.offer
			if pendingUpdate == nil {
				updateStatus = "最新版です（v" + appVersion() + "）"
			} else {
				updateStatus = "v" + pendingUpdate.Version + " が利用できます。設定 → その他から更新できます。"
				openNativeSettings(settingsOther)
			}
			invalidate(mainWindow)
			return
		}
		planPath := filepath.Join(event.stage, "plan.json")
		b, e := readBoundedFile(planPath, 16<<10)
		var plan updatePlan
		if e == nil {
			e = json.Unmarshal(b, &plan)
		}
		if e == nil {
			plan.DataDir = filepath.Dir(historyDir)
			b, _ = json.Marshal(plan)
			e = os.WriteFile(planPath, b, 0600)
		}
		if e != nil {
			updateStatus = "更新の準備に失敗しました"
			invalidate(mainWindow)
			return
		}
		cmd := exec.Command(filepath.Join(event.stage, "MabiDPS-JP-update.exe"), "-update-helper")
		cmd.Dir = event.stage
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
		if e = cmd.Start(); e != nil {
			updateStatus = "更新用プログラムを起動できません"
			invalidate(mainWindow)
			return
		}
		_ = cmd.Process.Release()
		stopCapture()
		saveCompleted()
		updateExiting = true
		pDestroy.Call(mainWindow)
	default:
	}
}

// Runs from a copy of the old EXE under <installation>/.updates/pending-*/.
// The installation root is derived from that fixed layout, not a plan field.
func runUpdateHelper() {
	exe, e := os.Executable()
	if e != nil {
		return
	}
	stage := filepath.Dir(exe)
	updates := filepath.Dir(stage)
	root := filepath.Dir(updates)
	if filepath.Base(exe) != "MabiDPS-JP-update.exe" || filepath.Base(updates) != ".updates" || !strings.HasPrefix(filepath.Base(stage), "pending-") {
		return
	}
	if checkUpdatePath(root, stage) != nil {
		return
	}
	if e = rejectUpdateReparse(root, stage); e != nil {
		return
	}
	b, e := readBoundedFile(filepath.Join(stage, "plan.json"), 16<<10)
	if e != nil {
		return
	}
	var plan updatePlan
	if json.Unmarshal(b, &plan) != nil || plan.Parent == 0 || plan.Parent == uint32(os.Getpid()) {
		return
	}
	if !newerVersion(plan.Version, appVersion()) {
		return
	}
	parent, e := windows.OpenProcess(windows.SYNCHRONIZE, false, plan.Parent)
	if e == nil {
		result, err := windows.WaitForSingleObject(parent, 60000)
		windows.CloseHandle(parent)
		if err != nil || result != windows.WAIT_OBJECT_0 {
			return
		}
	} else if e != windows.ERROR_INVALID_PARAMETER {
		return
	}
	// Hold the same mutex as the application until all replacement is complete.
	instance, e := openAppInstance(applicationInstanceName)
	if e != nil {
		if instance != 0 {
			windows.CloseHandle(instance)
		}
		return
	}
	files, e := inspectUpdateZIP(filepath.Join(stage, "package.zip"), plan.Version, plan.Digest)
	if e == nil {
		for name := range files {
			if e = rejectUpdateReparse(root, filepath.Join(root, filepath.FromSlash(name))); e != nil {
				break
			}
		}
	}
	if e == nil {
		e = installUpdateFiles(root, stage, files)
	}
	status := "更新完了。旧ファイルはこのフォルダーのbackupに保存しています。"
	if e != nil {
		status = "更新失敗：" + e.Error()
	}
	_ = os.WriteFile(filepath.Join(stage, "result.txt"), []byte(status), 0600)
	windows.CloseHandle(instance)
	if e != nil {
		pMessageBox.Call(0, uintptr(unsafe.Pointer(u16(status+"\n元の版を再起動します。復元に失敗した場合は元の配布ZIPを展開してください。"))), uintptr(unsafe.Pointer(u16("MABI DPS JP"))), 0x10)
	}
	args := []string{}
	if plan.DataDir != "" {
		args = append(args, "-data-dir", plan.DataDir)
	}
	cmd := exec.Command(filepath.Join(root, "MabiDPS-JP.exe"), args...)
	cmd.Dir = root
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err == nil {
		_ = cmd.Process.Release()
	}
}
func rejectUpdateReparse(root, dest string) error {
	if e := checkUpdatePath(root, dest); e != nil {
		return e
	}
	current := dest
	for {
		ptr, e := windows.UTF16PtrFromString(current)
		if e != nil {
			return e
		}
		attrs, e := windows.GetFileAttributes(ptr)
		if e == nil && attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return fmt.Errorf("リンク先には更新できません")
		}
		if e != nil && e != windows.ERROR_FILE_NOT_FOUND && e != windows.ERROR_PATH_NOT_FOUND {
			return e
		}
		if strings.EqualFold(current, root) {
			break
		}
		next := filepath.Dir(current)
		if next == current {
			return fmt.Errorf("更新先が不正です")
		}
		current = next
	}
	return nil
}
