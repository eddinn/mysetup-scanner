package scan

import (
	"regexp"
	"strings"

	"github.com/eddinn/mysetup-scanner/internal/smbios"
)

// WindowsSource is a Windows computer as the reader sees it, read without PowerShell and without administrator rights: the
// firmware's SMBIOS table, the processor's registry entry, the display adapters SetupAPI lists, and each disk's own answers
// to the storage queries.
type WindowsSource struct {
	Firmware          []byte
	CPUName           string
	CPUVendor         string
	LogicalProcessors int
	Adapters          []WindowsAdapter
	Disks             []WindowsDisk
	TotalMemory       int64
	Version           string
	// Architecture is PROCESSOR_ARCHITECTURE as Windows names it (AMD64, ARM64, x86).
	Architecture string
}

// WindowsAdapter is a display adapter: its device instance ID, of which only the PCI IDs are read, and its name.
type WindowsAdapter struct {
	InstanceID, Name string
}

// WindowsDisk is a disk's answers: its product name, size, bus type (STORAGE_BUS_TYPE) and whether seeking costs time.
type WindowsDisk struct {
	Model       string
	Bytes       int64
	Bus         int
	SeekPenalty *bool
}

var (
	virtualModel = regexp.MustCompile(`^(Virtual Machine|VirtualBox|KVM|HVM domU|Bochs)|VMware|Standard PC|QEMU|Parallels`)
	virtualMaker = regexp.MustCompile(`VMware|innotek|QEMU|Xen|Parallels`)
	pciIDs       = regexp.MustCompile(`^PCI\\VEN_([0-9A-Fa-f]{4})&DEV_([0-9A-Fa-f]{4})`)
	subsys       = regexp.MustCompile(`&SUBSYS_([0-9A-Fa-f]{4})([0-9A-Fa-f]{4})`)
)

// STORAGE_BUS_TYPE values as the report names them; 14 (virtual) and 15 (file-backed) are not parts of the build.
var windowsBuses = map[int]string{1: "scsi", 3: "ata", 7: "usb", 8: "raid", 9: "iscsi", 10: "sas", 11: "sata", 12: "sd", 13: "mmc", 17: "nvme"}

func ReadWindows(src WindowsSource) Hardware {
	h := Hardware{Family: "windows", OSName: "windows", OSVersion: src.Version}
	switch src.Architecture {
	case "AMD64":
		h.Architecture = "x86-64"
	case "ARM64":
		h.Architecture = "arm64"
	case "x86":
		h.Architecture = "x86"
	default:
		h.Architecture = "other"
	}
	// Tested on a real x86-64 machine, without administrator rights (2026-11-07); see docs/app.md.
	h.Tested = h.Architecture == "x86-64"
	table := smbios.Parse(src.Firmware)
	switch {
	case len(src.Firmware) == 0:
		h.Environment = "unknown"
	case virtualModel.MatchString(table.SystemModel) || virtualMaker.MatchString(table.SystemMaker):
		h.Environment = "virtual_machine"
	default:
		h.Environment = "physical"
	}

	h.CPU = CPU{Vendor: src.CPUVendor, Model: strings.TrimSpace(src.CPUName)}
	for _, p := range table.Processors {
		h.CPU.Cores += p.Cores
		h.CPU.Threads += p.Threads
	}
	if h.CPU.Threads == 0 {
		h.CPU.Threads = src.LogicalProcessors
	}

	// Only the IDs are taken from the instance ID; the rest of it is specific to this computer. Adapters under ROOT\ are
	// virtual (remote desktop, display software) and not parts of the build.
	for _, a := range src.Adapters {
		id := strings.ToUpper(a.InstanceID)
		if !strings.HasPrefix(id, `PCI\`) && !strings.HasPrefix(id, `ACPI\`) {
			continue
		}
		g := GPU{Model: a.Name}
		if m := pciIDs.FindStringSubmatch(id); m != nil {
			g.PCI = strings.ToLower(m[1] + ":" + m[2])
		}
		if m := subsys.FindStringSubmatch(id); m != nil {
			g.Subsystem = strings.ToLower(m[2] + ":" + m[1])
		}
		h.GPUs = append(h.GPUs, g)
	}

	h.System = System{Vendor: table.SystemMaker, Model: table.SystemModel, Family: table.SystemFamily, Version: table.SystemVersion,
		SKU: table.SystemSKU, Chassis: table.Chassis}
	h.Board = Board{Vendor: table.BoardMaker, Model: table.BoardModel}
	h.Memory.TotalMB = src.TotalMemory / 1048576
	h.Memory.Modules = modulesFrom(table)
	if len(src.Firmware) == 0 {
		h.Note("Windows did not hand over the firmware table, so the board and memory modules are missing.")
	}

	for _, d := range src.Disks {
		if d.Bytes <= 0 || d.Bus == 14 || d.Bus == 15 {
			continue
		}
		disk := Disk{Model: d.Model, Bytes: d.Bytes, Interface: windowsBuses[d.Bus]}
		if d.SeekPenalty != nil && disk.Interface != "usb" {
			disk.Media = map[bool]string{true: "hdd", false: "ssd"}[*d.SeekPenalty]
		}
		h.Disks = append(h.Disks, disk)
	}

	return h
}
