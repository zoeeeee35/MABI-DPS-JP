package main

// Map the November 9 JP receive opcodes to the existing internal message kinds.
// The wire ID remains available to capture observers for protocol research.
func jpReceiveMessageKind(id uint32) uint32 {
	switch id {
	case 0x52cc:
		return 0x5243 // Damage
	case 0x502e:
		return 0x4ff7 // Effect association
	case 0x29850:
		return 0x1d4f8 // Skill use
	case 0x298dd:
		return 0x1d596 // Resource update
	default:
		return id
	}
}
