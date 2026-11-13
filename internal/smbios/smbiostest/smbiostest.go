// Package smbiostest builds made-up SMBIOS tables for tests, full of planted values that must never reach a report.
package smbiostest

import "encoding/binary"

// Structure builds one SMBIOS structure: its formatted area (type and length filled in) and its strings.
func Structure(kind byte, formatted []byte, strs ...string) []byte {
	formatted[0], formatted[1] = kind, byte(len(formatted))
	out := append([]byte{}, formatted...)
	if len(strs) == 0 {
		return append(out, 0, 0)
	}
	for _, s := range strs {
		out = append(append(out, s...), 0)
	}

	return append(out, 0)
}

// MadeUpTable is a firmware table full of planted values that must never be read: serial numbers, asset tags, locators and a UUID.
func MadeUpTable() []byte {
	system := make([]byte, 0x1B)
	system[4], system[5], system[6], system[7] = 1, 2, 3, 4
	copy(system[8:], "FAKE-UUID-BYTES!") // The UUID, binary, at 0x08.
	system[0x19], system[0x1A] = 5, 6    // SKU and family.
	chassis := make([]byte, 0x0D)
	chassis[4], chassis[5], chassis[6], chassis[7], chassis[8] = 1, 0x80|0x0A, 2, 3, 4 // A locked notebook; version, serial, tag.
	board := make([]byte, 0x0F)
	board[4], board[5], board[6], board[7], board[8] = 1, 2, 3, 4, 5
	cpu := make([]byte, 0x30)
	cpu[0x18] = 0x41 // Populated, enabled.
	cpu[0x20] = 1    // Serial number string.
	cpu[0x23], cpu[0x25] = 8, 16
	empty := make([]byte, 0x30)
	memory := func(size uint16, extended uint32) []byte {
		m := make([]byte, 0x5C)
		binary.LittleEndian.PutUint16(m[0x0C:], size)
		binary.LittleEndian.PutUint32(m[0x1C:], extended)
		m[0x0E], m[0x10], m[0x11], m[0x12] = 0x09, 1, 2, 0x22
		binary.LittleEndian.PutUint16(m[0x15:], 0xFFFF)
		binary.LittleEndian.PutUint32(m[0x54:], 6000)
		m[0x17], m[0x18], m[0x19], m[0x1A] = 3, 4, 5, 6
		binary.LittleEndian.PutUint16(m[0x20:], 6000)

		return m
	}
	var table []byte
	table = append(table, Structure(1, system, "Micro-Star International Co., Ltd.", "GL65 Leopard 9SCSR", "REV:1.0", "FAKE-SERIAL", "16R3.1", "GL")...)
	table = append(table, Structure(3, chassis, "Micro-Star International Co., Ltd.", "N/A", "FAKE-SERIAL", "FAKE-TAG")...)
	table = append(table, Structure(2, board, "ASUSTeK COMPUTER INC.", "ROG STRIX X670E-E", "Rev 1.xx", "FAKE-SERIAL", "FAKE-TAG")...)
	table = append(table, Structure(4, cpu, "FAKE-SERIAL")...)
	table = append(table, Structure(4, empty)...)
	table = append(table, Structure(17, memory(0x7FFF, 32768), "FAKE-LOCATOR", "FAKE-LOCATOR", "Corsair", "FAKE-SERIAL", "FAKE-TAG", "CMK64GX5M2B6000C30")...)
	table = append(table, Structure(17, memory(0, 0), "FAKE-LOCATOR", "FAKE-LOCATOR")...)
	table = append(table, Structure(127, make([]byte, 4))...)

	return table
}
