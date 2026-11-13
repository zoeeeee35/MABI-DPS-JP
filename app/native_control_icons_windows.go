//go:build native && windows

package main

import "math"

// Small original vector icons stay sharp at any font setting.
func nativeControlIcon(dc uintptr, kind string, x, y int32, color uint32) {
	pen, _, _ := gdi32.NewProc("CreatePen").Call(0, 1, uintptr(color))
	old, _, _ := pSelect.Call(dc, pen)
	defer func() { pSelect.Call(dc, old); pDelete.Call(pen) }()
	line := func(points ...point) {
		if len(points) == 0 {
			return
		}
		gdi32.NewProc("MoveToEx").Call(dc, signed(points[0].X+x), signed(points[0].Y+y), 0)
		for _, p := range points[1:] {
			gdi32.NewProc("LineTo").Call(dc, signed(p.X+x), signed(p.Y+y))
		}
	}
	switch kind {
	case "settings":
		teeth := []point{}
		for i := 0; i < 32; i++ {
			radius := 7.5
			if i%4 >= 2 {
				radius = 5.8
			}
			a := float64(i) * math.Pi / 16
			teeth = append(teeth, point{8 + int32(math.Round(math.Cos(a)*radius)), 8 + int32(math.Round(math.Sin(a)*radius))})
		}
		teeth = append(teeth, teeth[0])
		line(teeth...)
		hollow, _, _ := gdi32.NewProc("GetStockObject").Call(5)
		brush, _, _ := pSelect.Call(dc, hollow)
		gdi32.NewProc("Ellipse").Call(dc, signed(x+5), signed(y+5), signed(x+12), signed(y+12))
		pSelect.Call(dc, brush)
	case "layers":
		line(point{1, 5}, point{8, 1}, point{15, 5}, point{8, 9}, point{1, 5})
		line(point{1, 8}, point{8, 12}, point{15, 8})
		line(point{1, 11}, point{8, 15}, point{15, 11})
	case "click":
		line(point{3, 1}, point{3, 12}, point{6, 9}, point{9, 15}, point{12, 13}, point{9, 7}, point{14, 7}, point{3, 1})
	case "overlay":
		line(point{0, 8}, point{4, 4}, point{8, 2}, point{12, 4}, point{16, 8}, point{12, 12}, point{8, 14}, point{4, 12}, point{0, 8})
		line(point{7, 5}, point{10, 5}, point{11, 8}, point{10, 11}, point{7, 11}, point{5, 8}, point{7, 5})
	}
}
