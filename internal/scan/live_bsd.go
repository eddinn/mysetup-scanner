//go:build darwin || freebsd || openbsd || netbsd

package scan

import (
	"encoding/binary"
	"os"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// The sysctl values that are numbers, and their width; the others are text.
var numericSysctls = map[string]int{
	"hw.physicalcpu": 32, "hw.logicalcpu": 32, "hw.optional.arm64": 32, "kern.hv_vmm_present": 32, "hw.memsize": 64,
	"kern.smp.cores": 32, "hw.ncpu": 32, "security.jail.jailed": 32, "hw.ncpuonline": 32,
	"hw.physmem": map[string]int{"freebsd": 64, "openbsd": 64}[runtime.GOOS], "hw.physmem64": 64,
}

// sysctl reads one value straight from the kernel, without running the sysctl tool; unknown names give nothing.
func sysctl(name string) string {
	switch numericSysctls[name] {
	case 32:
		if n, err := unix.SysctlUint32(name); err == nil {
			return strconv.FormatUint(uint64(n), 10)
		}
	case 64:
		if n, err := unix.SysctlUint64(name); err == nil {
			return strconv.FormatUint(n, 10)
		}
	default:
		if value, err := unix.Sysctl(name); err == nil {
			return strings.TrimSpace(value)
		}
	}

	return ""
}

// Read reads this computer.
func Read() Hardware {
	var name unix.Utsname
	_ = unix.Uname(&name)
	src := UnixSource{Sysctl: sysctl, Run: runTool, Root: os.Geteuid() == 0, Machine: unix.ByteSliceToString(name.Machine[:])}
	var h Hardware
	switch runtime.GOOS {
	case "darwin":
		h = ReadMacOS(src)
	case "freebsd":
		src.Firmware = freebsdFirmware
		h = ReadFreeBSD(src)
	case "openbsd":
		h = ReadSysctlBasics(src, "openbsd", "hw.model", "hw.ncpuonline", "hw.physmem", "hw.vendor", "hw.product")
	case "netbsd":
		h = ReadSysctlBasics(src, "netbsd", "machdep.cpu_brand", "hw.ncpuonline", "hw.physmem64", "machdep.dmi.board-vendor", "machdep.dmi.board-product")
	}
	h.FinalNotes()

	return h
}

// freebsdFirmware reads the SMBIOS table from physical memory, where the loader says its entry point is (kenv
// hint.smbios.0.mem), as dmidecode does. Only root may read /dev/mem.
func freebsdFirmware() []byte {
	address, err := strconv.ParseUint(strings.TrimPrefix(strings.TrimSpace(string(runTool("kenv", "-q", "hint.smbios.0.mem"))), "0x"), 16, 64)
	if err != nil || address == 0 {
		return nil
	}
	mem, err := os.Open("/dev/mem")
	if err != nil {
		return nil
	}
	defer mem.Close()
	entry := make([]byte, 32)
	if _, err := mem.ReadAt(entry, int64(address)); err != nil {
		return nil
	}
	var table uint64
	var length int
	switch {
	case string(entry[:5]) == "_SM3_":
		table, length = binary.LittleEndian.Uint64(entry[0x10:]), int(binary.LittleEndian.Uint32(entry[0x0C:]))
	case string(entry[:4]) == "_SM_":
		table, length = uint64(binary.LittleEndian.Uint32(entry[0x18:])), int(binary.LittleEndian.Uint16(entry[0x16:]))
	default:
		return nil
	}
	if length <= 0 || length > 1<<20 {
		return nil
	}
	data := make([]byte, length)
	if _, err := mem.ReadAt(data, int64(table)); err != nil {
		return nil
	}

	return data
}
