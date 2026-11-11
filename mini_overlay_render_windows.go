//go:build native && windows

package main

import (
	"fmt"
	"time"
	"unsafe"
)

const miniNoticeWidth, miniNoticeHeight int32 = 190, 34
const barrierBadgeBlue = 0xD47524

type miniOverlayContent struct {
	title, value, elapsed string
	notice                bool
}

type miniOverlayWindow struct {
	handle        uintptr
	shown, adjust bool
	bounds        rect
	prefs         miniCardPreferences
	content       miniOverlayContent
}

func (w *miniOverlayWindow) hide() {
	if w.shown {
		show(w.handle, false)
		w.shown = false
	}
}

func (w *miniOverlayWindow) present(bounds rect, prefs miniCardPreferences, content miniOverlayContent) {
	if w.handle == 0 {
		w.handle = createWindow("MabiDPSOverlay", "MABI DPS JP ミニ表示", exTopmost|exToolwindow|exLayered|exNoActivate|exTransparent, 0x80000000, bounds.Left, bounds.Top, 340, 210, 0, 0)
		if w.handle == 0 {
			return
		}
	}
	adjust := gameOverlayAdjust
	adjustChanged := w.adjust != adjust
	if adjustChanged {
		style := uintptr(exTopmost | exToolwindow | exLayered | exNoActivate)
		if !adjust {
			style |= exTransparent
		}
		pSetLong.Call(w.handle, exIndex(), style)
		w.adjust = adjust
	}
	width, height := bounds.Right-bounds.Left, bounds.Bottom-bounds.Top
	resized := width != w.bounds.Right-w.bounds.Left || height != w.bounds.Bottom-w.bounds.Top
	if adjustChanged || resized || w.prefs != prefs || w.content != content {
		if !renderMiniOverlay(w.handle, bounds, prefs, content) {
			w.hide()
			return
		}
		w.bounds, w.prefs, w.content = bounds, prefs, content
	} else if w.bounds != bounds {
		// Move the layered bitmap without rasterizing its contents again.
		ok, _, _ := pSetWindowPos.Call(w.handle, 0, signed(bounds.Left), signed(bounds.Top), 0, 0, 0x0015)
		if ok == 0 {
			return
		}
		w.bounds = bounds
	}
	if !w.shown {
		pSetWindowPos.Call(w.handle, ^uintptr(0), 0, 0, 0, 0, 0x0013|0x0040)
		w.shown = true
	}
}

func miniOverlayProc(window uintptr, msg uint32, w, l uintptr) uintptr {
	switch msg {
	case wmMouseActivate:
		return 3
	case 0x0084:
		if !gameOverlayAdjust {
			return ^uintptr(0)
		}
		return 1
	case wmLButtonDown:
		if gameOverlayAdjust {
			dragGameOverlay(window)
		}
		return 0
	case wmClose:
		if window == miniNotice.handle {
			miniNotice.hide()
		} else if window == miniBarrier.handle {
			miniBarrier.hide()
		} else {
			miniDPS.hide()
		}
		return 0
	}
	result, _, _ := pDefWindow.Call(window, uintptr(msg), w, l)
	return result
}

func miniStartTime(seconds int) string { return fmt.Sprintf("開始まで %d秒", seconds) }

func miniBarrierTime(remaining time.Duration) string {
	seconds := max(0, int((remaining+time.Second-1)/time.Second))
	if seconds >= 60 {
		return fmt.Sprintf("%d分前", (seconds+59)/60)
	}
	return fmt.Sprintf("%d秒前", seconds)
}

func renderMiniOverlay(window uintptr, bounds rect, prefs miniCardPreferences, content miniOverlayContent) bool {
	width, height := bounds.Right-bounds.Left, bounds.Bottom-bounds.Top
	if width < 1 || height < 1 {
		return false
	}
	dc, _, _ := gdi32.NewProc("CreateCompatibleDC").Call(0)
	if dc == 0 {
		return false
	}
	defer gdi32.NewProc("DeleteDC").Call(dc)
	header := struct {
		Size          uint32
		Width, Height int32
		Planes, Bits  uint16
		Rest          [6]uint32
	}{Size: 40, Width: width, Height: -height, Planes: 1, Bits: 32}
	var bits unsafe.Pointer
	bitmap, _, _ := gdi32.NewProc("CreateDIBSection").Call(dc, uintptr(unsafe.Pointer(&header)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bitmap == 0 {
		return false
	}
	defer pDelete.Call(bitmap)
	previous, _, _ := pSelect.Call(dc, bitmap)
	defer pSelect.Call(dc, previous)
	fill(dc, rect{0, 0, width, height}, 0)

	isBarrier := content.elapsed == "" && !content.notice
	baseWidth, baseHeight := miniWidth, miniDPSHeight
	if isBarrier {
		baseWidth, baseHeight = miniBarrierWidth, miniBarrierHeight
	} else if content.notice {
		baseWidth, baseHeight = miniNoticeWidth, miniNoticeHeight
	}
	saved, _, _ := gdi32.NewProc("SaveDC").Call(dc)
	if saved == 0 {
		return false
	}
	gdi32.NewProc("SetMapMode").Call(dc, 8)
	gdi32.NewProc("SetWindowExtEx").Call(dc, uintptr(baseWidth), uintptr(baseHeight), 0)
	gdi32.NewProc("SetViewportExtEx").Call(dc, uintptr(width), uintptr(height), 0)
	paintMiniTextMask(dc, prefs, content)
	gdi32.NewProc("RestoreDC").Call(dc, saved)
	gdi32.NewProc("GdiFlush").Call()

	pixels := unsafe.Slice((*uint32)(bits), int(width*height))
	alpha := uint32(miniOpacities[prefs.BackgroundOpacity] * 255 / 100)
	if gameOverlayAdjust {
		alpha = max(alpha, 64)
	}
	background := miniBackgroundColors[prefs.BackgroundColor]
	accent := miniTextColors[prefs.TextColor]
	radius := max(int32(3), width*9/miniWidth)
	if content.notice {
		radius = height / 2
	}
	for y := int32(0); y < height; y++ {
		for x := int32(0); x < width; x++ {
			i := y*width + x
			if isBarrier {
				pixels[i] = miniBarrierPixel(x, y, width, height, background, accent, alpha, (pixels[i]>>8)&255)
				continue
			}
			cornerX := max(radius-x, x-(width-radius-1), 0)
			cornerY := max(radius-y, y-(height-radius-1), 0)
			if cornerX*cornerX+cornerY*cornerY > radius*radius {
				pixels[i] = 0
				continue
			}
			text := uint32(0xEEEAE3)
			if content.notice || (x*miniWidth >= width*110 && y*baseHeight < height*42) {
				text = accent
			}
			pixels[i] = miniBlendPixel(background, text, alpha, (pixels[i]>>8)&255)
		}
	}

	position, dimensions, origin := point{bounds.Left, bounds.Top}, point{width, height}, point{}
	blend := uint32(0x01ff0000) // AC_SRC_OVER, full constant alpha, AC_SRC_ALPHA.
	ok, _, _ := user32.NewProc("UpdateLayeredWindow").Call(window, 0, uintptr(unsafe.Pointer(&position)), uintptr(unsafe.Pointer(&dimensions)), dc, uintptr(unsafe.Pointer(&origin)), 0, uintptr(unsafe.Pointer(&blend)), 2)
	return ok != 0
}

// Sample the badge and crystal at subpixel positions before compositing the
// text mask. This keeps the silhouette smooth at arbitrary card scales.
func miniBarrierPixel(x, y, width, height int32, background, accent, alpha, coverage uint32) uint32 {
	const samples = 4
	var circle, crystal, caption uint32
	for sy := 0; sy < samples; sy++ {
		py := (float64(y) + (float64(sy)+0.5)/samples) * float64(miniBarrierHeight) / float64(height)
		for sx := 0; sx < samples; sx++ {
			px := (float64(x) + (float64(sx)+0.5)/samples) * float64(miniBarrierWidth) / float64(width)
			dx, dy := px-36, py-16
			if dx*dx+dy*dy <= 15.5*15.5 {
				circle++
				if isBarrierCrystal(dx, dy) {
					crystal++
				}
			}
			dx, dy = max(17-px, px-55, 0), py-42
			if dx*dx+dy*dy <= 10*10 {
				caption++
			}
		}
	}
	const count = samples * samples
	if circle > 0 {
		return miniBlendPixel(barrierBadgeBlue, 0xFFFFFF, circle*alpha/count, crystal*255/count)
	}
	return miniBlendPixel(background, accent, caption*alpha/count, coverage*caption/count)
}

var barrierCrystalShards = [][][2]float64{
	{{-2, -10}, {2, -11}, {5, -3}, {2, 6}, {-2, 5}, {-4, -3}},
	{{-9, -3}, {-7, -4}, {-5, -2}, {-4, 4}, {-7, 2}, {-10, 0}},
	{{7, -3}, {9, -3}, {10, 0}, {6, 4}, {4, 4}, {6, -1}},
}

func drawBarrierBadge(dc uintptr, x, y, size int32) {
	// The regular overlay scales its horizontal and vertical axes independently.
	// Place the badge in device pixels and use one diameter for both axes.
	bounds := [...]point{{x, y}, {x + size, y + size}}
	if ok, _, _ := gdi32.NewProc("LPtoDP").Call(dc, uintptr(unsafe.Pointer(&bounds[0])), uintptr(len(bounds))); ok == 0 {
		return
	}
	width, height := bounds[1].X-bounds[0].X, bounds[1].Y-bounds[0].Y
	size = min(width, height)
	if size <= 0 {
		return
	}
	x = bounds[0].X + (width-size)/2
	y = bounds[0].Y + (height-size)/2
	saved, _, _ := gdi32.NewProc("SaveDC").Call(dc)
	if saved == 0 {
		return
	}
	defer gdi32.NewProc("RestoreDC").Call(dc, saved)
	gdi32.NewProc("SetMapMode").Call(dc, 1) // MM_TEXT
	gdi32.NewProc("SetWindowOrgEx").Call(dc, 0, 0, 0)
	gdi32.NewProc("SetViewportOrgEx").Call(dc, 0, 0, 0)

	brush, _, _ := pCreateBrush.Call(barrierBadgeBlue)
	oldBrush, _, _ := pSelect.Call(dc, brush)
	noPen, _, _ := gdi32.NewProc("GetStockObject").Call(8)
	oldPen, _, _ := pSelect.Call(dc, noPen)
	gdi32.NewProc("Ellipse").Call(dc, signed(x), signed(y), signed(x+size), signed(y+size))
	pSelect.Call(dc, oldPen)
	pSelect.Call(dc, oldBrush)
	pDelete.Call(brush)

	cx, cy := x+size/2, y+size/2
	scale := float64(size) / 31
	for _, shard := range barrierCrystalShards {
		points := make([]point, len(shard))
		for i, p := range shard {
			points[i] = point{cx + int32(p[0]*scale), cy + int32(p[1]*scale)}
		}
		skinPolygon(dc, points, 0xFFFFFF)
	}
	hollow, _, _ := gdi32.NewProc("GetStockObject").Call(5)
	whitePen, _, _ := gdi32.NewProc("GetStockObject").Call(6)
	oldBrush, _, _ = pSelect.Call(dc, hollow)
	oldPen, _, _ = pSelect.Call(dc, whitePen)
	dx := int32(9 * scale)
	gdi32.NewProc("Ellipse").Call(dc, signed(cx-dx), signed(cy+int32(5*scale)), signed(cx+dx+1), signed(cy+int32(10*scale)))
	pSelect.Call(dc, oldPen)
	pSelect.Call(dc, oldBrush)
}

func isBarrierCrystal(x, y float64) bool {
	// Thin elliptical ring beneath the three shards.
	ring := x*x/81 + (y-7)*(y-7)/6.25
	if ring >= 0.57 && ring <= 1 {
		return true
	}
	for _, shard := range barrierCrystalShards {
		inside := true
		for i, a := range shard {
			b := shard[(i+1)%len(shard)]
			if (b[0]-a[0])*(y-a[1])-(b[1]-a[1])*(x-a[0]) < 0 {
				inside = false
				break
			}
		}
		if inside {
			return true
		}
	}
	return false
}

// GDI supplies grayscale text coverage. Composite it over the adjustable
// background in premultiplied BGRA for UpdateLayeredWindow.
func miniBlendPixel(background, text, bgAlpha, coverage uint32) uint32 {
	back := (bgAlpha*(255-coverage) + 127) / 255
	channel := func(shift uint) uint32 {
		return (((text>>shift)&255)*coverage + ((background>>shift)&255)*back + 127) / 255
	}
	return (coverage+back)<<24 | channel(0)<<16 | channel(8)<<8 | channel(16)
}

func paintMiniTextMask(dc uintptr, prefs miniCardPreferences, content miniOverlayContent) {
	const white = 0xFFFFFF
	text := func(value string, r rect, size int32, align uintptr) {
		flags := uintptr(0x8024) | align
		face := u16(miniFonts[prefs.Font])
		font, _, _ := pFont.Call(signed(-size), 0, 0, 0, 700, 0, 0, 0, 128, 0, 0, 4, 0, uintptr(unsafe.Pointer(face)))
		if font == 0 {
			return
		}
		draw(dc, value, r, white, font, flags)
		pDelete.Call(font)
	}
	if content.notice {
		skinIcon(dc, "bell", 17, 10, 14, white)
		text(content.value, rect{34, 0, 172, miniNoticeHeight}, 15, 1)
		return
	}
	if content.elapsed == "" {
		text(content.value, rect{7, 32, 65, 52}, 14, 1)
		return
	}
	skinIcon(dc, "sword", 12, 15, 14, white)
	skinIcon(dc, "clock", 12, 52, 14, white)
	text(content.title, rect{36, 0, 95, 42}, 14, 0)
	size := int32(27)
	if len(content.value) > 10 {
		size = 21
	}
	text(content.value, rect{96, 0, 239, 42}, size, 2)
	fill(dc, rect{12, 42, 239, 43}, 0x606060)
	text("戦闘時間", rect{36, 43, 110, 80}, 13, 0)
	text(content.elapsed, rect{110, 43, 239, 80}, 16, 2)
}
