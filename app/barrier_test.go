package main

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func barrierTime(s string) time.Time {
	v, e := time.Parse(time.RFC3339, s)
	if e != nil {
		panic(e)
	}
	return v
}
func TestBarrierHourlyScheduleAndDedup(t *testing.T) {
	c := defaultBarrierSettings()
	c.Enabled = true
	s := BarrierScheduler{}
	now := barrierTime("2026-10-27T14:54:59+09:00")
	if len(s.Tick(now, c)) != 0 {
		t.Fatal("startup backlog")
	}
	e := s.Tick(now.Add(time.Second), c)
	if len(e) != 1 || e[0].Kind != "before" || e[0].Target.Hour() != 15 {
		t.Fatal(e)
	}
	if len(s.Tick(now.Add(2*time.Second), c)) != 0 {
		t.Fatal("duplicate")
	}
	s.Tick(barrierTime("2026-10-27T14:59:59+09:00"), c)
	e = s.Tick(barrierTime("2026-10-27T15:00:00+09:00"), c)
	if len(e) != 1 || e[0].Kind != "start" {
		t.Fatal(e)
	}
	s.Tick(now, c)
	if len(s.Tick(now.Add(time.Second), c)) != 0 {
		t.Fatal("clock rollback duplicate")
	}
}
func TestBarrierSleepSettingsAndCountdown(t *testing.T) {
	c := defaultBarrierSettings()
	c.Enabled = true
	s := BarrierScheduler{}
	now := barrierTime("2026-10-27T14:54:00+09:00")
	s.Tick(now, c)
	if len(s.Tick(now.Add(20*time.Minute), c)) != 0 {
		t.Fatal("resume backlog")
	}
	c.LeadMinutes = 1
	if len(s.Tick(now.Add(21*time.Minute), c)) != 0 {
		t.Fatal("settings backlog")
	}
	text, visible := barrierCountdown(barrierTime("2026-10-27T14:55:28+09:00"), defaultEnabledBarrier())
	if !visible || text != "04:32" {
		t.Fatal(text, visible)
	}
	c.Enabled = false
	if _, visible := barrierCountdown(now, c); visible {
		t.Fatal("disabled")
	}
	c = defaultEnabledBarrier()
	c.AlwaysCountdown = false
	if _, visible := barrierCountdown(now, c); visible {
		t.Fatal("outside lead")
	}
	c.AlwaysCountdown = true
	if _, visible := barrierCountdown(now, c); !visible {
		t.Fatal("always")
	}
	if nextBarrierTime(barrierTime("2026-10-27T15:00:00+09:00")).Hour() != 16 {
		t.Fatal("next hour")
	}
}
func defaultEnabledBarrier() BarrierSettings {
	c := defaultBarrierSettings()
	c.Enabled = true
	return c
}
func TestBarrierSettingsRoundTripAndFade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "barrier.json")
	c := defaultEnabledBarrier()
	c.SoundPath = "C:/通知 音/sample.mp3"
	c.Volume = 37
	if e := writeBarrierSettings(path, c); e != nil {
		t.Fatal(e)
	}
	v, e := readBarrierSettings(path)
	if e != nil || !reflect.DeepEqual(c, v) {
		t.Fatal(v, e)
	}
	if barrierFade(0, 8) != 0 || barrierFade(time.Second, 8) != 245 || barrierFade(8*time.Second, 8) != 0 || barrierFade(7900*time.Millisecond, 8) >= 245 {
		t.Fatal("fade")
	}
}

func TestBarrierSchedulerUsesWallClock(t *testing.T) {
	s := BarrierScheduler{}
	now := time.Now()
	s.Tick(now, defaultEnabledBarrier())
	if s.last != s.last.Round(0) {
		t.Fatal("monotonic clock retained")
	}
}

func TestBarrierEveryConfiguredLead(t *testing.T) {
	target := barrierTime("2026-10-27T15:00:00+09:00")
	for _, lead := range barrierLeadChoices {
		c := defaultEnabledBarrier()
		c.LeadMinutes = lead
		s := BarrierScheduler{}
		due := target.Add(-time.Duration(lead) * time.Minute)
		s.Tick(due.Add(-time.Second), c)
		events := s.Tick(due, c)
		if len(events) != 1 || events[0].Kind != "before" || events[0].LeadMinutes != lead {
			t.Fatal(lead, events)
		}
	}
}

func TestBarrierDefaultsOnAndSavedOffPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "barrier.json")
	c, err := readBarrierSettings(path)
	if err != nil || !c.Enabled || !c.AlwaysCountdown {
		t.Fatal(c, err)
	}
	c.Enabled = false
	c.AlwaysCountdown = false
	if err = writeBarrierSettings(path, c); err != nil {
		t.Fatal(err)
	}
	c, err = readBarrierSettings(path)
	if err != nil || c.Enabled || c.AlwaysCountdown {
		t.Fatal("saved preference overwritten", c, err)
	}
}

func TestBarrierCountdownStartedWindow(t *testing.T) {
	c := defaultEnabledBarrier()
	c.AlwaysCountdown = false
	for _, stamp := range []string{"2026-11-09T12:00:00+09:00", "2026-11-09T12:02:59+09:00"} {
		if label, visible := barrierCountdown(barrierTime(stamp), c); !visible || label != barrierStartedLabel {
			t.Errorf("%s: %q, visible=%t", stamp, label, visible)
		}
	}
	if _, visible := barrierCountdown(barrierTime("2026-11-09T12:03:00+09:00"), c); visible {
		t.Fatal("start display continued past 03:00")
	}
}

func TestBarrierBannerDurations(t *testing.T) {
	shown := time.Date(2026, 11, 11, 0, 55, 0, 0, time.UTC)
	before := BarrierEvent{Kind: "before", Target: shown.Add(5 * time.Minute)}
	start := BarrierEvent{Kind: "start", Target: before.Target}
	c := defaultBarrierSettings()
	c.BannerSeconds = 17
	if got := barrierBannerEnd(c, before, shown); !got.Equal(shown.Add(8 * time.Second)) {
		t.Fatalf("short before ends %v", got)
	}
	c.KeepBeforeBanner = true
	if got := barrierBannerEnd(c, before, shown); !got.Equal(before.Target) {
		t.Fatalf("persistent before ends %v", got)
	}
	if got := barrierBannerEnd(c, start, start.Target); !got.Equal(start.Target.Add(17 * time.Second)) {
		t.Fatalf("start ends %v", got)
	}
}
