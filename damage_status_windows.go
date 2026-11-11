//go:build native && windows

package main

import "fmt"

func rowDPSText(p Row) string {
	if p.DamageStatus == "unobserved" {
		return "未取得"
	}
	return formatNum(p.DPS)
}
func rowDamageText(p Row) string {
	if p.DamageStatus == "unobserved" {
		return "未取得"
	}
	return formatNum(float64(p.Damage))
}
func rowShareText(p Row) string {
	if p.DamageStatus == "unobserved" {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", p.Share*100)
}
