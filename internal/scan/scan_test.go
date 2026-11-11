package scan

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/eddinn/mysetup-scanner/internal/smbios/smbiostest"
)

// What a source holds beside what the report wants. None of it may ever reach a report or a note.
var planted = []string{"FAKE-SERIAL", "FAKE-TAG", "FAKE-LOCATOR", "FAKE-HOST", "FAKE-USER", "FAKE-UNIQUE", "FAKE-UUID",
	"de:ad:be:ef:00:01", "10.20.30.40", "12345678-1234-1234-1234-123456789abc"}

func assertClean(t *testing.T, h Hardware, text string) {
	t.Helper()
	all := text + strings.Join(h.Notes, "\n")
	for _, value := range planted {
		if strings.Contains(all, value) {
			t.Fatalf("%s reached the report or a note:\n%s", value, all)
		}
	}
}

func assertLines(t *testing.T, text string, lines ...string) {
	t.Helper()
	for _, line := range lines {
		if !strings.Contains("\n"+text, "\n"+line+"\n") {
			t.Errorf("missing %q in:\n%s", line, text)
		}
	}
}

func file(text string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(text)} }

// madeUpLinux is a desktop with a hybrid-style topology, an NVIDIA card, udev's memory entries, an NVMe and a SATA disk.
func madeUpLinux() fstest.MapFS {
	fsys := fstest.MapFS{
		"proc/sys/kernel/osrelease":                         file("6.12.43+deb13-amd64\n"),
		"proc/cpuinfo":                                      file("processor\t: 0\nvendor_id\t: AuthenticAMD\nmodel name\t: AMD Ryzen 7 7800X3D 8-Core Processor\nflags\t\t: fpu sse avx2\n"),
		"proc/meminfo":                                      file("MemTotal:       32656088 kB\nMemFree: 1 kB\n"),
		"etc/os-release":                                    file("PRETTY_NAME=\"FAKE-HOST\"\nID=debian\nVERSION_ID=\"13\"\n"),
		"etc/hostname":                                      file("FAKE-HOST\n"),
		"sys/class/dmi/id/board_vendor":                     file("ASUSTeK COMPUTER INC.\n"),
		"sys/class/dmi/id/board_name":                       file("ROG STRIX X670E-E GAMING WIFI\n"),
		"sys/class/dmi/id/board_serial":                     file("FAKE-SERIAL\n"),
		"sys/class/dmi/id/product_uuid":                     file("12345678-1234-1234-1234-123456789abc\n"),
		"sys/class/dmi/id/sys_vendor":                       file("ASUS\n"),
		"sys/bus/pci/devices/0000:01:00.0/class":            file("0x030000\n"),
		"sys/bus/pci/devices/0000:01:00.0/vendor":           file("0x10de\n"),
		"sys/bus/pci/devices/0000:01:00.0/device":           file("0x2204\n"),
		"sys/bus/pci/devices/0000:01:00.0/subsystem_vendor": file("0x1458\n"),
		"sys/bus/pci/devices/0000:01:00.0/subsystem_device": file("0x4037\n"),
		"sys/bus/pci/devices/0000:00:14.0/class":            file("0x0c0330\n"),
		"usr/share/misc/pci.ids": file("# comment\n1002  Advanced Micro Devices, Inc. [AMD/ATI]\n\t7480  Navi 33\n" +
			"10de  NVIDIA Corporation\n\t2204  GA102 [GeForce RTX 3090]\n\t\t1458 4037  Not this line\n10df  Emulex\n"),
		"run/udev/data/+dmi:id": file("E:MEMORY_DEVICE_0_SIZE=17179869184\nE:MEMORY_DEVICE_0_MANUFACTURER=Corsair\n" +
			"E:MEMORY_DEVICE_0_PART_NUMBER=CMK32GX5M2B6000C30\nE:MEMORY_DEVICE_0_SERIAL_NUMBER=FAKE-SERIAL\n" +
			"E:MEMORY_DEVICE_0_ASSET_TAG=FAKE-TAG\nE:MEMORY_DEVICE_0_LOCATOR=FAKE-LOCATOR\nE:MEMORY_DEVICE_0_SPEED_MTS=6000\n" +
			"E:MEMORY_DEVICE_0_CONFIGURED_SPEED_MTS=6000\nE:MEMORY_DEVICE_0_TYPE=DDR5\nE:MEMORY_DEVICE_0_FORM_FACTOR=DIMM\n" +
			"E:MEMORY_DEVICE_1_PRESENT=0\nE:MEMORY_DEVICE_1_SIZE=0\nE:MEMORY_DEVICE_1_SERIAL_NUMBER=FAKE-SERIAL\n"),
		"sys/block/nvme0n1/size":             file("3907029168\n"),
		"sys/block/nvme0n1/dev":              file("259:0\n"),
		"sys/block/nvme0n1/device/model":     file("Samsung SSD 990 PRO 2TB                 \n"),
		"sys/block/nvme0n1/device/serial":    file("FAKE-SERIAL\n"),
		"sys/block/nvme0n1/queue/rotational": file("0\n"),
		"sys/block/loop0/size":               file("100\n"),
		"sys/block/sda":                      &fstest.MapFile{Mode: fs.ModeSymlink, Data: []byte("../devices/pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0/block/sda")},
		"sys/devices/pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0/block/sda/size":             file("7814037168\n"),
		"sys/devices/pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0/block/sda/dev":              file("8:0\n"),
		"sys/devices/pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0/block/sda/queue/rotational": file("1\n"),
		"sys/devices/pci0000:00/0000:00:17.0/ata1/host0/target0:0:0/0:0:0:0/block/sda/device/model":     file("WDC WD40EFRX-68N\n"),
		"run/udev/data/b8:0": file("E:ID_MODEL_ENC=WDC\\x20WD40EFRX-68N32N0\\x20\\x20\nE:ID_SERIAL=FAKE-SERIAL\nE:ID_SERIAL_SHORT=FAKE-SERIAL\n"),
	}
	for cpu, topology := range map[string][2]string{"cpu0": {"0", "0"}, "cpu1": {"0", "0"}, "cpu2": {"0", "1"}, "cpu3": {"0", "2"}} {
		fsys["sys/devices/system/cpu/"+cpu+"/topology/physical_package_id"] = file(topology[0] + "\n")
		fsys["sys/devices/system/cpu/"+cpu+"/topology/core_id"] = file(topology[1] + "\n")
	}

	return fsys
}

func TestLinuxReadsItsOwnFilesAndNothingPrivate(t *testing.T) {
	h := ReadLinux(LinuxSource{FS: madeUpLinux(), Machine: "x86_64"})
	h.FinalNotes()
	text := h.Report().Text()
	assertClean(t, h, text)
	assertLines(t, text, "source=app/"+Version, "tested=yes", "os.family=linux", "os.name=debian", "os.version=13", "os.architecture=x86-64",
		"environment=physical", "cpu.vendor=AMD", "cpu.model=AMD Ryzen 7 7800X3D 8-Core Processor", "cpu.cores=3", "cpu.threads=4",
		"gpu.0.pci=10de:2204", "gpu.0.subsystem=1458:4037", "gpu.0.vendor=NVIDIA Corporation", "gpu.0.model=GA102 [GeForce RTX 3090]",
		"motherboard.vendor=ASUSTeK COMPUTER INC.", "motherboard.model=ROG STRIX X670E-E GAMING WIFI", "memory.total_mb=31890",
		"memory.0.size_mb=16384", "memory.0.maker=Corsair", "memory.0.part=CMK32GX5M2B6000C30", "memory.0.speed_mts=6000",
		"memory.0.configured_speed_mts=6000", "memory.0.type=DDR5", "memory.0.form=DIMM",
		"storage.0.model=WDC WD40EFRX-68N32N0", "storage.0.size_bytes=4000787030016", "storage.0.interface=sata", "storage.0.media=hdd",
		"storage.1.model=Samsung SSD 990 PRO 2TB", "storage.1.size_bytes=2000398934016", "storage.1.interface=nvme", "storage.1.media=ssd",
		"capability.memory_modules=available", "capability.configured_memory=available", "capability.displays.reason=not-collected")
	if strings.Contains(text, "memory.1.") || strings.Contains(text, "storage.2.") || len(h.Notes) != 0 {
		t.Fatalf("an empty slot, a loop device or a note came through:\n%s\n%v", text, h.Notes)
	}
}

// On ARM, /proc/cpuinfo names no processor: the system's own lscpu does, as for the script. Where the file names it, lscpu is not run.
func TestLinuxOnARMNamesTheProcessorThroughLscpu(t *testing.T) {
	arm := madeUpLinux()
	arm["proc/cpuinfo"] = &fstest.MapFile{Data: []byte("processor\t: 0\nBogoMIPS\t: 2000.00\nCPU implementer\t: 0x41\nCPU architecture: 8\n" +
		"CPU variant\t: 0x0\nCPU part\t: 0xd49\nCPU revision\t: 0\nSerial\t\t: FAKE-SERIAL\n")}
	var asked []string
	run := func(tool string, args ...string) []byte {
		asked = append(asked, strings.Join(append([]string{tool}, args...), " "))
		if tool != "lscpu" || len(args) != 0 {
			return nil
		}

		return []byte("Architecture:             aarch64\n  Byte Order:             Little Endian\nVendor ID:                ARM\n" +
			"  Model name:             Neoverse-N2\n    Model:                0\n    Thread(s) per core:   1\n")
	}
	h := ReadLinux(LinuxSource{FS: arm, Machine: "aarch64", Run: run})
	h.FinalNotes()
	text := h.Report().Text()
	assertClean(t, h, text)
	assertLines(t, text, "tested=no", "os.architecture=arm64", "cpu.vendor=ARM", "cpu.model=Neoverse-N2")
	if len(asked) != 1 || asked[0] != "lscpu" {
		t.Fatalf("lscpu should be asked once, with no arguments: %q", asked)
	}

	asked = nil
	if h := ReadLinux(LinuxSource{FS: madeUpLinux(), Machine: "x86_64", Run: run}); h.CPU.Model != "AMD Ryzen 7 7800X3D 8-Core Processor" || len(asked) != 0 {
		t.Fatalf("lscpu was run where /proc/cpuinfo names the processor: %q %+v", asked, h.CPU)
	}
}

func TestLinuxWithoutUdevNeedsRootForModulesAndReadsTheFirmwareAsRoot(t *testing.T) {
	fsys := madeUpLinux()
	delete(fsys, "run/udev/data/+dmi:id")
	delete(fsys, "run/udev/data/b8:0")
	delete(fsys, "usr/share/misc/pci.ids")
	h := ReadLinux(LinuxSource{FS: fsys, Machine: "x86_64"})
	h.FinalNotes()
	text := h.Report().Text()
	assertClean(t, h, text)
	assertLines(t, text, "tested=no", "capability.memory_modules=unavailable", "capability.memory_modules.reason=needs-root",
		"storage.0.model=WDC WD40EFRX-68N", "gpu.0.pci=10de:2204")
	if !strings.Contains(strings.Join(h.Notes, " "), "run the app again with sudo") || strings.Contains(text, "gpu.0.model") {
		t.Fatalf("notes %v, report:\n%s", h.Notes, text)
	}

	fsys["sys/firmware/dmi/tables/DMI"] = &fstest.MapFile{Data: smbiostest.MadeUpTable(), Mode: 0o400}
	root := ReadLinux(LinuxSource{FS: fsys, Machine: "x86_64", Root: true})
	text = root.Report().Text()
	assertClean(t, root, text)
	assertLines(t, text, "memory.0.size_mb=32768", "memory.0.maker=Corsair", "memory.0.part=CMK64GX5M2B6000C30", "memory.0.type=DDR5")
}

func TestLinuxTellsAVirtualMachineAContainerAndWSL(t *testing.T) {
	vm := madeUpLinux()
	vm["proc/cpuinfo"] = file("processor\t: 0\nvendor_id\t: GenuineIntel\nmodel name\t: QEMU Virtual CPU\nflags\t\t: fpu hypervisor\n")
	h := ReadLinux(LinuxSource{FS: vm, Machine: "x86_64"})
	h.FinalNotes()
	if h.Environment != "virtual_machine" || !strings.Contains(strings.Join(h.Notes, " "), "run the app on the host") {
		t.Fatalf("virtual machine: %s %v", h.Environment, h.Notes)
	}
	box := madeUpLinux()
	box["run/systemd/container"] = file("lxc\n")
	delete(box, "run/udev/data/+dmi:id")
	if h := ReadLinux(LinuxSource{FS: box, Machine: "x86_64"}); h.Environment != "container" || h.ModulesReason != "not-exposed" {
		t.Fatalf("container: %s %s", h.Environment, h.ModulesReason)
	}
	wsl := madeUpLinux()
	wsl["proc/sys/kernel/osrelease"] = file("6.6.87.2-microsoft-standard-WSL2\n")
	if h := ReadLinux(LinuxSource{FS: wsl, Machine: "x86_64"}); h.Environment != "wsl" || h.CPU.Model != "" {
		t.Fatalf("WSL should stop before reading: %+v", h)
	}
}

func TestWindowsReadsTheFirmwareAndDevicesWithoutPrivateParts(t *testing.T) {
	no, yes := false, true
	src := WindowsSource{
		Firmware: smbiostest.MadeUpTable(), CPUName: "13th Gen Intel(R) Core(TM) i9-13900KF  ", CPUVendor: "GenuineIntel", LogicalProcessors: 32,
		Adapters: []WindowsAdapter{
			{InstanceID: `PCI\VEN_10DE&DEV_2782&SUBSYS_88A71043&REV_A1\4&FAKE-UNIQUE&0&0008`, Name: "NVIDIA GeForce RTX 4070 Ti"},
			{InstanceID: `ROOT\DISPLAY\0000`, Name: "Remote Display Adapter"},
		},
		Disks: []WindowsDisk{
			{Model: "Samsung SSD 980 PRO 1TB", Bytes: 1000204886016, Bus: 17, SeekPenalty: &no},
			{Model: "Msft Virtual Disk", Bytes: 1 << 30, Bus: 14},
			{Model: "Portable SSD T5", Bytes: 500107862016, Bus: 7, SeekPenalty: &yes},
		},
		TotalMemory: 68464050176, Version: "10.0.26100", Architecture: "AMD64",
	}
	h := ReadWindows(src)
	h.FinalNotes()
	text := h.Report().Text()
	assertClean(t, h, text)
	assertLines(t, text, "tested=yes", "os.version=10.0.26100", "os.architecture=x86-64", "environment=physical", "cpu.vendor=Intel",
		"cpu.model=13th Gen Intel(R) Core(TM) i9-13900KF", "cpu.cores=8", "cpu.threads=16", "gpu.0.pci=10de:2782", "gpu.0.subsystem=1043:88a7",
		"gpu.0.model=NVIDIA GeForce RTX 4070 Ti", "motherboard.model=ROG STRIX X670E-E", "memory.total_mb=65292", "memory.0.size_mb=32768",
		"storage.0.interface=nvme", "storage.0.media=ssd", "storage.1.model=Portable SSD T5", "storage.1.interface=usb")
	if strings.Contains(text, "gpu.1.") || strings.Contains(text, "storage.2.") || strings.Contains(text, "storage.1.media") {
		t.Fatalf("a virtual adapter or disk, or a USB disk's media, came through:\n%s", text)
	}

	src.Firmware = nil
	if h := ReadWindows(src); h.Environment != "unknown" || h.Board.Model != "" || len(h.Notes) == 0 {
		t.Fatalf("without the firmware table: %+v", h)
	}
}

func TestMacOSReadsByExactKey(t *testing.T) {
	outputs := map[string]string{
		"system_profiler SPDisplaysDataType": "Graphics/Displays:\n\n    Apple M2:\n\n      Chipset Model: Apple M2\n      Type: GPU\n" +
			"      Vendor: Apple (0x106b)\n      Device ID: 0x0000\n      Displays:\n        Color LCD:\n          Display Serial Number: FAKE-SERIAL\n",
		"system_profiler SPMemoryDataType": "Memory:\n\n      Memory: 16 GB\n      Type: LPDDR5\n      Manufacturer: Hynix\n      Serial Number: FAKE-SERIAL\n",
		"diskutil list physical":           "/dev/disk0 (internal, physical):\n   #: TYPE NAME SIZE IDENTIFIER\n",
		"diskutil info disk0": "   Device / Media Name:      APPLE SSD AP0512Z\n   Disk Size:                500.3 GB (500277790720 Bytes) (exactly 977105060 512-Byte-Units)\n" +
			"   Protocol:                 Apple Fabric\n   Solid State:              Yes\n   Disk / Partition UUID:    12345678-1234-1234-1234-123456789abc\n",
	}
	sysctls := map[string]string{"kern.osproductversion": "15.1", "hw.optional.arm64": "1", "kern.hv_vmm_present": "0", "machdep.cpu.brand_string": "Apple M2",
		"hw.physicalcpu": "8", "hw.logicalcpu": "8", "hw.model": "Mac14,2", "hw.memsize": "17179869184"}
	h := ReadMacOS(UnixSource{Sysctl: func(name string) string { return sysctls[name] },
		Run: func(tool string, args ...string) []byte { return []byte(outputs[tool+" "+strings.Join(args, " ")]) }, Machine: "arm64"})
	text := h.Report().Text()
	assertClean(t, h, text)
	assertLines(t, text, "os.family=macos", "os.version=15.1", "os.architecture=arm64", "environment=physical", "cpu.vendor=Apple", "cpu.model=Apple M2",
		"gpu.0.pci=106b:0000", "gpu.0.vendor=Apple", "gpu.0.model=Apple M2", "motherboard.vendor=Apple", "motherboard.model=Mac14,2",
		"memory.total_mb=16384", "memory.0.size_mb=16384", "memory.0.maker=Hynix", "memory.0.type=LPDDR5", "storage.0.model=APPLE SSD AP0512Z",
		"storage.0.size_bytes=500277790720", "storage.0.interface=apple-fabric", "storage.0.media=ssd", "capability.configured_memory.reason=not-exposed")
}

func TestFreeBSDReadsByExactKeyAndNeedsRootForModules(t *testing.T) {
	outputs := map[string]string{
		"freebsd-version -u": "14.1-RELEASE-p5\n",
		"pciconf -l": "vgapci0@pci0:1:0:0:\tclass=0x030000 rev=0xa1 hdr=0x00 vendor=0x10de device=0x2204 subvendor=0x1458 subdevice=0x4037\n" +
			"xhci0@pci0:0:20:0:\tclass=0x0c0330 vendor=0x8086 device=0x7ae0\n",
		"kenv -q smbios.planar.maker":   "ASUSTeK COMPUTER INC.\n",
		"kenv -q smbios.planar.product": "PRIME B550M-A\n",
		"geom disk list": "Geom name: nda0\nProviders:\n1. Name: nda0\n   Mediasize: 1000204886016 (932G)\n   descr: WD Blue SN570 1TB\n" +
			"   ident: FAKE-SERIAL\n   rotationrate: 0\n\nGeom name: cd0\n   Mediasize: 0 (0B)\n",
	}
	sysctls := map[string]string{"kern.vm_guest": "none", "hw.model": "AMD Ryzen 5 5600X 6-Core Processor", "kern.smp.cores": "6", "hw.ncpu": "12",
		"hw.physmem": "34359738368"}
	h := ReadFreeBSD(UnixSource{Sysctl: func(name string) string { return sysctls[name] },
		Run: func(tool string, args ...string) []byte { return []byte(outputs[tool+" "+strings.Join(args, " ")]) }, Machine: "amd64"})
	h.FinalNotes()
	text := h.Report().Text()
	assertClean(t, h, text)
	assertLines(t, text, "os.version=14.1-RELEASE-p5", "environment=physical", "cpu.vendor=AMD", "cpu.cores=6", "cpu.threads=12", "gpu.0.pci=10de:2204",
		"gpu.0.subsystem=1458:4037", "motherboard.model=PRIME B550M-A", "memory.total_mb=32768", "storage.0.model=WD Blue SN570 1TB",
		"storage.0.interface=nvme", "storage.0.media=ssd", "capability.memory_modules.reason=needs-root", "tested=no")
	if strings.Contains(text, "gpu.1.") || strings.Contains(text, "storage.1.") {
		t.Fatalf("a non-display device or the CD drive came through:\n%s", text)
	}
}
