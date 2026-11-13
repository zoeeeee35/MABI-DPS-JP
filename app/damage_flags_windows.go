//go:build native && windows

package main

import (
	"fmt"
	"os"
	"sort"
	"syscall"
	"unsafe"
)

func nativeFlagPanel(dc uintptr, b *Battle, left, right, y int32) int32 {
	var events []Hit
	if b != nil {
		events = b.DamageEvents
	}
	s := damageFlagStatistics(events, "")
	nbox(dc, rect{left, y, right, y + 346}, 0x261912)
	ntxt(dc, "ダメージ判定候補（・表示区間）", rect{left + 16, y + 10, right - 220, y + 35}, 0xDFC9B0, 14)
	nbutton(dc, rect{right - 210, y + 8, right - 12, y + 40}, "個別フラグCSVを保存", "flags-csv", false, 0)
	labels := []string{"通常のみ※", "クリティカル候補", "強打候補", "連打候補", "追加攻撃候補"}
	counts := []int{s.Normal, s.Critical, s.Strong, s.Combo}
	for i, label := range labels {
		yy := y + 48 + int32(i*27)
		text := fmt.Sprintf("%d件 / —", s.Additional)
		if i < 4 {
			rate := 0.0
			if s.Total > 0 {
				rate = 100 * float64(counts[i]) / float64(s.Total)
			}
			text = fmt.Sprintf("%d件 / %.1f%%", counts[i], rate)
		} else if rate, ok := s.additionalRate(); ok {
			text = fmt.Sprintf("%d件 / %.1f%%", s.Additional, 100*rate)
		}
		ntxt(dc, label, rect{left + 20, yy, left + 240, yy + 26}, 0xD2BFA8, 12)
		ntxt(dc, text, rect{left + 250, yy, right - 16, yy + 26}, 0xB9A184, 12)
	}
	ntxt(dc, fmt.Sprintf("母数 %d件 / フラグ取得 %d件 / 未取得 %d件 / 連打候補不一致 %d件", s.Total, s.Observed, s.Total-s.Observed, s.Mismatch), rect{left + 20, y + 190, right - 16, y + 214}, 0xADD397, 11)
	ntxt(dc, "※通常は全候補OFF。追加率（暫定）はフラグ取得済みの追加攻撃以外のヒットが分母です。", rect{left + 20, y + 216, right - 16, y + 240}, 0xA99AC8, 11)
	keys := []string{}
	for k := range s.Combinations {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	ntxt(dc, "組合せ C=クリ候補 S=強打 M=連打 A=連打照合（0=OFF / 1=ON）", rect{left + 20, y + 244, right - 16, y + 264}, 0xA98B6D, 10)
	for row := 0; row < 4; row++ {
		line := ""
		for j := row * 4; j < min(len(keys), row*4+4); j++ {
			line += fmt.Sprintf("%s=%d  ", keys[j], s.Combinations[keys[j]])
		}
		ntxt(dc, line, rect{left + 20, y + 266 + int32(row*18), right - 16, y + 284 + int32(row*18)}, 0xA98B6D, 10)
	}
	return 362
}
func nativeSaveFlags() {
	b := displayBattle()
	if b == nil {
		alert(mainWindow, "表示中のダメージがありません")
		return
	}
	buffer := make([]uint16, 32768)
	filter := []uint16{}
	for _, s := range []string{"CSV (*.csv)", "*.csv", ""} {
		v, _ := syscall.UTF16FromString(s)
		filter = append(filter, v...)
	}
	ofn := openFileName{Owner: mainWindow, Filter: &filter[0], File: &buffer[0], MaxFile: uint32(len(buffer)), FilterIndex: 1, Flags: 0x80000 | 2 | 8, DefaultExt: u16("csv")}
	ofn.Size = uint32(unsafe.Sizeof(ofn))
	ok, _, _ := pGetSave.Call(uintptr(unsafe.Pointer(&ofn)))
	if ok == 0 {
		return
	}
	f, e := os.OpenFile(syscall.UTF16ToString(buffer), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if e == nil {
		e = exportDamageFlags(f, b.DamageEvents)
		closeErr := f.Close()
		if e == nil {
			e = closeErr
		}
	}
	if e != nil {
		alert(mainWindow, e.Error())
	}
}
