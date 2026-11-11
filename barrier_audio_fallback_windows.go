//go:build native && windows

package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

var barrierPlayMu, barrierWPFMu sync.Mutex
var barrierAudioCancel context.CancelFunc
var barrierAudioProcess *os.Process
var barrierAudioGeneration uint64

func stopBarrierAudio() {
	barrierWPFMu.Lock()
	barrierAudioGeneration++
	if barrierAudioProcess != nil {
		_ = barrierAudioProcess.Kill()
		barrierAudioProcess = nil
	}
	if barrierAudioCancel != nil {
		barrierAudioCancel()
		barrierAudioCancel = nil
	}
	barrierWPFMu.Unlock()
	stopBarrierMCI()
}
func playBarrierAudio(c BarrierSettings) error {
	barrierPlayMu.Lock()
	defer barrierPlayMu.Unlock()
	stopBarrierAudio()
	barrierWPFMu.Lock()
	generation := barrierAudioGeneration
	barrierWPFMu.Unlock()
	if e := playBarrierMCI(c); e == nil {
		return nil
	}
	stopBarrierMCI()
	return playBarrierWPF(c, generation)
}
func playBarrierWPF(c BarrierSettings, generation uint64) error {
	absolute, e := localBarrierSound(c.SoundPath)
	if e != nil {
		return e
	}
	pathData := base64.StdEncoding.EncodeToString([]byte(absolute))
	volume := float64(max(0, min(100, c.Volume))) / 100
	script := `$ErrorActionPreference='Stop';$ProgressPreference='SilentlyContinue';Add-Type -AssemblyName PresentationCore;$player=New-Object System.Windows.Media.MediaPlayer;$script:opened=$false;$script:failed=$false;$script:ended=$false;$player.add_MediaOpened({$script:opened=$true});$player.add_MediaFailed({$script:failed=$true});$player.add_MediaEnded({$script:ended=$true});$player.Volume=` + fmt.Sprintf("%.2f", volume) + `;$path=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('` + pathData + `'));$player.Open([Uri]$path);$deadline=[DateTime]::UtcNow.AddSeconds(8);function Pump { $frame=New-Object System.Windows.Threading.DispatcherFrame;[void][System.Windows.Threading.Dispatcher]::CurrentDispatcher.BeginInvoke([Action]{$frame.Continue=$false},[System.Windows.Threading.DispatcherPriority]::Background);[System.Windows.Threading.Dispatcher]::PushFrame($frame);Start-Sleep -Milliseconds 25 };while(-not $script:opened -and -not $script:failed -and [DateTime]::UtcNow -lt $deadline){Pump};if(-not $script:opened -or $script:failed){$player.Close();exit 2};$player.Play();[Console]::Out.WriteLine('READY');[Console]::Out.Flush();while(-not $script:ended -and -not $script:failed){Pump};$player.Close();if($script:failed){exit 2}`
	ctx, cancel := context.WithTimeout(barrierCtx, 10*time.Minute)
	barrierWPFMu.Lock()
	if generation != barrierAudioGeneration {
		barrierWPFMu.Unlock()
		cancel()
		return context.Canceled
	}
	barrierAudioCancel = cancel
	barrierWPFMu.Unlock()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-STA", "-NoLogo", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodedPowerShell(script))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	stdout, e := cmd.StdoutPipe()
	if e != nil {
		cancel()
		return e
	}
	if e = cmd.Start(); e != nil {
		cancel()
		return fmt.Errorf("Windows音声再生を起動できません")
	}
	barrierWPFMu.Lock()
	if ctx.Err() != nil {
		_ = cmd.Process.Kill()
	} else {
		barrierAudioProcess = cmd.Process
	}
	barrierWPFMu.Unlock()
	ready := make(chan error, 1)
	go func() {
		opened := false
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if scanner.Text() == "READY" && !opened {
				opened = true
				ready <- nil
			}
		}
		err := cmd.Wait()
		if !opened {
			ready <- fmt.Errorf("Windows音声機能でファイルを開けません。形式・コーデックを確認してください")
		} else if err != nil && ctx.Err() == nil {
			barrierReport("通知音の再生処理でエラーが発生しました")
		}
		cancel()
		barrierWPFMu.Lock()
		if generation == barrierAudioGeneration {
			barrierAudioCancel = nil
			barrierAudioProcess = nil
		}
		barrierWPFMu.Unlock()
	}()
	select {
	case err := <-ready:
		return err
	case <-time.After(10 * time.Second):
		cancel()
		return fmt.Errorf("通知音の読込が制限時間を超えました")
	case <-ctx.Done():
		return ctx.Err()
	}
}
