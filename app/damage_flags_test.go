package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDamageFlagsIndependentCombinations(t *testing.T) {
	events := []Hit{}
	for mask := 0; mask < 16; mask++ {
		p := make([]byte, 53)
		if mask&1 != 0 {
			p[32] = 1
		}
		if mask&2 != 0 {
			p[33] |= 2
		}
		if mask&4 != 0 {
			p[33] |= 4
		}
		if mask&8 != 0 {
			p[40] = 32
		}
		f := decodeDamageFlags(p)
		events = append(events, Hit{PlayerID: "a", Damage: 10, Flags: f})
	}
	s := damageFlagStatistics(events, "a")
	if s.Total != 16 || s.Normal != 1 || s.Critical != 8 || s.Strong != 8 || s.Combo != 8 || s.Mismatch != 8 || len(s.Combinations) != 16 {
		t.Fatal(s)
	}
	if damageFlagStatistics(events, "b").Total != 0 {
		t.Fatal("player leak")
	}
	if decodeDamageFlags(make([]byte, 52)) != nil {
		t.Fatal("bad length accepted")
	}
}
func TestFlagMetadataDoesNotAlterDamageOrCritical(t *testing.T) {
	v, _ := newJPProvisionalInput(time.Now(), 5000)
	p := make([]byte, 53)
	p[32] = 1
	p[33] = 6
	p[40] = 32
	f := decodeDamageFlags(p)
	accepted, e := v.append(JPValue{At: v.Live.origin, ActorToken: "a", TargetToken: "b", EventToken: "one", Value: 123, Flags: f})
	if e != nil || !accepted {
		t.Fatal(e)
	}
	if _, e = v.append(JPValue{At: v.Live.origin, ActorToken: "a", TargetToken: "b", EventToken: "one", Value: 123, Flags: f}); e != nil {
		t.Fatal(e)
	}
	log := v.Live.Session.Log
	if len(log.Events) != 1 || log.Events[0].Damage != 123 || log.Events[0].Critical || log.Events[0].Flags == nil {
		t.Fatal(log.Events)
	}
	s := damageFlagStatistics(log.Events, "")
	if s.Critical != 1 || s.Strong != 1 || s.Combo != 1 || s.Total != 1 {
		t.Fatal(s)
	}
}

func TestAdditionalAttackRateFromStoredFlags(t *testing.T) {
	events := []Hit{
		{Damage: 777, Flags: &DamageFlags{B35: 0x08, Critical: true}},
		{Damage: 981, Flags: &DamageFlags{B35: 0x40}},
		{Damage: 1111, Flags: &DamageFlags{B35: 0x02}},
		{Damage: 1200},
	}
	s := damageFlagStatistics(events, "")
	if rate, ok := s.additionalRate(); !ok || rate != 0.5 || s.Additional != 1 || s.Critical != 1 || s.Normal != 2 {
		t.Fatalf("unexpected additional attack statistics: %+v, rate=%v", s, rate)
	}
	if _, ok := damageFlagStatistics(events[:1], "").additionalRate(); ok {
		t.Fatal("rate must be absent when no non-additional hits were observed")
	}
}
func TestFlagsCSVWarningAndMissingMetadata(t *testing.T) {
	p := make([]byte, 53)
	p[33] = 4
	p[35] = 99
	f := decodeDamageFlags(p)
	f.AttackerID = "548446"
	f.TargetID = "548943"
	var b bytes.Buffer
	events := []Hit{{Damage: 100, Flags: f}, {Damage: 200}}
	if e := exportDamageFlags(&b, events); e != nil {
		t.Fatal(e)
	}
	records, e := csv.NewReader(strings.NewReader(b.String())).ReadAll()
	if e != nil || len(records) != 2 || !strings.Contains(b.String(), "COMBO_CANDIDATES_DISAGREE") {
		t.Fatal(b.String(), e)
	}
	s := damageFlagStatistics(events, "")
	if s.Total != 2 || s.Observed != 1 || s.Normal != 0 {
		t.Fatal(s)
	}
}
func TestLocalFlagReplay(t *testing.T) {
	path := os.Getenv("MABI_FLAGS_PCAP")
	if path == "" {
		t.Skip("local")
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	s, e := replayPCAP(f, 16000)
	if e != nil {
		t.Fatal(e)
	}
	if s.provisionalLog == nil {
		t.Fatal("missing log")
	}
	n := 0
	for _, h := range s.provisionalLog.Events {
		if h.Flags == nil {
			t.Fatal("flag metadata lost")
		}
		if h.Critical {
			t.Fatal("provisional flag changed confirmed critical")
		}
		n++
	}
	if n == 0 {
		t.Fatal("no events")
	}
	t.Logf("flag metadata retained for %d damage events", n)
	own := []Hit{}
	for _, h := range s.provisionalLog.Events {
		if h.Flags.AttackerID == "548446" {
			own = append(own, h)
		}
	}
	stats := damageFlagStatistics(own, "")
	t.Logf("selected player flags: %+v", stats)
	if out := os.Getenv("MABI_FLAGS_REPORT"); out != "" {
		data, _ := json.MarshalIndent(stats, "", "  ")
		if err := os.WriteFile(out, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if out := os.Getenv("MABI_FLAGS_CSV"); out != "" {
		file, err := os.Create(out)
		if err != nil {
			t.Fatal(err)
		}
		err = exportDamageFlags(file, own)
		_ = file.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}
