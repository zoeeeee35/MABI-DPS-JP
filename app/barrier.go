package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type BarrierSettings struct {
	Enabled          bool   `json:"enabled"`
	LeadMinutes      int    `json:"leadMinutes"`
	AtStart          bool   `json:"atStart"`
	WindowsToast     bool   `json:"windowsToast"`
	Sound            bool   `json:"sound"`
	SoundPath        string `json:"soundPath,omitempty"`
	Volume           int    `json:"volume"`
	Overlay          bool   `json:"overlay"`
	AlwaysCountdown  bool   `json:"alwaysCountdown"`
	BannerSeconds    int    `json:"bannerSeconds"`
	KeepBeforeBanner bool   `json:"keepBeforeBanner,omitempty"`
}

const defaultBarrierSound = "sounds/Inspiration02-1(High).mp3"

func defaultBarrierSettings() BarrierSettings {
	return BarrierSettings{Enabled: true, AlwaysCountdown: true, LeadMinutes: 5, AtStart: true, WindowsToast: true, Sound: true, SoundPath: defaultBarrierSound, Volume: 50, Overlay: true, BannerSeconds: 8}
}

var barrierLeadChoices = []int{1, 3, 5, 10, 15, 30}

func normalizeBarrierSettings(c BarrierSettings) BarrierSettings {
	valid := false
	for _, m := range barrierLeadChoices {
		if m == c.LeadMinutes {
			valid = true
		}
	}
	if !valid {
		c.LeadMinutes = 5
	}
	c.Volume = max(0, min(100, c.Volume))
	if c.BannerSeconds < 1 || c.BannerSeconds > 30 {
		c.BannerSeconds = 8
	}
	return c
}
func readBarrierSettings(path string) (BarrierSettings, error) {
	c := defaultBarrierSettings()
	b, e := readBoundedFile(path, 64<<10)
	if os.IsNotExist(e) {
		return c, nil
	}
	if e != nil {
		return c, e
	}
	if len(b) > 64<<10 {
		return c, fmt.Errorf("通知設定ファイルが大きすぎます")
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return defaultBarrierSettings(), e
	}
	return normalizeBarrierSettings(c), nil
}
func writeBarrierSettings(path string, c BarrierSettings) error {
	c = normalizeBarrierSettings(c)
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	if e = os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	tmp := path + ".tmp"
	if e = os.WriteFile(tmp, b, 0600); e != nil {
		return e
	}
	if e = os.Rename(tmp, path); e != nil {
		if e = os.WriteFile(path, b, 0600); e != nil {
			return e
		}
		_ = os.Remove(tmp)
	}
	return nil
}

var barrierJST = time.FixedZone("Asia/Tokyo", 9*3600)

func nextBarrierTime(now time.Time) time.Time {
	return now.In(barrierJST).Truncate(time.Hour).Add(time.Hour)
}

const barrierStartedLabel = "結界開始"

func barrierCountdown(now time.Time, c BarrierSettings) (string, bool) {
	if !c.Enabled || !c.Overlay {
		return "", false
	}
	if now.In(barrierJST).Minute() < 3 {
		return barrierStartedLabel, true
	}
	remain := nextBarrierTime(now).Sub(now)
	if !c.AlwaysCountdown && remain > time.Duration(c.LeadMinutes)*time.Minute {
		return "", false
	}
	seconds := int((remain + time.Second - 1) / time.Second)
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60), true
}

type BarrierEvent struct {
	Kind        string
	Target      time.Time
	LeadMinutes int
}

func (e BarrierEvent) Text() string {
	if e.Kind == "start" {
		return "結界が始まりました。"
	}
	return fmt.Sprintf("結界開始%d分前です。\n出現予定時刻：%s", e.LeadMinutes, e.Target.In(barrierJST).Format("15:04"))
}

type BarrierScheduler struct {
	last time.Time
	key  string
	seen map[string]time.Time
}

func (s *BarrierScheduler) Tick(now time.Time, c BarrierSettings) []BarrierEvent {
	now = now.Round(0) // Scheduling follows wall-clock adjustments, not Go's monotonic clock.
	c = normalizeBarrierSettings(c)
	key := fmt.Sprintf("%t:%d:%t", c.Enabled, c.LeadMinutes, c.AtStart)
	if s.last.IsZero() || s.key != key || now.Before(s.last) || now.Sub(s.last) > 90*time.Second {
		s.last = now
		s.key = key
		return nil
	}
	previous := s.last
	s.last = now
	if !c.Enabled {
		return nil
	}
	if s.seen == nil {
		s.seen = map[string]time.Time{}
	}
	for k, t := range s.seen {
		if now.Sub(t) > 48*time.Hour {
			delete(s.seen, k)
		}
	}
	out := []BarrierEvent{}
	hour := now.In(barrierJST).Truncate(time.Hour)
	for _, target := range []time.Time{hour, hour.Add(time.Hour)} {
		for _, kind := range []string{"before", "start"} {
			due := target
			if kind == "before" {
				due = target.Add(-time.Duration(c.LeadMinutes) * time.Minute)
			} else if !c.AtStart {
				continue
			}
			id := fmt.Sprintf("%d:%s", target.Unix(), kind)
			if previous.Before(due) && !now.Before(due) && now.Sub(due) <= 30*time.Second {
				if _, done := s.seen[id]; !done {
					s.seen[id] = target
					out = append(out, BarrierEvent{kind, target, c.LeadMinutes})
				}
			}
		}
	}
	return out
}
func barrierBannerEnd(c BarrierSettings, event BarrierEvent, shownAt time.Time) time.Time {
	if event.Kind == "before" {
		if c.KeepBeforeBanner {
			return event.Target
		}
		return shownAt.Add(8 * time.Second)
	}
	return shownAt.Add(time.Duration(c.BannerSeconds) * time.Second)
}

func barrierFade(age time.Duration, seconds int) uint8 {
	return barrierFadeDuration(age, time.Duration(seconds)*time.Second)
}

func barrierFadeDuration(age, total time.Duration) uint8 {
	if age < 0 || age >= total {
		return 0
	}
	alpha := 1.0
	if age < 250*time.Millisecond {
		alpha = float64(age) / float64(250*time.Millisecond)
	}
	if total-age < 350*time.Millisecond {
		alpha = min(alpha, float64(total-age)/float64(350*time.Millisecond))
	}
	return uint8(max(0, min(245, 245*alpha)))
}
