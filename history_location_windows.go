//go:build native && windows

package main

import (
	"syscall"
	"unsafe"
)

var historyNoteWindow, historyNoteEdit uintptr
var historyNotePath string
var historyNoteBrush uintptr

func openHistoryLocation(index int) {
	if index < 0 || index >= len(history) {
		return
	}
	if historyNoteWindow != 0 {
		pDestroy.Call(historyNoteWindow)
	}
	h := history[index]
	historyNotePath = h.Path
	_ = registerClass("MabiDPSHistoryNote", syscall.NewCallback(historyLocationProc), 0)
	historyNoteWindow = createWindow("MabiDPSHistoryNote", "履歴の場所メモ（手動）", 0, 0x00ca0000, 240, 200, 480, 210, mainWindow, 0)
	label := createWindow("STATIC", "場所は未解析です。手動のメモとして保存します。", 0, wsChild|wsVisible, 16, 12, 440, 25, historyNoteWindow, 0)
	send(label, 0x30, nf(14), 1)
	historyNoteEdit = createWindow("EDIT", h.Archive.LocationNote, 0x200, wsChild|wsVisible|wsTabStop|0x80, 16, 46, 430, 32, historyNoteWindow, 10000)
	send(historyNoteEdit, 0x30, nf(15), 1)
	send(historyNoteEdit, 0xc5, 80, 0)
	for i, text := range []string{"保存", "キャンセル"} {
		w := createWindow("BUTTON", text, 0, wsChild|wsVisible|wsTabStop, 220+int32(i*116), 100, 110, 32, historyNoteWindow, uintptr(10001+i))
		send(w, 0x30, nf(14), 1)
	}
	show(historyNoteWindow, true)
	user32.NewProc("SetFocus").Call(historyNoteEdit)
}
func historyLocationProc(w uintptr, msg uint32, a, b uintptr) uintptr {
	switch msg {
	case wmCommand:
		switch int(a & 0xffff) {
		case 10001:
			var text [256]uint16
			send(historyNoteEdit, 0xd, 256, uintptr(unsafe.Pointer(&text[0])))
			if e := updateHistoryLocation(historyNotePath, syscall.UTF16ToString(text[:])); e != nil {
				alert(w, e.Error())
				return 0
			}
			currentPath := ""
			if historyIndex >= 0 && historyIndex < len(history) {
				currentPath = history[historyIndex].Path
			}
			refreshHistory()
			if currentPath != "" {
				for i, h := range history {
					if h.Path == currentPath {
						historyIndex = i
						break
					}
				}
			}
			pDestroy.Call(w)
			updateDisplay()
		case 10002:
			pDestroy.Call(w)
		}
		return 0
	case 0x0133, 0x0138:
		if historyNoteBrush == 0 {
			historyNoteBrush, _, _ = pCreateBrush.Call(0x251A14)
		}
		gdi32.NewProc("SetTextColor").Call(a, 0xE9E2D7)
		gdi32.NewProc("SetBkColor").Call(a, 0x251A14)
		return historyNoteBrush
	case 0x0014:
		var r rect
		pGetClient.Call(w, uintptr(unsafe.Pointer(&r)))
		fill(a, r, 0x251A14)
		return 1
	case wmClose:
		pDestroy.Call(w)
		return 0
	case wmDestroy:
		historyNoteWindow = 0
		historyNoteEdit = 0
		return 0
	}
	r, _, _ := pDefWindow.Call(w, uintptr(msg), a, b)
	return r
}
