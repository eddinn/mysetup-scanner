package scan

import (
	"bufio"
	"bytes"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/eddinn/mysetup-scanner/internal/smbios"
)

// LinuxSource is a Linux computer as the reader sees it: its files from "/" (FS paths have no leading slash), whether the
// app runs as root, and the kernel's machine type.
type LinuxSource struct {
	FS      fs.FS
	Root    bool
	Machine string
}

// ReadLinux reads a Linux computer from its own files: /proc, /sys, os-release and the udev database. It runs no program.
// Sources that also hold serial numbers (udev, the firmware table) are read by exact key.
func ReadLinux(src LinuxSource) Hardware {
	h := Hardware{Family: "linux", Architecture: NormalArchitecture(src.Machine)}
	h.Environment = linuxEnvironment(src.FS)
	h.OSName, h.OSVersion = linuxOS(src.FS)
	// Tested on real x86-64 machines with udev (2026-11-07); see the README.
	h.Tested = h.Architecture == "x86-64" && exists(src.FS, "run/udev/data")
	if h.Environment == "wsl" {
		return h
	}
	h.CPU = linuxCPU(src.FS)
	h.GPUs = linuxGPUs(src.FS, &h)
	h.Board = Board{Vendor: firstLine(src.FS, "sys/class/dmi/id/board_vendor"), Model: firstLine(src.FS, "sys/class/dmi/id/board_name")}
	if h.Board.Model == "" {
		h.Board.Model = firstLine(src.FS, "proc/device-tree/model")
	}
	h.Memory.TotalMB = linuxMemTotal(src.FS)
	linuxModules(src, &h)
	h.Disks = linuxDisks(src.FS)

	return h
}

func linuxEnvironment(fsys fs.FS) string {
	release := strings.ToLower(firstLine(fsys, "proc/sys/kernel/osrelease"))
	if strings.Contains(release, "microsoft") || strings.Contains(release, "wsl") {
		return "wsl"
	}
	for _, marker := range []string{"run/systemd/container", ".dockerenv", "run/.containerenv"} {
		if exists(fsys, marker) {
			return "container"
		}
	}
	flags := " " + keyValue(readFile(fsys, "proc/cpuinfo"), ":", "flags") + " "
	if strings.Contains(flags, " hypervisor ") {
		return "virtual_machine"
	}
	vendor := strings.ToLower(firstLine(fsys, "sys/class/dmi/id/sys_vendor") + " " + firstLine(fsys, "sys/class/dmi/id/product_name"))
	for _, virtual := range []string{"qemu", "kvm", "vmware", "innotek", "virtualbox", "xen", "bochs", "parallels", "virtual machine"} {
		if strings.Contains(vendor, virtual) {
			return "virtual_machine"
		}
	}
	if strings.TrimSpace(flags) != "" {
		return "physical"
	}
	switch board := firstLine(fsys, "proc/device-tree/model"); {
	case board == "", strings.Contains(strings.ToLower(board), "virt"), strings.Contains(board, "KVM"), strings.Contains(board, "QEMU"):
		return "unknown"
	}

	return "physical"
}

// linuxOS reads os-release as text: it is shell syntax, and is never run.
func linuxOS(fsys fs.FS) (string, string) {
	text := readFile(fsys, "etc/os-release")
	if text == nil {
		text = readFile(fsys, "usr/lib/os-release")
	}
	unquote := func(value string) string { return strings.Trim(value, `"'`) }

	return unquote(keyValue(text, "=", "ID")), unquote(keyValue(text, "=", "VERSION_ID"))
}

func linuxCPU(fsys fs.FS) CPU {
	info := readFile(fsys, "proc/cpuinfo")
	c := CPU{Vendor: keyValue(info, ":", "vendor_id"), Model: keyValue(info, ":", "model name")}
	if c.Model == "" {
		c.Model = keyValue(info, ":", "Processor")
	}
	// Each logical CPU's package and core, counted once each: right for hybrid processors too.
	cores := map[string]bool{}
	cpus, _ := fs.Glob(fsys, "sys/devices/system/cpu/cpu[0-9]*")
	for _, cpu := range cpus {
		if !isDir(fsys, cpu) {
			continue
		}
		c.Threads++
		if core := firstLine(fsys, cpu+"/topology/core_id"); core != "" {
			cores[firstLine(fsys, cpu+"/topology/physical_package_id")+":"+core] = true
		}
	}
	c.Cores = len(cores)
	if c.Threads == 0 {
		c.Threads = bytes.Count(info, []byte("\nprocessor")) + boolInt(bytes.HasPrefix(info, []byte("processor")))
	}

	return c
}

func linuxGPUs(fsys fs.FS, h *Hardware) []GPU {
	var gpus []GPU
	names := openPCIIDs(fsys)
	devices, _ := fs.Glob(fsys, "sys/bus/pci/devices/*")
	sort.Strings(devices)
	for _, dev := range devices {
		if !strings.HasPrefix(firstLine(fsys, dev+"/class"), "0x03") {
			continue
		}
		hex := func(file string) string {
			return strings.TrimPrefix(strings.ToLower(firstLine(fsys, dev+"/"+file)), "0x")
		}
		g := GPU{PCI: hex("vendor") + ":" + hex("device"), Subsystem: hex("subsystem_vendor") + ":" + hex("subsystem_device")}
		if names != nil {
			g.Vendor, g.Model = names.lookup(hex("vendor"), hex("device"))
		}
		gpus = append(gpus, g)
	}
	if len(gpus) > 0 && names == nil {
		h.Note("Graphics are listed by PCI ID only: this system has no list of PCI names (pci.ids), which the app reads if present.")
	}
	// Graphics that are not on PCI, as on many ARM boards: what the device tree calls them.
	cards, _ := fs.Glob(fsys, "sys/class/drm/card[0-9]*")
	sort.Strings(cards)
	for _, card := range cards {
		if strings.Contains(path.Base(card), "-") {
			continue // A connector, such as card0-HDMI-A-1.
		}
		event := readFile(fsys, card+"/device/uevent")
		if event == nil || keyValue(event, "=", "PCI_ID") != "" {
			continue
		}
		if name := keyValue(event, "=", "OF_COMPATIBLE_0"); name != "" {
			gpus = append(gpus, GPU{Model: name})
		}
	}

	return gpus
}

func linuxMemTotal(fsys fs.FS) int64 {
	fields := strings.Fields(keyValue(readFile(fsys, "proc/meminfo"), ":", "MemTotal"))
	if len(fields) == 0 {
		return 0
	}
	kb, _ := strconv.ParseInt(fields[0], 10, 64)

	return kb / 1024
}

// linuxModules reads the memory modules from udev's copy of the firmware's memory tables, which any user may read on systemd
// systems; as root, from the firmware table itself, where dmidecode would read it. Neither runs a program.
func linuxModules(src LinuxSource, h *Hardware) {
	h.Memory.Modules = udevModules(readFile(src.FS, "run/udev/data/+dmi:id"))
	if len(h.Memory.Modules) > 0 {
		return
	}
	if h.Environment == "container" || !exists(src.FS, "sys/class/dmi/id") {
		h.ModulesReason = "not-exposed"

		return
	}
	if src.Root {
		if table := readFile(src.FS, "sys/firmware/dmi/tables/DMI"); table != nil {
			h.Memory.Modules = modulesFrom(smbios.Parse(table))
		}
		if len(h.Memory.Modules) == 0 {
			h.Note("Running as root, the app still could not read the memory modules from the firmware table.")
		}

		return
	}
	h.ModulesReason = "needs-root"
	h.Note("Memory modules need root here. To add them, run the app again with sudo.")
}

var udevMemory = regexp.MustCompile(`^E:MEMORY_DEVICE_([0-9]+)_(SIZE|PRESENT|MANUFACTURER|PART_NUMBER|SPEED_MTS|CONFIGURED_SPEED_MTS|TYPE|FORM_FACTOR)=(.*)$`)

// udevModules reads the modules from udev's database entry for the firmware, by exact key: its serial numbers, asset tags
// and locators are never kept.
func udevModules(data []byte) []Module {
	devices := map[int]map[string]string{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		m := udevMemory.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		if devices[n] == nil {
			devices[n] = map[string]string{}
		}
		devices[n][m[2]] = m[3]
	}
	keys := make([]int, 0, len(devices))
	for n := range devices {
		keys = append(keys, n)
	}
	sort.Ints(keys)
	var modules []Module
	for _, n := range keys {
		d := devices[n]
		size, _ := strconv.ParseInt(d["SIZE"], 10, 64)
		if size <= 0 || d["PRESENT"] == "0" {
			continue
		}
		speed, _ := strconv.Atoi(d["SPEED_MTS"])
		running, _ := strconv.Atoi(d["CONFIGURED_SPEED_MTS"])
		modules = append(modules, Module{SizeMB: size / 1048576, Maker: d["MANUFACTURER"], Part: d["PART_NUMBER"], Speed: speed,
			Running: running, Type: d["TYPE"], Form: d["FORM_FACTOR"]})
	}

	return modules
}

func modulesFrom(t smbios.Table) []Module {
	modules := make([]Module, 0, len(t.Memory))
	for _, m := range t.Memory {
		modules = append(modules, Module{SizeMB: m.SizeMB, Maker: m.Maker, Part: m.Part, Speed: m.Speed, Running: m.Running, Type: m.Type, Form: m.Form})
	}

	return modules
}

var notADisk = regexp.MustCompile(`^(loop|ram|zram|zd|nbd|md|dm-|sr|fd|mtdblock|drbd|rbd|bcache|nullb)|(boot[0-9]|rpmb)$`)

// linuxDisks lists whole physical disks; virtual and partition-like devices are not parts of the build. The model comes from
// udev where it has the disk (Linux cuts SATA names to 16 characters elsewhere); the serial number is never read.
func linuxDisks(fsys fs.FS) []Disk {
	blocks, _ := fs.Glob(fsys, "sys/block/*")
	// In device-number order, as lsblk lists them (SATA's 8:0 before NVMe's 259:0), then by name.
	number := func(block string) (int, int) {
		major, minor, _ := strings.Cut(firstLine(fsys, block+"/dev"), ":")
		a, errA := strconv.Atoi(major)
		b, errB := strconv.Atoi(minor)
		if errA != nil || errB != nil {
			return 1 << 30, 0
		}

		return a, b
	}
	sort.Slice(blocks, func(i, j int) bool {
		ai, bi := number(blocks[i])
		aj, bj := number(blocks[j])
		if ai != aj {
			return ai < aj
		}
		if bi != bj {
			return bi < bj
		}

		return blocks[i] < blocks[j]
	})
	var disks []Disk
	for _, block := range blocks {
		name := path.Base(block)
		if notADisk.MatchString(name) {
			continue
		}
		sectors, err := strconv.ParseInt(firstLine(fsys, block+"/size"), 10, 64)
		if err != nil || sectors <= 0 {
			continue
		}
		d := Disk{Bytes: sectors * 512, Interface: interfaceFromName(name)}
		udev := readFile(fsys, "run/udev/data/b"+firstLine(fsys, block+"/dev"))
		d.Model = unescapeUdev(keyValue(udev, "=", "E:ID_MODEL_ENC"))
		for _, file := range []string{"/device/model", "/device/name"} {
			if strings.TrimSpace(d.Model) == "" {
				d.Model = firstLine(fsys, block+file)
			}
		}
		if d.Interface == "" {
			if target, err := fs.ReadLink(fsys, block); err == nil {
				switch {
				case strings.Contains(target, "/usb"):
					d.Interface = "usb"
				case strings.Contains(target, "/ata"):
					d.Interface = "sata"
				case strings.Contains(target, "/end_device-"):
					d.Interface = "sas"
				}
			}
		}
		switch d.Interface {
		case "usb", "virtio", "xen", "":
		default:
			switch firstLine(fsys, block+"/queue/rotational") {
			case "0":
				d.Media = "ssd"
			case "1":
				d.Media = "hdd"
			}
		}
		disks = append(disks, d)
	}

	return disks
}

func interfaceFromName(name string) string {
	switch {
	case strings.HasPrefix(name, "nvme"):
		return "nvme"
	case strings.HasPrefix(name, "mmcblk"):
		return "mmc"
	case strings.HasPrefix(name, "vd"):
		return "virtio"
	case strings.HasPrefix(name, "xvd"):
		return "xen"
	}

	return ""
}

var udevEscape = regexp.MustCompile(`\\x([0-9a-fA-F]{2})`)

// unescapeUdev turns udev's "\x20" escapes back into the characters they stand for.
func unescapeUdev(value string) string {
	return strings.TrimSpace(udevEscape.ReplaceAllStringFunc(value, func(escape string) string {
		n, _ := strconv.ParseUint(escape[2:], 16, 8)

		return string(rune(n))
	}))
}

// Helpers for files, shared by the readers.

func readFile(fsys fs.FS, name string) []byte {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil
	}

	return data
}

// firstLine is a file's first line, trimmed, without the NUL bytes device-tree files end in.
func firstLine(fsys fs.FS, name string) string {
	data := bytes.ReplaceAll(readFile(fsys, name), []byte{0}, nil)
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		data = data[:i]
	}

	return strings.TrimSpace(string(data))
}

// keyValue is the value of the first line whose key, before the separator and trimmed, is exactly key.
func keyValue(data []byte, separator, key string) string {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		at := strings.Index(line, separator)
		if at < 0 || strings.TrimSpace(line[:at]) != key {
			continue
		}

		return strings.TrimSpace(line[at+len(separator):])
	}

	return ""
}

func exists(fsys fs.FS, name string) bool {
	_, err := fs.Stat(fsys, name)

	return err == nil
}

func isDir(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)

	return err == nil && info.IsDir()
}

func boolInt(b bool) int {
	if b {
		return 1
	}

	return 0
}
