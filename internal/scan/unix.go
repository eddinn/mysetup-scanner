package scan

import (
	"bufio"
	"bytes"
	"regexp"
	"strconv"
	"strings"

	"github.com/eddinn/mysetup-scanner/internal/smbios"
)

// UnixSource is a macOS, BSD or illumos computer as the readers see it: its sysctl values, the output of its own built-in
// tools (Run takes the tool's name and arguments; the live runner finds it in the system's own directories only), whether
// the app runs as root, and the firmware's SMBIOS table where the system hands it to root.
type UnixSource struct {
	Sysctl   func(name string) string
	Run      func(tool string, args ...string) []byte
	Root     bool
	Machine  string
	Firmware func() []byte
}

// ReadMacOS reads a Mac from sysctl and Apple's own system_profiler and diskutil, by exact key: the entries for displays and
// memory also hold serial numbers, which are never kept. Not yet tested on a real Mac.
func ReadMacOS(src UnixSource) Hardware {
	h := Hardware{Family: "macos", OSName: "macos", OSVersion: src.Sysctl("kern.osproductversion"), Architecture: NormalArchitecture(src.Machine)}
	if src.Sysctl("hw.optional.arm64") == "1" {
		h.Architecture = "arm64"
	}
	switch src.Sysctl("kern.hv_vmm_present") {
	case "1":
		h.Environment = "virtual_machine"
	case "0":
		h.Environment = "physical"
	default:
		h.Environment = "unknown"
	}
	h.CPU.Model = src.Sysctl("machdep.cpu.brand_string")
	switch {
	case strings.HasPrefix(h.CPU.Model, "Apple"):
		h.CPU.Vendor = "Apple"
	case strings.Contains(h.CPU.Model, "Intel"):
		h.CPU.Vendor = "Intel"
	}
	h.CPU.Cores, _ = strconv.Atoi(src.Sysctl("hw.physicalcpu"))
	h.CPU.Threads, _ = strconv.Atoi(src.Sysctl("hw.logicalcpu"))

	// Only these keys; the "Displays" entries below them hold serial numbers.
	var gpu *GPU
	var vendor, device string
	flush := func() {
		if gpu != nil {
			if vendor != "" && device != "" {
				gpu.PCI = vendor + ":" + device
			}
			h.GPUs = append(h.GPUs, *gpu)
		}
		gpu, vendor, device = nil, "", ""
	}
	vendorID := regexp.MustCompile(` *\(0x([0-9a-f]{4})\)`)
	eachKey(src.Run("system_profiler", "SPDisplaysDataType"), func(key, value string) {
		switch key {
		case "Chipset Model":
			flush()
			gpu = &GPU{Model: value}
		case "Vendor":
			if gpu != nil {
				if m := vendorID.FindStringSubmatch(value); m != nil {
					vendor = m[1]
					value = vendorID.ReplaceAllString(value, "")
				}
				gpu.Vendor = strings.TrimPrefix(value, "sppci_vendor_")
			}
		case "Device ID":
			device = strings.TrimPrefix(value, "0x")
		}
	})
	flush()

	if model := src.Sysctl("hw.model"); model != "" {
		// A Mac is its model ("MacBookPro18,3"); the site does not list the board apart when it is the same.
		h.System = System{Vendor: "Apple", Model: model}
		h.Board = Board{Vendor: "Apple", Model: model}
	}
	h.Memory.TotalMB = bytesToMB(src.Sysctl("hw.memsize"))
	// By exact key; "Serial Number" is not kept. Empty slots say "Empty" and are skipped.
	var module *Module
	flushModule := func() {
		if module != nil && module.SizeMB > 0 {
			h.Memory.Modules = append(h.Memory.Modules, *module)
		}
		module = nil
	}
	eachKey(src.Run("system_profiler", "SPMemoryDataType"), func(key, value string) {
		switch {
		case key == "Size" || (key == "Memory" && value != ""):
			flushModule()
			module = &Module{SizeMB: sizeToMB(value)}
		case module == nil:
		case key == "Type":
			module.Type = value
		case key == "Speed":
			module.Speed = leadingNumber(value)
		case key == "Manufacturer":
			module.Maker = value
		case key == "Part Number":
			module.Part = value
		}
	})
	flushModule()
	if len(h.Memory.Modules) == 0 {
		h.ModulesReason = "not-exposed"
	}
	h.ConfiguredReason = "not-exposed"

	// "diskutil info" by exact key, for the disks it calls physical.
	diskName := regexp.MustCompile(`(?m)^/dev/(disk[0-9]+) `)
	for _, m := range diskName.FindAllSubmatch(src.Run("diskutil", "list", "physical"), -1) {
		info := src.Run("diskutil", "info", string(m[1]))
		d := Disk{Model: keyValue(info, ":", "Device / Media Name")}
		if size := regexp.MustCompile(`\(([0-9]+) Bytes\)`).FindStringSubmatch(keyValue(info, ":", "Disk Size")); size != nil {
			d.Bytes, _ = strconv.ParseInt(size[1], 10, 64)
		}
		d.Interface = map[string]string{"Apple Fabric": "apple-fabric", "PCI-Express": "nvme", "SATA": "sata", "USB": "usb",
			"Thunderbolt": "thunderbolt"}[keyValue(info, ":", "Protocol")]
		d.Media = map[string]string{"Yes": "ssd", "No": "hdd"}[keyValue(info, ":", "Solid State")]
		h.Disks = append(h.Disks, d)
	}

	return h
}

// ReadFreeBSD reads a FreeBSD computer from sysctl and the base system's kenv, pciconf and geom, by exact key; geom's "ident"
// (the serial number) is never kept. As root, the memory modules come from the firmware table. Not yet tested on a real machine.
func ReadFreeBSD(src UnixSource) Hardware {
	h := Hardware{Family: "freebsd", OSName: "freebsd", OSVersion: strings.TrimSpace(string(src.Run("freebsd-version", "-u"))),
		Architecture: NormalArchitecture(src.Machine)}
	switch {
	case src.Sysctl("security.jail.jailed") == "1":
		h.Environment = "container"
	case src.Sysctl("kern.vm_guest") == "none":
		h.Environment = "physical"
	case src.Sysctl("kern.vm_guest") == "":
		h.Environment = "unknown"
	default:
		h.Environment = "virtual_machine"
	}
	h.CPU.Model = src.Sysctl("hw.model")
	switch {
	case strings.Contains(h.CPU.Model, "Intel"):
		h.CPU.Vendor = "Intel"
	case strings.Contains(h.CPU.Model, "AMD"):
		h.CPU.Vendor = "AMD"
	}
	h.CPU.Cores, _ = strconv.Atoi(src.Sysctl("kern.smp.cores"))
	h.CPU.Threads, _ = strconv.Atoi(src.Sysctl("hw.ncpu"))

	// Display-class devices from "pciconf -l": vendor=/device= here, chip=/card= on older systems.
	pick := func(line, key string) string {
		if m := regexp.MustCompile(`\b` + key + `=0x([0-9a-f]+)`).FindStringSubmatch(line); m != nil {
			return m[1]
		}

		return ""
	}
	for _, line := range strings.Split(string(src.Run("pciconf", "-l")), "\n") {
		if !strings.Contains(line, "class=0x03") {
			continue
		}
		vendor, device, svendor, sdevice := pick(line, "vendor"), pick(line, "device"), pick(line, "subvendor"), pick(line, "subdevice")
		if chip := pick(line, "chip"); vendor == "" && len(chip) == 8 {
			device, vendor = chip[:4], chip[4:]
		}
		if card := pick(line, "card"); svendor == "" && len(card) == 8 {
			sdevice, svendor = card[:4], card[4:]
		}
		if vendor != "" {
			h.GPUs = append(h.GPUs, GPU{PCI: vendor + ":" + device, Subsystem: svendor + ":" + sdevice})
		}
	}

	kenv := func(name string) string { return strings.TrimSpace(string(src.Run("kenv", "-q", name))) }
	// By name: smbios.system.serial and smbios.system.uuid are never read. FreeBSD hands over no chassis type.
	h.System = System{Vendor: kenv("smbios.system.maker"), Model: kenv("smbios.system.product"), Family: kenv("smbios.system.family"),
		Version: kenv("smbios.system.version"), SKU: kenv("smbios.system.sku")}
	h.Board = Board{Vendor: kenv("smbios.planar.maker"), Model: kenv("smbios.planar.product")}
	h.Memory.TotalMB = bytesToMB(src.Sysctl("hw.physmem"))
	firmwareModules(src, &h)

	// "geom disk list" by exact key.
	var name, model, size, rotation string
	flush := func() {
		if name != "" && !strings.HasPrefix(name, "cd") {
			d := Disk{Model: model}
			d.Bytes, _ = strconv.ParseInt(size, 10, 64)
			switch {
			case strings.HasPrefix(name, "ada"):
				d.Interface = "sata"
			case strings.HasPrefix(name, "nvd"), strings.HasPrefix(name, "nda"):
				d.Interface = "nvme"
			case strings.HasPrefix(name, "mmcsd"):
				d.Interface = "mmc"
			case strings.HasPrefix(name, "vtbd"):
				d.Interface = "virtio"
			}
			switch {
			case rotation == "0":
				d.Media = "ssd"
			case rotation != "" && rotation[0] >= '1' && rotation[0] <= '9':
				d.Media = "hdd"
			}
			h.Disks = append(h.Disks, d)
		}
		name, model, size, rotation = "", "", "", ""
	}
	eachKey(src.Run("geom", "disk", "list"), func(key, value string) {
		switch key {
		case "Geom name":
			flush()
			name = value
		case "Mediasize":
			size = strings.Fields(value + " ")[0]
		case "descr":
			model = value
		case "rotationrate":
			rotation = value
		}
	})
	flush()

	return h
}

// ReadSysctlBasics reads what OpenBSD and NetBSD give through sysctl: the processor, its thread count, the memory, the computer's
// model and the board. Not yet tested on a real machine.
func ReadSysctlBasics(src UnixSource, family, model, threads, memory, systemVendor, systemModel, boardVendor, boardModel string) Hardware {
	h := Hardware{Family: family, OSName: family, OSVersion: src.Sysctl("kern.osrelease"), Architecture: NormalArchitecture(src.Machine),
		Environment: "unknown"}
	h.CPU.Model = src.Sysctl(model)
	h.CPU.Threads, _ = strconv.Atoi(src.Sysctl(threads))
	h.Memory.TotalMB = bytesToMB(src.Sysctl(memory))
	h.System = System{Vendor: src.Sysctl(systemVendor), Model: src.Sysctl(systemModel)}
	h.Board = Board{Vendor: src.Sysctl(boardVendor), Model: src.Sysctl(boardModel)}
	firmwareModules(src, &h)

	return h
}

// ReadIllumos reads an illumos computer from its psrinfo, prtconf and smbios tools, by exact key; smbios's "Serial Number" is
// never kept. Not yet tested on a real machine.
func ReadIllumos(src UnixSource) Hardware {
	h := Hardware{Family: "illumos", OSName: "illumos", OSVersion: src.Sysctl("kern.osrelease"), Architecture: NormalArchitecture(src.Machine),
		Environment: "unknown"}
	lines := strings.Split(strings.TrimSpace(string(src.Run("psrinfo", "-pv"))), "\n")
	h.CPU.Model = strings.TrimSpace(lines[len(lines)-1])
	if out := strings.TrimSpace(string(src.Run("psrinfo"))); out != "" {
		h.CPU.Threads = len(strings.Split(out, "\n"))
	}
	h.Memory.TotalMB, _ = strconv.ParseInt(strings.TrimSpace(string(src.Run("prtconf", "-m"))), 10, 64)
	system := src.Run("smbios", "-t", "SMB_TYPE_SYSTEM")
	h.System = System{Vendor: keyValue(system, ":", "Manufacturer"), Model: keyValue(system, ":", "Product"), Family: keyValue(system, ":", "Family"),
		Version: keyValue(system, ":", "Version"), SKU: keyValue(system, ":", "SKU Number")}
	// "Chassis Type: 0xa (notebook)"; the chassis's serial number and asset tag are not read.
	if m := regexp.MustCompile(`^0x([0-9a-f]{1,2})\b`).FindStringSubmatch(keyValue(src.Run("smbios", "-t", "SMB_TYPE_CHASSIS"), ":", "Chassis Type")); m != nil {
		if kind, err := strconv.ParseUint(m[1], 16, 8); err == nil {
			h.System.Chassis = smbios.ChassisName(byte(kind))
		}
	}
	board := src.Run("smbios", "-t", "SMB_TYPE_BASEBOARD")
	h.Board = Board{Vendor: keyValue(board, ":", "Manufacturer"), Model: keyValue(board, ":", "Product")}
	firmwareModules(src, &h)

	return h
}

// firmwareModules reads the memory modules from the firmware table where the system hands it to root; otherwise says what it
// would take.
func firmwareModules(src UnixSource, h *Hardware) {
	if h.Environment == "container" {
		h.ModulesReason = "not-exposed"

		return
	}
	if !src.Root {
		h.ModulesReason = "needs-root"
		h.Note("Memory modules need root here. To add them, run the app again with sudo or doas.")

		return
	}
	if src.Firmware != nil {
		if table := src.Firmware(); table != nil {
			h.Memory.Modules = modulesFrom(smbios.Parse(table))
		}
	}
	if len(h.Memory.Modules) == 0 {
		h.ModulesReason = "no-source"
		h.Note("Running as root, the app still could not read the memory modules on this system.")
	}
}

// eachKey calls fn for every "key: value" line of a tool's output, both trimmed; lines without a colon are skipped.
func eachKey(output []byte, fn func(key, value string)) {
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if at := strings.Index(line, ":"); at >= 0 {
			fn(strings.TrimSpace(line[:at]), strings.TrimSpace(line[at+1:]))
		}
	}
}

func bytesToMB(value string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0
	}

	return n / 1048576
}

// sizeToMB reads "16 GB", "512 MB" or "1 TB".
func sizeToMB(value string) int64 {
	fields := strings.Fields(value)
	if len(fields) < 2 {
		return 0
	}
	n, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	switch fields[1] {
	case "TB":
		return int64(n * 1048576)
	case "GB":
		return int64(n * 1024)
	case "MB":
		return int64(n)
	}

	return 0
}

func leadingNumber(value string) int {
	n, _ := strconv.Atoi(regexp.MustCompile(`^[0-9]*`).FindString(value))

	return n
}
