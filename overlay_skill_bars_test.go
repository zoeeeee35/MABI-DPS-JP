package main

import "testing"

func TestOverlaySkillBarsExcludeTotalsAndChildren(t *testing.T) {
	rows := []SkillDamageRow{{Key: "all", Damage: 1000}, {Key: "small", Name: "小さい技", Damage: 100, Hits: 2}, {Key: "large", Name: "大きい技", Damage: 900, Hits: 3}, {Key: "unknown-child:one", Damage: 500}, {Key: "usage", Uses: 5}}
	bars := overlaySkillBars(rows)
	if len(bars) != 2 || bars[0].Name != "大きい技" || bars[0].Share != 0.9 || bars[0].Width != 1 || bars[1].Share != 0.1 || bars[1].Hits != 2 {
		t.Fatal(bars)
	}
	if rows[1].Key != "small" {
		t.Fatal("source rows reordered")
	}
}
func TestOverlayButtonsRemainBelowInsertedSkills(t *testing.T) {
	if overlayControlY(305) != -1 || overlayControlY(305+overlaySkillTableHeight-1) != -1 || overlayControlY(305+overlaySkillTableHeight) != 305 {
		t.Fatal("table overlaps controls")
	}
	y := overlayControlY(350 + overlaySkillTableHeight)
	if soloTimerHit(130, y) != 1 || soloTimerHit(270, y) != 4 {
		t.Fatal("timer hit mapping shifted")
	}
}
