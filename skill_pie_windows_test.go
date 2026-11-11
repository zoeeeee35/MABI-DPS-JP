//go:build native && windows

package main

import (
	"strings"
	"testing"
)

func TestNativePieTooltipWrapPreservesFullLongName(t *testing.T) {
	dc, _, _ := gdi32.NewProc("CreateCompatibleDC").Call(0)
	if dc == 0 {
		t.Fatal("memory DC unavailable")
	}
	defer gdi32.NewProc("DeleteDC").Call(dc)
	old, _, _ := pSelect.Call(dc, nf(14))
	defer pSelect.Call(dc, old)
	name := strings.Repeat("Priest_Long_Internal_Skill_Name_01", 4) + " / とても長い日本語のスキル名（スキル群）"
	lines := nativeWrapTooltip(dc, name, 180)
	if len(lines) < 3 || strings.Join(lines, "") != name {
		t.Fatal("name lost", lines)
	}
	for _, line := range lines {
		if nativeTextWidth(dc, line) > 180 {
			t.Fatal("line overflow", line)
		}
	}
}
