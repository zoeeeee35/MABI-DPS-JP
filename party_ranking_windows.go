//go:build native && windows

package main

import (
	"fmt"
	"strings"
	"unsafe"
)

type playerDrawItem struct {
	Type, ID, Item, Action, State uint32
	Window, DC                    uintptr
	Bounds                        rect
	Data                          uintptr
}

func drawPlayerRanking(l uintptr) bool {
	if l == 0 {
		return false
	}
	d := (*playerDrawItem)(nativeMessagePointer(l))
	if int(d.ID) != idPlayers {
		return false
	}
	r := d.Bounds
	fill(d.DC, r, 0x271B12)
	b := displayBattle()
	if b == nil || int(d.Item) >= len(b.Players) {
		return true
	}
	p := b.Players[d.Item]
	if d.State&1 != 0 {
		fill(d.DC, r, 0x443123)
	}
	fill(d.DC, rect{r.Left + 2, r.Top + 2, r.Left + 2 + int32(float64(r.Right-r.Left-4)*p.Share), r.Bottom - 2}, 0x55432A)
	class := mapped("class", p.ClassID)
	icon := "人"
	if strings.Contains(class, "氷") {
		icon = "氷"
	} else if strings.Contains(class, "剣") {
		icon = "剣"
	} else if strings.Contains(class, "弓") || strings.Contains(class, "クロスボウ") {
		icon = "弓"
	}
	draw(d.DC, fmt.Sprintf("%d  [%s] %s", d.Item+1, icon, p.Name), rect{r.Left + 10, r.Top + 3, r.Left + 320, r.Bottom}, 0xF2EAE2, overlayFont, 0x8020)
	draw(d.DC, fmt.Sprintf("DPS %s   累計 %s   %s", rowDPSText(p), rowDamageText(p), rowShareText(p)), rect{r.Left + 325, r.Top + 3, r.Right - 12, r.Bottom}, 0xB9DC81, overlayFont, 0x8022)
	return true
}

// Windows owns LPARAM during synchronous WM_DRAWITEM/WM_MEASUREITEM callbacks.
// Copy pointer bits from the ABI word without pointer arithmetic or retaining it.
func nativeMessagePointer(word uintptr) unsafe.Pointer {
	return *(*unsafe.Pointer)(unsafe.Pointer(&word))
}
