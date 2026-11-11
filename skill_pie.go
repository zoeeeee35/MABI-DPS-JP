package main

import (
	"math"
	"sort"
	"strings"
)

type SkillPieSlice struct {
	Key, Name         string
	Damage            int64
	Share, Start, End float64
}

func skillPieData(rows []SkillDamageRow) []SkillPieSlice {
	out := []SkillPieSlice{}
	var total int64
	for _, r := range rows {
		if r.Key == "all" || strings.HasPrefix(r.Key, "unknown-child:") || r.Damage <= 0 {
			continue
		}
		out = append(out, SkillPieSlice{Key: r.Key, Name: r.Name, Damage: r.Damage})
		total += r.Damage
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Damage == out[j].Damage {
			return out[i].Key < out[j].Key
		}
		return out[i].Damage > out[j].Damage
	})
	at := 0.0
	for i := range out {
		out[i].Share = float64(out[i].Damage) / float64(total)
		out[i].Start = at
		at += out[i].Share
		out[i].End = at
	}
	if len(out) > 0 {
		out[len(out)-1].End = 1
	}
	return out
}
func skillPieHit(slices []SkillPieSlice, x, y, cx, cy, radius float64) int {
	dx, dy := x-cx, y-cy
	if radius <= 0 || dx*dx+dy*dy > radius*radius {
		return -1
	}
	angle := math.Atan2(dy, dx) + math.Pi/2
	if angle < 0 {
		angle += 2 * math.Pi
	}
	fraction := angle / (2 * math.Pi)
	for i, s := range slices {
		if fraction >= s.Start && fraction < s.End {
			return i
		}
	}
	return -1
}
