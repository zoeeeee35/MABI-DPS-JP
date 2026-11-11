//go:build native && windows

package main

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"net"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Uses only PROCESS_QUERY_LIMITED_INFORMATION for an executable basename.
// Does not request PROCESS_VM_READ, write memory, inject, or inspect arguments.
func gameConnections(profile CaptureProfile) (map[string]bool, error) {
	flows := map[string]bool{}
	allowed := map[uint32]bool{}
	checked := map[uint32]bool{}
	var firstError error
	for _, family := range []uint32{2, 23} {
		var size uint32
		proc := windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedTcpTable")
		code, _, _ := proc.Call(0, uintptr(unsafe.Pointer(&size)), 0, uintptr(family), 5, 0)
		if code != 122 && code != 0 {
			if firstError == nil {
				firstError = fmt.Errorf("TCP接続一覧を取得できません (%d)", code)
			}
			continue
		}
		if size < 4 || size > 16<<20 {
			continue
		}
		buf := make([]byte, size)
		code, _, _ = proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0, uintptr(family), 5, 0)
		if code != 0 {
			if firstError == nil {
				firstError = fmt.Errorf("TCP接続一覧を取得できません (%d)", code)
			}
			continue
		}
		count := int(binary.LittleEndian.Uint32(buf[:4]))
		rowSize := 24
		if family == 23 {
			rowSize = 56
		}
		if count > (len(buf)-4)/rowSize {
			return flows, fmt.Errorf("TCP接続一覧のサイズが不正です")
		}
		for i := 0; i < count; i++ {
			r := buf[4+i*rowSize : 4+(i+1)*rowSize]
			var pid, state, lp, rp uint32
			var local, remote net.IP
			if family == 2 {
				state = binary.LittleEndian.Uint32(r)
				local = net.IP(r[4:8])
				lp = binary.LittleEndian.Uint32(r[8:12])
				remote = net.IP(r[12:16])
				rp = binary.LittleEndian.Uint32(r[16:20])
				pid = binary.LittleEndian.Uint32(r[20:24])
			} else {
				local = net.IP(r[:16])
				lp = binary.LittleEndian.Uint32(r[20:24])
				remote = net.IP(r[24:40])
				rp = binary.LittleEndian.Uint32(r[44:48])
				state = binary.LittleEndian.Uint32(r[48:52])
				pid = binary.LittleEndian.Uint32(r[52:56])
			}
			if state != 5 {
				continue
			}
			if !checked[pid] {
				checked[pid] = true
				h, e := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
				if e == nil {
					var path [32768]uint16
					n := uint32(len(path))
					e = windows.QueryFullProcessImageName(h, 0, &path[0], &n)
					windows.CloseHandle(h)
					if e == nil {
						name := filepath.Base(windows.UTF16ToString(path[:n]))
						for _, want := range profile.ProcessNames {
							if strings.EqualFold(name, want) {
								allowed[pid] = true
							}
						}
					}
				}
			}
			if allowed[pid] {
				flows[flowKey(remote.String(), bits.ReverseBytes16(uint16(rp)), local.String(), bits.ReverseBytes16(uint16(lp)))] = true
			}
		}
	}
	if len(flows) > 0 {
		return flows, nil
	}
	return flows, firstError
}

// UDP endpoints expose only the local bind, not a remote connection. Counts
// therefore remain candidates; no UDP payload is passed to the wire decoder.
func gameUDPEndpoints(profile CaptureProfile) (map[string]bool, error) {
	endpoints := map[string]bool{}
	type binding struct {
		key, portKey string
		pid          uint32
	}
	rows := []binding{}
	owners := map[string]map[uint32]bool{}
	for _, family := range []uint32{2, 23} {
		proc := windows.NewLazySystemDLL("iphlpapi.dll").NewProc("GetExtendedUdpTable")
		var size uint32
		code, _, _ := proc.Call(0, uintptr(unsafe.Pointer(&size)), 0, uintptr(family), 1, 0)
		if code != 0 && code != 122 {
			return endpoints, fmt.Errorf("UDP一覧取得エラー (%d)", code)
		}
		if size < 4 || size > 16<<20 {
			continue
		}
		buf := make([]byte, size)
		code, _, _ = proc.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0, uintptr(family), 1, 0)
		if code != 0 {
			return endpoints, fmt.Errorf("UDP一覧取得エラー (%d)", code)
		}
		count := int(binary.LittleEndian.Uint32(buf[:4]))
		rowSize := 12
		if family == 23 {
			rowSize = 28
		}
		if count > (len(buf)-4)/rowSize {
			return endpoints, fmt.Errorf("UDP一覧のサイズが不正です")
		}
		for i := 0; i < count; i++ {
			r := buf[4+i*rowSize : 4+(i+1)*rowSize]
			ip := net.IP(r[:4])
			portAt := 4
			pidAt := 8
			if family == 23 {
				ip = net.IP(r[:16])
				portAt = 20
				pidAt = 24
			}
			port := bits.ReverseBytes16(uint16(binary.LittleEndian.Uint32(r[portAt : portAt+4])))
			pid := binary.LittleEndian.Uint32(r[pidAt : pidAt+4])
			pk := fmt.Sprintf("%d:%d", family, port)
			if owners[pk] == nil {
				owners[pk] = map[uint32]bool{}
			}
			owners[pk][pid] = true
			rows = append(rows, binding{udpEndpointKey(ip.String(), port), pk, pid})
		}
	}
	checked := map[uint32]bool{}
	allowed := map[uint32]bool{}
	for _, r := range rows {
		if len(owners[r.portKey]) != 1 {
			continue
		}
		if !checked[r.pid] {
			checked[r.pid] = true
			h, e := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, r.pid)
			if e == nil {
				var path [32768]uint16
				n := uint32(len(path))
				e = windows.QueryFullProcessImageName(h, 0, &path[0], &n)
				windows.CloseHandle(h)
				if e == nil {
					name := filepath.Base(windows.UTF16ToString(path[:n]))
					for _, want := range profile.ProcessNames {
						if strings.EqualFold(name, want) {
							allowed[r.pid] = true
						}
					}
				}
			}
		}
		if allowed[r.pid] {
			endpoints[r.key] = true
		}
	}
	return endpoints, nil
}
