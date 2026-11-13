//go:build native && windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"syscall"
	"unsafe"
)

var partyWindow uintptr
var partyIDs []string
var partyRoles = []EntityRole{RoleUnknown, RoleSelf, RoleParty, RoleOutside, RoleEnemy, RoleSummon}
var partyRoleNames = []string{"未識別", "自分", "PTメンバー", "検出プレイヤー・PT外", "敵・NPC", "召喚物"}
var partyScopes = []string{"party", "self", "members", "all"}

func partySession() *Session {
	if session != nil && session.Log.Source == jpProvisionalSource {
		return session
	}
	return nil
}
func partyText(id int) string {
	var b [256]uint16
	send(controls[id], 0x000d, 256, uintptr(unsafe.Pointer(&b[0])))
	return syscall.UTF16ToString(b[:])
}
func partyRefresh() {
	s := partySession()
	if s == nil {
		return
	}
	if s.Log.Party == nil {
		s.Log.Party = newPartyConfig()
	}
	c := s.Log.Party
	c.discover(s.Log)
	partyIDs = nil
	for id := range c.Entities {
		partyIDs = append(partyIDs, id)
	}
	sort.Strings(partyIDs)
	send(controls[801], 0x014b, 0, 0)
	for _, id := range partyIDs {
		e := c.Entities[id]
		label := string(e.Role)
		sendString(controls[801], 0x0143, fmt.Sprintf("%s / %s / %s", entityDisplayID(e), e.Name, label))
	}
	send(controls[801], 0x014e, 0, 0)
	partySelect()
}
func partySelect() {
	s := partySession()
	i := int(send(controls[801], 0x0147, 0, 0))
	if s == nil || i < 0 || i >= len(partyIDs) {
		return
	}
	e := s.Log.Party.Entities[partyIDs[i]]
	callText(controls[802], e.Name)
	callText(controls[803], e.Class)
	callText(controls[805], e.Owner)
	for n, r := range partyRoles {
		if r == e.Role {
			send(controls[804], 0x014e, uintptr(n), 0)
		}
	}
	targetMode := uintptr(1)
	if s.Log.Party.AllowUnknownTargets {
		targetMode = 0
	}
	send(controls[811], 0x014e, targetMode, 0)
	for n, v := range partyScopes {
		if v == s.Log.Party.Scope {
			send(controls[806], 0x014e, uintptr(n), 0)
		}
	}
}
func partyChild(class, text string, id int, x, y, w, h int32, style uintptr) {
	v := createWindow(class, text, 0, wsChild|wsVisible|style, x, y, w, h, partyWindow, uintptr(id))
	controls[id] = v
	send(v, 0x0030, normalFont, 1)
}
func openPartySettings() {
	if partySession() == nil {
		alert(mainWindow, "キャプチャを開始し、候補を受信してください。")
		return
	}
	if partyWindow != 0 {
		partyRefresh()
		show(partyWindow, true)
		return
	}
	_ = registerClass("MabiDPSParty", syscall.NewCallback(partyProc), 0)
	partyWindow = createWindow("MabiDPSParty", "検出プレイヤーと敵の設定（現在の取得セッション限定）", 0, 0x00ca0000, 200, 140, 900, 570, mainWindow, 0)
	partyChild("STATIC", "職業スキルから推定したプレイヤーを表示。PT所属は未判定。未識別は手動指定できます。", 820, 20, 15, 850, 24, 0)
	partyChild("COMBOBOX", "", 801, 20, 50, 845, 240, 0x0003|wsTabStop|0x00200000)
	for i, label := range []string{"キャラクター名", "職業（不明なら空欄）", "エンティティ分類", "召喚物の所有者ID（判明時のみ）"} {
		y := int32(100 + i*53)
		partyChild("STATIC", label, 821+i, 20, y, 270, 25, 0)
		id := 802 + i
		class := "EDIT"
		style := uintptr(0x00800000 | wsTabStop)
		if id == 804 {
			class = "COMBOBOX"
			style = 0x0003 | wsTabStop
		}
		partyChild(class, "", id, 300, y-3, 560, 150, style)
		if class == "EDIT" {
			move(controls[id], 300, y-3, 560, 29)
		}
	}
	for _, n := range partyRoleNames {
		sendString(controls[804], 0x0143, n)
	}
	partyChild("STATIC", "表示対象", 826, 20, 319, 270, 25, 0)
	partyChild("COMBOBOX", "", 806, 300, 315, 560, 150, 0x0003|wsTabStop)
	for _, n := range []string{"自分＋PTメンバー（手動指定）", "自分のみ", "PTメンバーのみ", "検出プレイヤー一覧（標準）"} {
		sendString(controls[806], 0x0143, n)
	}
	partyChild("COMBOBOX", "", 811, 300, 350, 560, 130, 0x0003|wsTabStop)
	sendString(controls[811], 0x0143, "対象未識別も含む（）")
	sendString(controls[811], 0x0143, "敵・NPC指定の対象のみ")
	send(controls[811], 0x014e, 0, 0)
	partyChild("BUTTON", "設定を適用", 807, 20, 389, 160, 33, wsTabStop)
	partyChild("BUTTON", "対応付けを保存", 808, 195, 389, 185, 33, wsTabStop)
	partyChild("BUTTON", "同一セッションへ読込", 809, 395, 389, 220, 33, wsTabStop)
	partyChild("BUTTON", "所属をすべて解除", 810, 630, 389, 230, 33, wsTabStop)
	partyChild("STATIC", "未識別の攻撃元・敵の攻撃・既知プレイヤーへの攻撃を除外。未識別対象への値は。\r\n同じIDでも、エリア移動後の再利用は保証できません。移動時は解除し、再指定してください。\r\n回復等の種別は未確定のため、表示は引き続きDPSです。", 827, 20, 425, 845, 85, 0)
	partyRefresh()
	show(partyWindow, true)
}
func partyCommand(id int) {
	s := partySession()
	if s == nil {
		return
	}
	c := s.Log.Party
	var err error
	switch id {
	case 807:
		i := int(send(controls[801], 0x0147, 0, 0))
		r := int(send(controls[804], 0x0147, 0, 0))
		scope := int(send(controls[806], 0x0147, 0, 0))
		if i < 0 || i >= len(partyIDs) || r < 0 || r >= len(partyRoles) {
			return
		}
		owner := partyText(805)
		for key, e := range c.Entities {
			if entityDisplayID(e) == owner {
				owner = key
				break
			}
		}
		err = c.set(PartyEntity{DisplayID: c.Entities[partyIDs[i]].DisplayID, ID: partyIDs[i], Name: partyText(802), Class: partyText(803), Role: partyRoles[r], Owner: owner, Evidence: "manual"})
		c.AllowUnknownTargets = send(controls[811], 0x0147, 0, 0) == 0
		if scope >= 0 && scope < len(partyScopes) {
			c.Scope = partyScopes[scope]
		}
	case 808, 809:
		path, e := dialogPath(partyWindow, id == 808)
		if e != nil || path == "" {
			return
		}
		if id == 808 {
			data, _ := json.MarshalIndent(c, "", "  ")
			err = os.WriteFile(path, data, 0600)
		} else {
			data, e := os.ReadFile(path)
			err = e
			var saved PartyConfig
			if err == nil {
				err = json.Unmarshal(data, &saved)
			}
			if err == nil && saved.Session != c.Session {
				err = fmt.Errorf("別の取得セッションです。IDの自動再利用は行いません")
			}
			if err == nil {
				for _, e := range saved.Entities {
					if _, ok := c.Entities[e.ID]; !ok {
						err = fmt.Errorf("現在の候補にないIDが含まれます")
						break
					}
				}
			}
			if err == nil {
				s.Log.Party = &saved
			}
		}
	case 810:
		for id, e := range c.Entities {
			e.Role = RoleUnknown
			e.Evidence = "manual"
			e.Owner = ""
			c.Entities[id] = e
		}
	}
	if err != nil {
		alert(partyWindow, err.Error())
		return
	}
	_ = s.recompute()
	lastRows = ""
	lastSkills = ""
	updateDisplay()
	partyRefresh()
}
func partyProc(w uintptr, msg uint32, a, b uintptr) uintptr {
	switch msg {
	case wmCommand:
		if int(a&0xffff) == 801 && int(a>>16) == 1 {
			partySelect()
		} else {
			partyCommand(int(a & 0xffff))
		}
		return 0
	case wmClose:
		pDestroy.Call(w)
		return 0
	case wmDestroy:
		partyWindow = 0
		return 0
	}
	v, _, _ := pDefWindow.Call(w, uintptr(msg), a, b)
	return v
}
