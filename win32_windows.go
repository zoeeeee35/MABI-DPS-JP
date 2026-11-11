//go:build native && windows

package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

var user32 = syscall.NewLazyDLL("user32.dll")
var gdi32 = syscall.NewLazyDLL("gdi32.dll")
var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var comdlg32 = syscall.NewLazyDLL("comdlg32.dll")
var pCreateWindow = user32.NewProc("CreateWindowExW")
var pDefWindow = user32.NewProc("DefWindowProcW")
var pSendMessage = user32.NewProc("SendMessageW")
var pSetText = user32.NewProc("SetWindowTextW")
var pGetText = user32.NewProc("GetWindowTextW")
var pShowWindow = user32.NewProc("ShowWindow")
var pInvalidate = user32.NewProc("InvalidateRect")
var pSetWindowPos = user32.NewProc("SetWindowPos")
var pSetLong = user32.NewProc("SetWindowLongPtrW")
var pGetLong = user32.NewProc("GetWindowLongPtrW")
var pLayer = user32.NewProc("SetLayeredWindowAttributes")
var pGetClient = user32.NewProc("GetClientRect")
var pFill = user32.NewProc("FillRect")
var pDrawText = user32.NewProc("DrawTextW")
var pBeginPaint = user32.NewProc("BeginPaint")
var pEndPaint = user32.NewProc("EndPaint")
var pCreateBrush = gdi32.NewProc("CreateSolidBrush")
var pDelete = gdi32.NewProc("DeleteObject")
var pSelect = gdi32.NewProc("SelectObject")
var pSetBkMode = gdi32.NewProc("SetBkMode")
var pSetTextColor = gdi32.NewProc("SetTextColor")
var pFont = gdi32.NewProc("CreateFontW")
var pSetTimer = user32.NewProc("SetTimer")
var pMessageBox = user32.NewProc("MessageBoxW")
var pGetOpen = comdlg32.NewProc("GetOpenFileNameW")
var pGetSave = comdlg32.NewProc("GetSaveFileNameW")
var pGetMessage = user32.NewProc("GetMessageW")
var pTranslate = user32.NewProc("TranslateMessage")
var pDispatch = user32.NewProc("DispatchMessageW")
var pDestroy = user32.NewProc("DestroyWindow")
var pPostQuit = user32.NewProc("PostQuitMessage")
var pRegisterClass = user32.NewProc("RegisterClassExW")
var pLoadCursor = user32.NewProc("LoadCursorW")
var pGetModule = kernel32.NewProc("GetModuleHandleW")
var pReleaseCapture = user32.NewProc("ReleaseCapture")
var pSetForeground = user32.NewProc("SetForegroundWindow")
var pRegisterHotKey = user32.NewProc("RegisterHotKey")
var pUnregisterHotKey = user32.NewProc("UnregisterHotKey")
var pMetrics = user32.NewProc("GetSystemMetrics")
var pMove = user32.NewProc("MoveWindow")
var pIsWindow = user32.NewProc("IsWindow")

const (
	wmDestroy       = 0x0002
	wmSize          = 0x0005
	wmPaint         = 0x000f
	wmClose         = 0x0010
	wmCommand       = 0x0111
	wmTimer         = 0x0113
	wmHotkey        = 0x0312
	wmLButtonDown   = 0x0201
	wmErase         = 0x0014
	wmMouseActivate = 0x0021
	wsVisible       = 0x10000000
	wsChild         = 0x40000000
	wsTabStop       = 0x00010000
	exTopmost       = 0x00000008
	exTransparent   = 0x00000020
	exToolwindow    = 0x00000080
	exLayered       = 0x00080000
	exNoActivate    = 0x08000000
	lbReset         = 0x0184
	lbAdd           = 0x0180
	lbSetSel        = 0x0186
	lbGetSel        = 0x0188
)

type point struct{ X, Y int32 }
type rect struct{ Left, Top, Right, Bottom int32 }
type message struct {
	Window         uintptr
	ID             uint32
	WParam, LParam uintptr
	Time           uint32
	Point          point
	Private        uint32
}
type paintStruct struct {
	DC                 uintptr
	Erase              int32
	Rect               rect
	Restore, IncUpdate int32
	Reserved           [32]byte
}
type windowClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}
type openFileName struct {
	Size                      uint32
	Owner, Instance           uintptr
	Filter, CustomFilter      *uint16
	MaxCustom, FilterIndex    uint32
	File                      *uint16
	MaxFile                   uint32
	FileTitle                 *uint16
	MaxFileTitle              uint32
	InitialDir, Title         *uint16
	Flags                     uint32
	FileOffset, FileExtension uint16
	DefaultExt                *uint16
	CustomData, Hook          uintptr
	Template                  *uint16
	Reserved                  uintptr
	Reserved2, FlagsEx        uint32
}

func u16(s string) *uint16 {
	p, e := syscall.UTF16PtrFromString(s)
	if e != nil {
		p, _ = syscall.UTF16PtrFromString("不正な文字列")
	}
	return p
}
func signed(v int32) uintptr { return uintptr(int64(v)) }
func exIndex() uintptr       { return ^uintptr(19) } // GWL_EXSTYLE = -20
func callText(h uintptr, s string) {
	p := u16(s)
	pSetText.Call(h, uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
}
func send(h uintptr, m uint32, w, l uintptr) uintptr {
	v, _, _ := pSendMessage.Call(h, uintptr(m), w, l)
	return v
}
func addList(h uintptr, s string) {
	p := u16(s)
	send(h, lbAdd, 0, uintptr(unsafe.Pointer(p)))
	runtime.KeepAlive(p)
}
func invalidate(h uintptr) { pInvalidate.Call(h, 0, 0) }
func show(h uintptr, yes bool) {
	v := uintptr(0)
	if yes {
		v = 4
	}
	pShowWindow.Call(h, v)
}
func fill(dc uintptr, r rect, color uint32) {
	brush, _, _ := pCreateBrush.Call(uintptr(color))
	pFill.Call(dc, uintptr(unsafe.Pointer(&r)), brush)
	pDelete.Call(brush)
}
func draw(dc uintptr, s string, r rect, color uint32, font uintptr, flags uintptr) {
	old, _, _ := pSelect.Call(dc, font)
	pSetBkMode.Call(dc, 1)
	pSetTextColor.Call(dc, uintptr(color))
	p := u16(s)
	pDrawText.Call(dc, uintptr(unsafe.Pointer(p)), ^uintptr(0), uintptr(unsafe.Pointer(&r)), flags)
	pSelect.Call(dc, old)
	runtime.KeepAlive(p)
}
func newFont(height int32, bold bool) uintptr {
	weight := uintptr(400)
	if bold {
		weight = 700
	}
	p := u16("Yu Gothic UI")
	h, _, _ := pFont.Call(signed(-height), 0, 0, 0, weight, 0, 0, 0, 128, 0, 0, 5, 0, uintptr(unsafe.Pointer(p)))
	return h
}
func createWindow(class, title string, ex, style uintptr, x, y, w, h int32, parent, id uintptr) uintptr {
	c, t := u16(class), u16(title)
	inst, _, _ := pGetModule.Call(0)
	win, _, _ := pCreateWindow.Call(ex, uintptr(unsafe.Pointer(c)), uintptr(unsafe.Pointer(t)), style, signed(x), signed(y), signed(w), signed(h), parent, id, inst, 0)
	runtime.KeepAlive(c)
	runtime.KeepAlive(t)
	return win
}
func move(h uintptr, x, y, w, height int32) {
	pMove.Call(h, signed(x), signed(y), signed(w), signed(height), 1)
}
func registerClass(name string, proc uintptr, brush uintptr) error {
	inst, _, _ := pGetModule.Call(0)
	cursor, _, _ := pLoadCursor.Call(0, 32512)
	p := u16(name)
	wc := windowClass{Style: 0x0003, Proc: proc, Instance: inst, Cursor: cursor, Background: brush, Name: p}
	wc.Icon, _, _ = user32.NewProc("LoadImageW").Call(inst, 1, 1, 32, 32, 0x8000)
	wc.SmallIcon, _, _ = user32.NewProc("LoadImageW").Call(inst, 1, 1, 16, 16, 0x8000)
	wc.Size = uint32(unsafe.Sizeof(wc))
	a, _, e := pRegisterClass.Call(uintptr(unsafe.Pointer(&wc)))
	if a == 0 {
		return fmt.Errorf("画面を登録できません: %v", e)
	}
	return nil
}
func dialogPath(owner uintptr, save bool) (string, error) {
	buffer := make([]uint16, 32768)
	filter, _ := syscall.UTF16FromString("戦闘データ (*.json)\x00*.json\x00\x00")
	// UTF16FromString rejects embedded NUL; build the Win32 double-NUL filter explicitly.
	filter = []uint16{}
	for _, part := range []string{"戦闘データ (*.json)", "*.json", ""} {
		p, _ := syscall.UTF16FromString(part)
		filter = append(filter, p...)
	}
	ofn := openFileName{Owner: owner, Filter: &filter[0], File: &buffer[0], MaxFile: uint32(len(buffer)), FilterIndex: 1, Flags: 0x00080000 | 0x00001000 | 0x00000008, DefaultExt: u16("json")}
	ofn.Size = uint32(unsafe.Sizeof(ofn))
	proc := pGetOpen
	if save {
		proc = pGetSave
		ofn.Flags = 0x00080000 | 0x00000002 | 0x00000008
	}
	v, _, _ := proc.Call(uintptr(unsafe.Pointer(&ofn)))
	if v == 0 {
		code, _, _ := comdlg32.NewProc("CommDlgExtendedError").Call()
		if code != 0 {
			return "", fmt.Errorf("ファイル選択に失敗しました (%x)", code)
		}
		return "", nil
	}
	return syscall.UTF16ToString(buffer), nil
}
func alert(owner uintptr, text string) {
	pMessageBox.Call(owner, uintptr(unsafe.Pointer(u16(text))), uintptr(unsafe.Pointer(u16("MABI DPS JP"))), 0x10)
}

func dialogPCAPPath(owner uintptr) (string, error) {
	buffer := make([]uint16, 32768)
	filter := []uint16{}
	for _, s := range []string{"通信記録 (*.pcapng;*.pcap)", "*.pcapng;*.pcap", ""} {
		v, _ := syscall.UTF16FromString(s)
		filter = append(filter, v...)
	}
	ofn := openFileName{Owner: owner, Filter: &filter[0], File: &buffer[0], MaxFile: uint32(len(buffer)), FilterIndex: 1, Flags: 0x00080000 | 0x00001000 | 0x00000008, DefaultExt: u16("pcapng")}
	ofn.Size = uint32(unsafe.Sizeof(ofn))
	v, _, _ := pGetOpen.Call(uintptr(unsafe.Pointer(&ofn)))
	if v == 0 {
		return "", nil
	}
	return syscall.UTF16ToString(buffer), nil
}
