package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"time"
)

// Experimental metadata only. Never updates Hit.Critical or damage arithmetic.
type DamageFlags struct {
	Timestamp                         time.Time `json:"timestamp"`
	AttackerID                        string    `json:"attackerId"`
	TargetID                          string    `json:"targetId"`
	B32, B33, B35, B40                byte
	Critical, Strong, Combo, ComboAlt bool
}

func decodeDamageFlags(p []byte) *DamageFlags {
	if len(p) != 53 {
		return nil
	}
	return &DamageFlags{B32: p[32], B33: p[33], B35: p[35], B40: p[40], Critical: p[32]&1 != 0, Strong: p[33]&2 != 0, Combo: p[33]&4 != 0, ComboAlt: p[40]&32 != 0}
}

// Read the stored byte so older JP9 histories can also be classified.
func (f *DamageFlags) additionalAttack() bool { return f.B35&0x08 != 0 }

type FlagStatistics struct {
	Total, Observed, Normal, Critical, Strong, Combo, Additional, Mismatch int
	Combinations                                                           map[string]int
}

func (s FlagStatistics) additionalRate() (float64, bool) {
	baseHits := s.Observed - s.Additional
	if baseHits <= 0 {
		return 0, false
	}
	return float64(s.Additional) / float64(baseHits), true
}

func damageFlagStatistics(events []Hit, player string) FlagStatistics {
	s := FlagStatistics{Combinations: map[string]int{}}
	for _, e := range events {
		if e.Damage <= 0 || (player != "" && e.PlayerID != player) {
			continue
		}
		s.Total++
		f := e.Flags
		if f == nil {
			continue
		}
		s.Observed++
		if f.Critical {
			s.Critical++
		}
		if f.Strong {
			s.Strong++
		}
		if f.Combo {
			s.Combo++
		}
		if f.Combo != f.ComboAlt {
			s.Mismatch++
		}
		additional := f.additionalAttack()
		if additional {
			s.Additional++
		}
		if !additional && !f.Critical && !f.Strong && !f.Combo && !f.ComboAlt {
			s.Normal++
		}
		s.Combinations[fmt.Sprintf("C%d/S%d/M%d/A%d", flagInt(f.Critical), flagInt(f.Strong), flagInt(f.Combo), flagInt(f.ComboAlt))]++
	}
	return s
}
func flagInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func exportDamageFlags(w io.Writer, events []Hit) error {
	c := csv.NewWriter(w)
	if err := c.Write([]string{"Timestamp", "Attacker ID", "Target ID", "Damage", "payload[32]", "payload[33]", "payload[35]", "payload[40]", "Critical Candidate", "Strong Candidate", "Combo Candidate", "Combo Alt Candidate", "Warning", "Additional Attack Candidate"}); err != nil {
		return err
	}
	for _, e := range events {
		f := e.Flags
		if f == nil {
			continue
		}
		warning := ""
		if f.Combo != f.ComboAlt {
			warning = "COMBO_CANDIDATES_DISAGREE"
		}
		if err := c.Write([]string{f.Timestamp.UTC().Format(time.RFC3339Nano), f.AttackerID, f.TargetID, fmt.Sprint(e.Damage), fmt.Sprintf("0x%02X", f.B32), fmt.Sprintf("0x%02X", f.B33), fmt.Sprintf("0x%02X", f.B35), fmt.Sprintf("0x%02X", f.B40), fmt.Sprint(f.Critical), fmt.Sprint(f.Strong), fmt.Sprint(f.Combo), fmt.Sprint(f.ComboAlt), warning, fmt.Sprint(f.additionalAttack())}); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}
