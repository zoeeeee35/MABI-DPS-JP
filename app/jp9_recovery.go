package main

import "encoding/binary"

// Recovery requires three complete JP9 receive frames and two independently
// validated known messages. Never join pre-gap damage with post-gap metadata.
// Unknown packets alone and a single signature cannot establish synchronization.
func jp9RecoveryOffset(b []byte) int {
	for start := 0; start+9 <= len(b); start++ {
		pos, anchors := start, 0
		valid := true
		for frame := 0; frame < 3; frame++ {
			if pos+9 > len(b) {
				valid = false
				break
			}
			id := binary.LittleEndian.Uint32(b[pos:])
			size := binary.LittleEndian.Uint32(b[pos+4:])
			if id == 0 || id > 0x200000 || size > maxDecodedBlock || b[pos+8] > 1 || uint64(pos)+9+uint64(size) > uint64(len(b)) {
				valid = false
				break
			}
			body := b[pos+9 : pos+9+int(size)]
			if b[pos+8] == 0 {
				switch jpReceiveMessageKind(id) {
				case 0x5243:
					if len(body) != 53 {
						valid = false
					} else {
						anchors++
					}
				case 0x4ff7:
					if len(body) != 35 {
						valid = false
					} else {
						anchors++
					}
				case 0x1d596:
					if len(body) != 21 {
						valid = false
					} else {
						anchors++
					}
				case 0x1d4f8:
					if _, ok := decodeJPSkillUse(body); !ok {
						valid = false
					} else {
						anchors++
					}
				}
			}
			if !valid {
				break
			}
			pos += 9 + int(size)
		}
		if valid && anchors >= 2 {
			return start
		}
	}
	return -1
}
func (d *jp9Decoder) recoverReceive() bool {
	offset := jp9RecoveryOffset(d.buffer)
	if offset < 0 {
		// Bound search memory and CPU while waiting for stronger evidence.
		if len(d.buffer) > 128<<10 {
			drop := len(d.buffer) - (64 << 10)
			d.counts.Discarded += uint64(drop)
			d.buffer = append([]byte(nil), d.buffer[drop:]...)
		}
		return false
	}
	d.counts.Discarded += uint64(offset)
	d.buffer = d.buffer[offset:]
	d.last = nil
	// Missing HP updates must not produce a cross-gap HP/damage match.
	d.health = jpHealthTracker{Stream: d.health.Stream}
	d.stopped = false
	d.counts.Recovered++
	return true
}
