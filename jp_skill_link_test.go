package main

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"unicode/utf16"
)

func linkSkill(actor, target uint64, effect uint32, name string) []byte {
	u := utf16.Encode([]rune(name))
	p := make([]byte, 12+len(u)*2+29)
	binary.LittleEndian.PutUint64(p, actor)
	binary.LittleEndian.PutUint32(p[8:], uint32(len(u)*2))
	for i, c := range u {
		binary.LittleEndian.PutUint16(p[12+i*2:], c)
	}
	tail := p[12+len(u)*2:]
	binary.LittleEndian.PutUint64(tail[17:], target)
	binary.LittleEndian.PutUint32(tail[25:], effect)
	return jpFrame(0x1d4f8, p, false)
}
func linkHit(actor, target uint64, effect uint32) ([]byte, []byte) {
	p := make([]byte, 53)
	binary.LittleEndian.PutUint64(p, actor)
	binary.LittleEndian.PutUint64(p[8:], target)
	binary.LittleEndian.PutUint64(p[16:], 949)
	copy(p[32:], []byte{1, 2, 3, 4, 5, 6, 7, 8})
	m := make([]byte, 35)
	copy(m, p[:16])
	binary.LittleEndian.PutUint32(m[16:], effect)
	copy(m[24:], p[32:40])
	return jpFrame(0x5243, p, false), jpFrame(0x4ff7, m, false)
}
func TestJPSkillLink949AndLateAmbiguity(t *testing.T) {
	now := time.Now()
	d := jp9Decoder{}
	a, m := linkHit(1, 2, 123)
	got := d.feed(a, false, now)
	if len(got) != 1 || got[0].Link.Paired {
		t.Fatal(got)
	}
	if len(d.feed(m, false, now)) != 0 || !got[0].Link.Paired {
		t.Fatal("double damage/missing join")
	}
	d.feed(linkSkill(1, 2, 123, "IceMage_One"), false, now)
	l := CombatLog{Events: []Hit{{Damage: 949, Link: got[0].Link}}}
	applySkillLinks(&l)
	if l.Events[0].SkillID != "jp-skill:IceMage_One" {
		t.Fatal(l.Events)
	}
	d.feed(linkSkill(1, 2, 123, "IceMage_Two"), false, now)
	applySkillLinks(&l)
	if len(l.Events[0].Link.Names) != 2 || l.Events[0].Damage != 949 {
		t.Fatal(l.Events)
	}
	separate := jp9Decoder{}
	separate.feed(linkSkill(1, 2, 123, "IceMage_Three"), false, now)
	applySkillLinks(&l)
	if len(l.Events[0].Link.Names) != 2 {
		t.Fatal("cross connection dictionary")
	}
}
func TestJPSkillLinkRejectWrongFieldsAndNonAdjacent(t *testing.T) {
	for _, kind := range []string{"actor", "target", "field", "length", "nonadjacent"} {
		t.Run(kind, func(t *testing.T) {
			d := jp9Decoder{}
			a, m := linkHit(1, 2, 123)
			got := d.feed(a, false, time.Now())
			switch kind {
			case "actor":
				m[9]++
			case "target":
				m[17]++
			case "field":
				m[33]++
			case "length":
				m = jpFrame(0x4ff7, m[9:43], false)
			case "nonadjacent":
				d.feed(jpFrame(99, nil, false), false, time.Now())
			}
			for _, b := range m {
				d.feed([]byte{b}, false, time.Now())
			}
			if got[0].Link.Paired {
				t.Fatal("unsafe join")
			}
		})
	}
}
func TestLinkedSkillRowsKeepGroupsAndUnknownAndSort(t *testing.T) {
	b := &Battle{Start: 0, End: 100, DamageEvents: []Hit{{PlayerID: "a", Damage: 949, Link: &JPSkillLink{Paired: true, Effect: 1, Names: []string{"IceMage_A"}}}, {PlayerID: "a", Damage: 50, Link: &JPSkillLink{Paired: true, Effect: 2, Names: []string{"Melee_1", "Melee_2"}}}, {PlayerID: "a", Damage: 10}}}
	rows := linkedSkillRows(b, CombatLog{}, "a")
	var sum int64
	for _, r := range rows {
		sum += r.Damage
	}
	if len(rows) != 3 || sum != 1009 {
		t.Fatal(rows)
	}
	sortLinkedSkills(rows, 3, true)
	if rows[0].Damage != 949 {
		t.Fatal(rows)
	}
	sortLinkedSkills(rows, 6, false)
	if rows[0].Min != 10 {
		t.Fatal(rows)
	}
}
func TestLocalSkillLinkReplayRegression(t *testing.T) {
	root := os.Getenv("MABI_LINK_ROOT")
	if root == "" {
		t.Skip("local captures")
	}
	report := map[string]any{}
	files := []string{}
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e == nil && !d.IsDir() && filepath.Ext(path) == ".pcapng" {
			files = append(files, path)
		}
		return nil
	})
	for _, path := range files {
		f, e := os.Open(path)
		if e != nil {
			t.Fatal(e)
		}
		stats, e := replayPCAP(f, 16000)
		f.Close()
		if e != nil {
			t.Fatal(path, e)
		}
		if stats.provisionalLog == nil {
			continue
		}
		l := *stats.provisionalLog
		l.Party = l.Detected
		data, _ := json.Marshal(l)
		var before CombatLog
		_ = json.Unmarshal(data, &before)
		for i := range before.Events {
			before.Events[i].Link = nil
			before.Events[i].SkillID = "jp5243:unidentified"
		}
		until := int64(0)
		if len(l.Events) > 0 {
			until = l.Events[len(l.Events)-1].At + 5000
		}
		afterResult, err := analyze(Request{Log: l, Until: until, Idle: 5000})
		if err != nil {
			t.Fatal(err)
		}
		beforeResult, err := analyze(Request{Log: before, Until: until, Idle: 5000})
		if err != nil {
			t.Fatal(err)
		}
		type totals struct {
			Damage   int64
			Duration float64
			Player   map[string]Row
		}
		roll := func(r Result) []totals {
			out := []totals{}
			for _, b := range r.Battles {
				p := map[string]Row{}
				for _, x := range b.Players {
					x.Skills = nil
					p[x.ID] = x
				}
				out = append(out, totals{b.Damage, b.Duration, p})
			}
			return out
		}
		if !reflect.DeepEqual(roll(beforeResult), roll(afterResult)) {
			t.Fatal("DPS changed", path)
		}
		var total int64
		for i := range afterResult.Battles {
			b := &afterResult.Battles[i]
			rows := linkedSkillRows(b, l, "")
			var sum int64
			for _, r := range rows {
				sum += r.Damage
			}
			if sum != b.Damage {
				t.Fatal("classified sum changed", sum, b.Damage)
			}
			total += sum
		}
		rel, _ := filepath.Rel(root, path)
		report[rel] = map[string]any{"stats": skillLinkStats(l.Events), "totalDamage": total, "dpsUnchanged": true}
		t.Log(rel, skillLinkStats(l.Events), total)
	}
	if len(report) == 0 {
		t.Fatal("no capture")
	}
	if out := os.Getenv("MABI_LINK_REPORT"); out != "" {
		data, _ := json.MarshalIndent(report, "", "  ")
		_ = os.WriteFile(out, data, 0600)
	}
}
