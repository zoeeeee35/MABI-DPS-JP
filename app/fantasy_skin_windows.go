//go:build native && windows

package main

import (
	"math"
	"syscall"
	"unsafe"
)

type skinVertex struct {
	X, Y       int32
	R, G, B, A uint16
}

func skinVertexAt(x, y int32, color uint32) skinVertex {
	return skinVertex{x, y, uint16(color&255) * 257, uint16((color>>8)&255) * 257, uint16((color>>16)&255) * 257, 0}
}
func fantasyCard(dc uintptr, r rect, top, bottom uint32, radius int32) {
	saved, _, _ := gdi32.NewProc("SaveDC").Call(dc)
	// GDI clipping regions use device coordinates even when drawing is mapped.
	points := []point{{r.Left, r.Top}, {r.Right + 1, r.Bottom + 1}, {0, 0}, {radius, radius}}
	gdi32.NewProc("LPtoDP").Call(dc, uintptr(unsafe.Pointer(&points[0])), uintptr(len(points)))
	rx, ry := max(int32(1), points[3].X-points[2].X), max(int32(1), points[3].Y-points[2].Y)
	region, _, _ := gdi32.NewProc("CreateRoundRectRgn").Call(signed(points[0].X), signed(points[0].Y), signed(points[1].X), signed(points[1].Y), uintptr(rx), uintptr(ry))
	gdi32.NewProc("ExtSelectClipRgn").Call(dc, region, 1)
	vertices := []skinVertex{skinVertexAt(r.Left, r.Top, top), skinVertexAt(r.Right, r.Bottom, bottom)}
	mesh := [2]uint32{0, 1}
	ok, _, _ := sysGradientFill.Call(dc, uintptr(unsafe.Pointer(&vertices[0])), 2, uintptr(unsafe.Pointer(&mesh)), 1, 1)
	if ok == 0 {
		fill(dc, r, bottom)
	}
	gdi32.NewProc("RestoreDC").Call(dc, saved)
	pDelete.Call(region)
	pen, _, _ := gdi32.NewProc("CreatePen").Call(0, 1, 0x514031)
	oldPen, _, _ := pSelect.Call(dc, pen)
	hollow, _, _ := gdi32.NewProc("GetStockObject").Call(5)
	oldBrush, _, _ := pSelect.Call(dc, hollow)
	gdi32.NewProc("RoundRect").Call(dc, signed(r.Left), signed(r.Top), signed(r.Right), signed(r.Bottom), uintptr(radius), uintptr(radius))
	pSelect.Call(dc, oldBrush)
	pSelect.Call(dc, oldPen)
	pDelete.Call(pen)
}

var sysGradientFill = syscall.NewLazyDLL("msimg32.dll").NewProc("GradientFill")

func skinPolygon(dc uintptr, points []point, color uint32) {
	if len(points) < 3 {
		return
	}
	brush, _, _ := pCreateBrush.Call(uintptr(color))
	old, _, _ := pSelect.Call(dc, brush)
	pen, _, _ := gdi32.NewProc("CreatePen").Call(0, 1, uintptr(color))
	op, _, _ := pSelect.Call(dc, pen)
	gdi32.NewProc("Polygon").Call(dc, uintptr(unsafe.Pointer(&points[0])), uintptr(len(points)))
	pSelect.Call(dc, op)
	pSelect.Call(dc, old)
	pDelete.Call(pen)
	pDelete.Call(brush)
}
func skinIcon(dc uintptr, kind string, x, y, size int32, color uint32) {
	line := func(points ...point) {
		pen, _, _ := gdi32.NewProc("CreatePen").Call(0, 2, uintptr(color))
		old, _, _ := pSelect.Call(dc, pen)
		for i, p := range points {
			if i == 0 {
				gdi32.NewProc("MoveToEx").Call(dc, signed(p.X), signed(p.Y), 0)
			} else {
				gdi32.NewProc("LineTo").Call(dc, signed(p.X), signed(p.Y))
			}
		}
		pSelect.Call(dc, old)
		pDelete.Call(pen)
	}
	cx, cy := x+size/2, y+size/2
	switch kind {
	case "class", "mp":
		skinPolygon(dc, []point{{cx, y}, {x + size, cy}, {cx, y + size}, {x, cy}}, color)
		skinPolygon(dc, []point{{cx, y + 2}, {cx, y + size - 3}, {x + size - 3, cy}}, 0xE2F6F2)
	case "hp":
		points := []point{}
		for i := 0; i < 40; i++ {
			t := 2 * math.Pi * float64(i) / 40
			px := 16 * math.Pow(math.Sin(t), 3)
			py := 13*math.Cos(t) - 5*math.Cos(2*t) - 2*math.Cos(3*t) - math.Cos(4*t)
			points = append(points, point{cx + int32(px*float64(size)/34), cy - int32(py*float64(size)/34)})
		}
		skinPolygon(dc, points, color)
	case "bell":
		line(point{x + 2, y + size - 4}, point{x + 4, y + size - 7}, point{x + 4, y + 5}, point{cx, y + 2}, point{x + size - 4, y + 5}, point{x + size - 4, y + size - 7}, point{x + size - 2, y + size - 4}, point{x + 2, y + size - 4})
		line(point{cx - 2, y + size - 1}, point{cx + 2, y + size - 1})
	case "chart":
		for i := int32(0); i < 3; i++ {
			fill(dc, rect{x + i*size/3, y + size - (i+1)*size/3, x + i*size/3 + max(2, size/5), y + size}, color)
		}
	case "clock":
		points := []point{}
		for i := 0; i <= 24; i++ {
			a := float64(i) * 2 * math.Pi / 24
			points = append(points, point{cx + int32(float64(size/2)*math.Cos(a)), cy + int32(float64(size/2)*math.Sin(a))})
		}
		line(points...)
		line(point{cx, y + 3}, point{cx, cy}, point{x + size - 3, cy})
	case "sword", "combo":
		line(point{x, y + size}, point{x + size, y})
		line(point{x, y}, point{x + size, y + size})
		line(point{x, y + size - 5}, point{x + 5, y + size})
	case "reset":
		line(point{x + size - 2, y + size/2}, point{x + size - 4, y + size - 2}, point{x + 3, y + size - 1}, point{x, y + size/2}, point{x + 4, y + 2}, point{x + size - 3, y + 3})
		line(point{x + size - 3, y}, point{x + size - 3, y + 6}, point{x + size - 9, y + 6})
	case "file":
		line(point{x + 2, y}, point{x + size - 4, y}, point{x + size - 1, y + 4}, point{x + size - 1, y + size}, point{x + 2, y + size}, point{x + 2, y})
		line(point{x + 5, y + 6}, point{x + size - 4, y + 6})
		line(point{x + 5, y + 10}, point{x + size - 4, y + 10})
	default:
		skinPolygon(dc, []point{{cx, y}, {cx + 2, cy - 2}, {x + size, cy}, {cx + 2, cy + 2}, {cx, y + size}, {cx - 2, cy + 2}, {x, cy}, {cx - 2, cy - 2}}, color)
	}
}
func overlayFitText(dc uintptr, text string, r rect, color uint32, size, minSize int32) {
	for size > minSize {
		font := soloFont(size)
		old, _, _ := pSelect.Call(dc, font)
		width := nativeTextWidth(dc, text)
		pSelect.Call(dc, old)
		if width <= r.Right-r.Left-2 {
			break
		}
		size--
	}
	draw(dc, text, r, color, soloFont(size), 0x8021)
}
