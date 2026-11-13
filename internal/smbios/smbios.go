// Package smbios reads the firmware's SMBIOS table, by structure type and field offset as the DMTF specification (DSP0134)
// sets them. Only what the report needs is taken: the computer's maker, model, family, version and SKU, which name its model
// (and tell a virtual machine), and its chassis type; the board's maker and model; the processors' core and thread counts; and
// each memory device's size, maker, part number, speeds, type and form. Serial numbers, asset tags, UUIDs and slot names live in
// the same structures and are never read.
package smbios

import "encoding/binary"

// Table is what was read from one SMBIOS table.
type Table struct {
	SystemMaker, SystemModel               string
	SystemFamily, SystemVersion, SystemSKU string
	// Chassis is the chassis type's name (ChassisName), such as "notebook"; empty when unknown.
	Chassis                string
	BoardMaker, BoardModel string
	Processors             []Processor
	Memory                 []MemoryDevice
}

// Processor is one populated processor socket.
type Processor struct {
	Cores, Threads int
}

// MemoryDevice is one populated memory slot.
type MemoryDevice struct {
	SizeMB         int64
	Maker, Part    string
	Speed, Running int
	Type, Form     string
}

// The memory types and forms the report names (DSP0134 7.18.2 and 7.18.1); others are left unknown.
var memoryTypes = map[byte]string{0x12: "DDR", 0x13: "DDR2", 0x18: "DDR3", 0x1A: "DDR4", 0x1B: "LPDDR", 0x1C: "LPDDR2",
	0x1D: "LPDDR3", 0x1E: "LPDDR4", 0x22: "DDR5", 0x23: "LPDDR5"}

var memoryForms = map[byte]string{0x09: "DIMM", 0x0C: "RIMM", 0x0D: "SODIMM", 0x0E: "SRIMM", 0x0F: "FB-DIMM"}

// Parse reads the structures of a raw table (without the entry point). A malformed structure ends the reading; what was
// read before it is kept.
func Parse(data []byte) Table {
	var t Table
	for at := 0; at+4 <= len(data); {
		kind, length := data[at], int(data[at+1])
		if length < 4 || at+length > len(data) {
			break
		}
		formatted := data[at : at+length]
		end := at + length
		for end+1 < len(data) && !(data[end] == 0 && data[end+1] == 0) {
			end++
		}
		if end+1 >= len(data) {
			break
		}
		strs := data[at+length : end]
		switch kind {
		case 1:
			// 0x07 is the serial number and 0x08 to 0x17 the UUID: never read.
			t.SystemMaker, t.SystemModel, t.SystemVersion = str(formatted, strs, 0x04), str(formatted, strs, 0x05), str(formatted, strs, 0x06)
			t.SystemSKU, t.SystemFamily = str(formatted, strs, 0x19), str(formatted, strs, 0x1A)
		case 3:
			// Only the type: 0x07 is the serial number and 0x08 the asset tag, never read.
			if len(formatted) > 0x05 {
				t.Chassis = ChassisName(formatted[0x05])
			}
		case 2:
			t.BoardMaker, t.BoardModel = str(formatted, strs, 0x04), str(formatted, strs, 0x05)
		case 4:
			if p, ok := processor(formatted); ok {
				t.Processors = append(t.Processors, p)
			}
		case 17:
			if m, ok := memory(formatted, strs); ok {
				t.Memory = append(t.Memory, m)
			}
		}
		at = end + 2
		if kind == 127 {
			break
		}
	}

	return t
}

// processor reads a type 4 structure: populated sockets only, with the 2.0 core and thread counts above 255 when given.
func processor(f []byte) (Processor, bool) {
	if len(f) < 0x24 || f[0x18]&0x40 == 0 { // Status bit 6: socket populated.
		return Processor{}, false
	}
	p := Processor{Cores: int(f[0x23])}
	if len(f) >= 0x26 {
		p.Threads = int(f[0x25])
	}
	if len(f) >= 0x30 {
		if p.Cores == 0xFF {
			p.Cores = int(binary.LittleEndian.Uint16(f[0x2A:]))
		}
		if p.Threads == 0xFF {
			p.Threads = int(binary.LittleEndian.Uint16(f[0x2E:]))
		}
	}

	return p, true
}

// memory reads a type 17 structure. Size 0 is an empty slot, 0xFFFF unknown, 0x7FFF a size in the extended field; bit 15
// set counts in kilobytes. The serial number (0x18), asset tag (0x19) and locators (0x10, 0x11) are never read.
func memory(f, strs []byte) (MemoryDevice, bool) {
	if len(f) < 0x15 {
		return MemoryDevice{}, false
	}
	raw := binary.LittleEndian.Uint16(f[0x0C:])
	var size int64
	switch {
	case raw == 0 || raw == 0xFFFF:
		return MemoryDevice{}, false
	case raw == 0x7FFF && len(f) >= 0x20:
		size = int64(binary.LittleEndian.Uint32(f[0x1C:]) & 0x7FFFFFFF)
	case raw&0x8000 != 0:
		size = int64(raw&0x7FFF) / 1024
	default:
		size = int64(raw)
	}
	if size <= 0 {
		return MemoryDevice{}, false
	}
	m := MemoryDevice{SizeMB: size, Form: memoryForms[f[0x0E]], Type: memoryTypes[f[0x12]]}
	if len(f) >= 0x17 {
		m.Speed = int(binary.LittleEndian.Uint16(f[0x15:]))
	}
	if len(f) >= 0x1B {
		m.Maker, m.Part = str(f, strs, 0x17), str(f, strs, 0x1A)
	}
	if len(f) >= 0x22 {
		m.Running = int(binary.LittleEndian.Uint16(f[0x20:]))
	}
	// 0xFFFF in the speeds means the extended fields hold them (SMBIOS 3.3).
	if len(f) >= 0x5C {
		if m.Speed == 0xFFFF {
			m.Speed = int(binary.LittleEndian.Uint32(f[0x54:]))
		}
		if m.Running == 0xFFFF {
			m.Running = int(binary.LittleEndian.Uint32(f[0x58:]))
		}
	}

	return m, true
}

// chassisNames are DSP0134's chassis types (7.4.1) as the report writes them; "Other" and "Unknown" are left out.
var chassisNames = map[byte]string{0x03: "desktop", 0x04: "low-profile-desktop", 0x05: "pizza-box", 0x06: "mini-tower", 0x07: "tower",
	0x08: "portable", 0x09: "laptop", 0x0A: "notebook", 0x0B: "hand-held", 0x0C: "docking-station", 0x0D: "all-in-one",
	0x0E: "sub-notebook", 0x0F: "space-saving", 0x10: "lunch-box", 0x11: "main-server-chassis", 0x12: "expansion-chassis",
	0x13: "sub-chassis", 0x14: "bus-expansion-chassis", 0x15: "peripheral-chassis", 0x16: "raid-chassis", 0x17: "rack-mount-chassis",
	0x18: "sealed-case-pc", 0x19: "multi-system-chassis", 0x1A: "compact-pci", 0x1B: "advanced-tca", 0x1C: "blade",
	0x1D: "blade-enclosure", 0x1E: "tablet", 0x1F: "convertible", 0x20: "detachable", 0x21: "iot-gateway", 0x22: "embedded-pc",
	0x23: "mini-pc", 0x24: "stick-pc"}

// ChassisName is a chassis type's name, as Linux's chassis_type file gives the number too; the top bit (a lock) is ignored.
func ChassisName(kind byte) string {
	return chassisNames[kind&0x7F]
}

// str is the string a formatted field points to, counting from 1; 0 or a missing string is empty.
func str(f, strs []byte, offset int) string {
	if offset >= len(f) || f[offset] == 0 {
		return ""
	}
	n := int(f[offset])
	start := 0
	for i := 1; start < len(strs); i++ {
		end := start
		for end < len(strs) && strs[end] != 0 {
			end++
		}
		if i == n {
			return string(strs[start:end])
		}
		start = end + 1
	}

	return ""
}
