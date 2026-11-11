#!/usr/bin/env python3
"""Tests scripts/mysetup-scan.sh on made-up computers: python3 tests/test_scan_sh.py

Each computer is a directory standing in for / with fake commands in bin/. PATH holds only those, so the
real computer is never asked and any tool the script needs but does not declare fails the test. Every source
also holds planted values the report must never carry. The tests run under every shell and awk found here.
"""

from __future__ import annotations

import os
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
from validate_report import problems  # noqa: E402

SCRIPT = HERE.parent / "scripts" / "mysetup-scan.sh"

# Shared with the fake sources below. None may ever reach the report or the notes.
PLANTED = [
    "FAKE-SERIAL",
    "FAKE-HOST",
    "FAKE-USER",
    "FAKE-UUID",
    "FAKE-TAG",
    "FAKE-LOCATOR",
    "FAKE-WWN",
    "de:ad:be:ef:00:01",
    "10.20.30.40",
    "12345678-1234-1234-1234-123456789abc",
]

REQUIRED_TOOLS = ["awk", "tr", "id"]


def available_shells() -> list[list[str]]:
    shells = []
    for shell in (["dash"], ["bash", "--posix"], ["busybox", "sh"], ["zsh", "--emulate", "sh"], ["ksh"], ["mksh"], ["yash"]):
        path = shutil.which(shell[0])
        if path:
            shells.append([path, *shell[1:]])
    return shells


def runs_own_uname(shell: list[str]) -> bool:
    """Whether a shell runs a uname of its own whatever PATH says, as BusyBox's does with its applet, so that a made-up computer
    cannot change what uname answers there: it says the processor of the machine running the test."""
    with tempfile.TemporaryDirectory(prefix="mysetup-scan-test-") as folder:
        made_up = Path(folder) / "uname"
        made_up.write_text("#!/bin/sh\necho made-up\n")
        made_up.chmod(0o755)
        environment = {**os.environ, "PATH": folder + os.pathsep + os.environ.get("PATH", "")}
        answer = subprocess.run([*shell, "-c", "uname -m"], capture_output=True, text=True, env=environment, check=False).stdout
    return answer.strip() != "made-up"


def available_awks() -> list[str]:
    # "awk" is whatever this system calls by that name, such as the one macOS ships.
    return [awk for awk in ("awk", "mawk", "gawk", "original-awk", "busybox-awk") if shutil.which(awk.replace("busybox-awk", "busybox"))]


class Computer:
    """A made-up computer: files under a temporary root, commands as small shell scripts."""

    def __init__(self, awk: str | None = None):
        self.dir = tempfile.TemporaryDirectory(prefix="mysetup-scan-test-")
        self.root = Path(self.dir.name)
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.awk = awk or next(iter(available_awks()), "awk")
        for tool in REQUIRED_TOOLS:
            self.link(tool)
        self.command("uname", {"-s": "Linux", "-m": "x86_64", "-r": "6.1.0"}, default="FAKE-HOST FAKE-USER")

    def link(self, tool: str) -> None:
        if tool == "awk":
            if self.awk == "busybox-awk":
                (self.bin / "awk").symlink_to(shutil.which("busybox"))
            else:
                (self.bin / "awk").symlink_to(shutil.which(self.awk))
        else:
            (self.bin / tool).symlink_to(shutil.which(tool))

    def file(self, path: str, content: str | bytes) -> None:
        target = self.root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        if isinstance(content, bytes):
            target.write_bytes(content)
        else:
            target.write_text(content)

    def command(self, name: str, answers: dict[str, str], default: str = "", status: int = 0) -> None:
        """Answers by the exact arguments, else by the default, which plants values by default too."""
        lines = ["#!/bin/sh", 'case "$*" in']
        for arguments, output in answers.items():
            lines.append(f"    {shlex.quote(arguments)}) printf '%s\\n' {shlex.quote(output)}; exit 0 ;;")
        lines.append("esac")
        if default:
            lines.append(f"printf '%s\\n' {shlex.quote(default)}")
        lines.append(f"exit {status}")
        path = self.bin / name
        path.unlink(missing_ok=True)  # Never through a link to the real tool.
        path.write_text("\n".join(lines) + "\n")
        path.chmod(0o755)

    def run(self, *arguments: str, shell: list[str] | None = None) -> subprocess.CompletedProcess:
        before = self.snapshot()
        shell = shell or available_shells()[0]
        result = subprocess.run(
            [*shell, str(SCRIPT), *arguments],
            env={"MYSETUP_SCAN_TEST_ROOT": str(self.root), "HOME": "/nonexistent", "PATH": "/nonexistent"},
            capture_output=True,
            text=True,
            cwd=self.root,
            timeout=60,
        )
        if self.snapshot() != before:
            raise AssertionError("the script changed files")
        return result

    def snapshot(self) -> dict[str, float]:
        return {str(path): path.stat().st_mtime for path in self.root.rglob("*") if not path.is_symlink()}

    def close(self) -> None:
        self.dir.cleanup()


def sysfs_cpus(computer: Computer, pairs: list[tuple[int, int]]) -> None:
    for number, (package, core) in enumerate(pairs):
        computer.file(f"sys/devices/system/cpu/cpu{number}/topology/physical_package_id", f"{package}\n")
        computer.file(f"sys/devices/system/cpu/cpu{number}/topology/core_id", f"{core}\n")


def linux_common(computer: Computer) -> None:
    computer.file("proc/sys/kernel/osrelease", "6.1.0-18-amd64\n")
    computer.file("etc/os-release", 'PRETTY_NAME="Debian GNU/Linux 12 (FAKE-HOST)"\nID=debian\nID_LIKE=other\nVERSION_ID="12"\nHOME_URL="https://10.20.30.40/"\n')
    computer.file("proc/meminfo", "MemTotal:       32768000 kB\nMemFree:        1 kB\n")
    computer.file(
        "proc/cpuinfo",
        "processor\t: 0\nvendor_id\t: GenuineIntel\nmodel name\t: Intel(R) Core(TM) i7-7700K CPU @ 4.20GHz\nflags\t\t: fpu sse\nSerial\t\t: FAKE-SERIAL\n",
    )
    # Files the kernel keeps for root in real life. They are here so a read of them would be caught.
    for name in ("board_serial", "product_serial", "chassis_serial", "product_uuid"):
        computer.file(f"sys/class/dmi/id/{name}", "FAKE-SERIAL\n")
    computer.file("sys/class/dmi/id/board_vendor", "ASUSTeK COMPUTER INC.\n")
    computer.file("sys/class/dmi/id/board_name", "PRIME Z270-K\n")
    computer.file("sys/class/dmi/id/product_name", "FAKE-HOST\n")
    computer.file("etc/hostname", "FAKE-HOST\n")
    computer.command("hostname", {}, default="FAKE-HOST")
    computer.command("whoami", {}, default="FAKE-USER")
    computer.command("ip", {}, default="inet 10.20.30.40 link/ether de:ad:be:ef:00:01")


LSCPU = (
    "Architecture:            x86_64\n"
    "Vendor ID:               GenuineIntel\n"
    "BIOS Model name:         FAKE-SERIAL\n"
    "Model name:              Intel(R) Core(TM) i7-7700K CPU @ 4.20GHz\n"
    "Core(s) per socket:      99\n"
)
UDEV = (
    "MEMORY_ARRAY_LOCATION=FAKE-LOCATOR\n"
    "MEMORY_DEVICE_0_LOCATOR=FAKE-LOCATOR\n"
    "MEMORY_DEVICE_0_SERIAL_NUMBER=FAKE-SERIAL\n"
    "MEMORY_DEVICE_0_ASSET_TAG=FAKE-TAG\n"
    "MEMORY_DEVICE_0_SIZE=17179869184\n"
    "MEMORY_DEVICE_0_MANUFACTURER=Corsair\n"
    "MEMORY_DEVICE_0_PART_NUMBER=CMK32GX4M2A2400C14  \n"
    "MEMORY_DEVICE_0_SPEED_MTS=2400\n"
    "MEMORY_DEVICE_0_CONFIGURED_SPEED_MTS=2133\n"
    "MEMORY_DEVICE_0_TYPE=DDR4\n"
    "MEMORY_DEVICE_0_FORM_FACTOR=DIMM\n"
    "MEMORY_DEVICE_1_LOCATOR=FAKE-LOCATOR\n"
    "MEMORY_DEVICE_1_PART_NUMBER=NO DIMM\n"
    "MEMORY_DEVICE_1_MANUFACTURER=NO DIMM\n"
    "MEMORY_DEVICE_2_SIZE=17179869184\n"
    "MEMORY_DEVICE_2_MANUFACTURER=Unknown\n"
    "MEMORY_DEVICE_2_PART_NUMBER=Array1_PartNumber0\n"
    "MEMORY_DEVICE_2_SERIAL_NUMBER=FAKE-SERIAL\n"
    "MEMORY_DEVICE_2_TYPE=Other\n"
    "MEMORY_DEVICE_2_FORM_FACTOR=SODIMM\n"
)
DMIDECODE = """# dmidecode 3.5
Getting SMBIOS data from sysfs.

Handle 0x0040, DMI type 17, 40 bytes
Memory Device
	Array Handle: 0x003F
	Size: 16 GB
	Form Factor: DIMM
	Locator: FAKE-LOCATOR
	Bank Locator: FAKE-LOCATOR
	Type: DDR4
	Speed: 3200 MT/s
	Manufacturer: G Skill Intl
	Serial Number: FAKE-SERIAL
	Asset Tag: FAKE-TAG
	Part Number: F4-3200C16-16GTZ
	Configured Memory Speed: 3000 MT/s

Handle 0x0041, DMI type 17, 40 bytes
Memory Device
	Size: No Module Installed
	Locator: FAKE-LOCATOR
	Manufacturer: Not Specified
	Part Number: Not Specified

Handle 0x0042, DMI type 17, 40 bytes
Memory Device
	Size: 8192 MB
	Form Factor: SODIMM
	Type: DDR3
	Speed: 1600 MHz
	Manufacturer: Kingston
	Serial Number: FAKE-SERIAL
	Part Number: KVR16S11/8
	Configured Clock Speed: 1333 MHz
"""


def pci_device(computer: Computer, address: str, device_class: str, vendor: str, device: str, svendor: str, sdevice: str) -> None:
    folder = f"sys/bus/pci/devices/{address}"
    computer.file(f"{folder}/class", f"{device_class}\n")
    computer.file(f"{folder}/vendor", f"0x{vendor}\n")
    computer.file(f"{folder}/device", f"0x{device}\n")
    computer.file(f"{folder}/subsystem_vendor", f"0x{svendor}\n")
    computer.file(f"{folder}/subsystem_device", f"0x{sdevice}\n")


def lspci_answers() -> dict[str, str]:
    return {
        "-vmm -nn -s 0000:01:00.0": "Slot:\t01:00.0\nClass:\tVGA compatible controller [0300]\nVendor:\tNVIDIA Corporation [10de]\n"
        "Device:\tGP102 [GeForce GTX 1080 Ti] [1b06]\nSVendor:\tASUSTeK Computer Inc. [1043]\nSDevice:\tFAKE-SERIAL [85ea]\n"
        "IOMMUGroup:\t1\n",
    }


LSBLK = (
    'NAME="sda" TYPE="disk" MODEL="WDC WD40EFRX-68N32N0  " SIZE="4000787030016" TRAN="sata" ROTA="1"\n'
    'NAME="nvme0n1" TYPE="disk" MODEL="Samsung SSD 970 EVO 1TB" SIZE="1000204886016" TRAN="nvme" ROTA="0"\n'
    'NAME="sdb" TYPE="disk" MODEL="USB Flash Disk" SIZE="32000000000" TRAN="usb" ROTA="1"\n'
    'NAME="zd0" TYPE="disk" MODEL="" SIZE="107374182400" TRAN="" ROTA="1"\n'
    'NAME="sr0" TYPE="rom" MODEL="DVD" SIZE="1073741312" TRAN="sata" ROTA="1"\n'
    'NAME="loop0" TYPE="loop" MODEL="" SIZE="4096" TRAN="" ROTA="0"\n'
    'NAME="sdc" TYPE="disk" MODEL="Empty Card Reader" SIZE="0" TRAN="usb" ROTA="1"\n'
)
LSBLK_ARGUMENTS = "-d -b -n -P -o NAME,TYPE,MODEL,SIZE,TRAN,ROTA"


def desktop(awk: str | None = None) -> Computer:
    """A physical Debian desktop with everything a systemd host has."""
    computer = Computer(awk)
    linux_common(computer)
    sysfs_cpus(computer, [(0, 0), (0, 1), (0, 2), (0, 3), (0, 0), (0, 1), (0, 2), (0, 3)])
    pci_device(computer, "0000:00:1f.3", "0x040300", "8086", "a2f0", "1043", "8694")
    pci_device(computer, "0000:01:00.0", "0x030000", "10de", "1b06", "1043", "85ea")
    computer.command("systemd-detect-virt", {}, default="none", status=1)
    computer.command("lscpu", {}, default=LSCPU)
    computer.command("lspci", lspci_answers())
    computer.command("udevadm", {"info -q property -p /devices/virtual/dmi/id": UDEV})
    computer.command("lsblk", {LSBLK_ARGUMENTS: LSBLK}, default="FAKE-SERIAL FAKE-WWN")
    return computer


DESKTOP_REPORT = """mysetup-report 1
source=script-sh/0.1.0
tested=yes
os.family=linux
os.name=debian
os.version=12
os.architecture=x86-64
environment=physical
cpu.vendor=Intel
cpu.model=Intel(R) Core(TM) i7-7700K CPU @ 4.20GHz
cpu.cores=4
cpu.threads=8
gpu.0.pci=10de:1b06
gpu.0.subsystem=1043:85ea
gpu.0.vendor=NVIDIA Corporation
gpu.0.model=GP102 [GeForce GTX 1080 Ti]
motherboard.vendor=ASUSTeK COMPUTER INC.
motherboard.model=PRIME Z270-K
memory.total_mb=32000
memory.0.size_mb=16384
memory.0.maker=Corsair
memory.0.part=CMK32GX4M2A2400C14
memory.0.speed_mts=2400
memory.0.configured_speed_mts=2133
memory.0.type=DDR4
memory.0.form=DIMM
memory.1.size_mb=16384
memory.1.form=SODIMM
storage.0.model=WDC WD40EFRX-68N32N0
storage.0.size_bytes=4000787030016
storage.0.interface=sata
storage.0.media=hdd
storage.1.model=Samsung SSD 970 EVO 1TB
storage.1.size_bytes=1000204886016
storage.1.interface=nvme
storage.1.media=ssd
storage.2.model=USB Flash Disk
storage.2.size_bytes=32000000000
storage.2.interface=usb
capability.cpu=available
capability.gpu=available
capability.motherboard=available
capability.memory_modules=available
capability.configured_memory=available
capability.storage=available
capability.displays=unavailable
capability.displays.reason=not-collected
mysetup-report-end
"""


def minimal_linux(awk: str | None = None) -> Computer:
    """Alpine or a NAS: no systemd, no util-linux, no pciutils, only /sys and /proc."""
    computer = Computer(awk)
    linux_common(computer)
    computer.file("etc/os-release", "ID=alpine\nVERSION_ID=3.20.1\n")
    computer.file("proc/cpuinfo", "processor : 0\nvendor_id : AuthenticAMD\nmodel name : AMD Ryzen 5 5600G with Radeon Graphics\nflags : fpu sse\nprocessor : 1\n")
    sysfs_cpus(computer, [(0, 0), (0, 0)])
    pci_device(computer, "0000:05:00.0", "0x030000", "1002", "1638", "1458", "d000")
    computer.file("sys/block/sda/size", "7814037168\n")
    computer.file("sys/block/sda/device/model", "ST4000DM004-2CV1\n")
    computer.file("sys/block/sda/device/serial", "FAKE-SERIAL\n")
    computer.file("sys/block/sda/device/wwid", "FAKE-WWN\n")
    computer.file("sys/block/sda/queue/rotational", "1\n")
    computer.file("sys/block/nvme0n1/size", "1953525168\n")
    computer.file("sys/block/nvme0n1/device/model", "WD Blue SN570\n")
    computer.file("sys/block/nvme0n1/device/serial", "FAKE-SERIAL\n")
    computer.file("sys/block/nvme0n1/queue/rotational", "0\n")
    computer.file("sys/block/loop0/size", "2048\n")
    computer.file("sys/block/zram0/size", "2048\n")
    computer.file("sys/block/mmcblk0boot0/size", "2048\n")
    computer.command("apk", {})
    return computer


def raspberry_pi(awk: str | None = None) -> Computer:
    computer = Computer(awk)
    computer.command("uname", {"-s": "Linux", "-m": "aarch64"})
    computer.file("proc/sys/kernel/osrelease", "6.6.31+rpt-rpi-v8\n")
    computer.file("etc/os-release", "ID=debian\nVERSION_ID=12\n")
    computer.file("proc/meminfo", "MemTotal: 3884000 kB\n")
    computer.file("proc/cpuinfo", "processor : 0\nFeatures : fp asimd\nHardware : BCM2835\nModel : FAKE-HOST\nSerial : FAKE-SERIAL\n")
    computer.file("proc/device-tree/model", b"Raspberry Pi 4 Model B Rev 1.4\x00")
    computer.file("proc/device-tree/serial-number", b"FAKE-SERIAL\x00")
    sysfs_cpus(computer, [(0, 0), (0, 1), (0, 2), (0, 3)])
    computer.file("sys/class/drm/card0/device/uevent", "DRIVER=vc4-drm\nOF_NAME=gpu\nOF_COMPATIBLE_0=brcm,bcm2711-vc5\nOF_COMPATIBLE_N=1\n")
    computer.command("lscpu", {}, default="Vendor ID:  ARM\nModel name:  Cortex-A72\n")
    computer.command("systemd-detect-virt", {}, default="none", status=1)
    computer.command("udevadm", {})
    computer.command(
        "lsblk",
        {LSBLK_ARGUMENTS: 'NAME="mmcblk0" TYPE="disk" MODEL="" SIZE="63864569856" TRAN="" ROTA="0"\nNAME="mmcblk0boot0" TYPE="disk" MODEL="" SIZE="4194304" TRAN="" ROTA="0"\n'},
    )
    computer.file("sys/block/mmcblk0/device/name", "SC64G\n")
    computer.file("sys/block/mmcblk0/device/serial", "FAKE-SERIAL\n")
    return computer


def mac_intel(awk: str | None = None) -> Computer:
    computer = Computer(awk)
    computer.command("uname", {"-s": "Darwin", "-m": "x86_64"})
    computer.command(
        "sysctl",
        {
            "-n machdep.cpu.brand_string": "Intel(R) Core(TM) i7-9750H CPU @ 2.60GHz",
            "-n hw.physicalcpu": "6",
            "-n hw.logicalcpu": "12",
            "-n hw.model": "MacBookPro16,1",
            "-n hw.memsize": "17179869184",
            "-n kern.hv_vmm_present": "0",
            "-n hw.optional.arm64": "",
        },
        status=1,
    )
    computer.command("sw_vers", {"-productVersion": "14.5"}, default="FAKE-HOST")
    computer.command(
        "system_profiler",
        {
            "SPDisplaysDataType": "Graphics/Displays:\n\n    Intel UHD Graphics 630:\n\n      Chipset Model: Intel UHD Graphics 630\n      Type: GPU\n"
            "      Vendor: Intel (0x8086)\n      Device ID: 0x3e9b\n      Displays:\n        Color LCD:\n          Display Serial Number: FAKE-SERIAL\n\n"
            "    AMD Radeon Pro 5300M:\n\n      Chipset Model: AMD Radeon Pro 5300M\n      Vendor: AMD (0x1002)\n      Device ID: 0x7340\n",
            "SPMemoryDataType": "Memory:\n\n    Memory Slots:\n\n      ECC: Disabled\n      Upgradeable Memory: No\n\n        BANK 0/ChannelA-DIMM0:\n\n"
            "          Size: 8 GB\n          Type: DDR4\n          Speed: 2667 MHz\n          Status: OK\n          Manufacturer: 0x80AD\n"
            "          Part Number: 0x484D\n          Serial Number: FAKE-SERIAL\n\n        BANK 2/ChannelB-DIMM0:\n\n          Size: Empty\n          Type: Empty\n",
        },
        default="Serial Number (system): FAKE-SERIAL\nHardware UUID: FAKE-UUID\n",
    )
    computer.command(
        "diskutil",
        {
            "list physical": "/dev/disk0 (internal, physical):\n   #: TYPE NAME SIZE IDENTIFIER\n   0: GUID_partition_scheme *1.0 TB disk0\n/dev/disk3 (external, physical):\n",
            "info /dev/disk0": "   Device Identifier:  disk0\n   Device / Media Name:  APPLE SSD AP1024N\n   Protocol:  PCI-Express\n   Solid State:  Yes\n"
            "   Disk Size:  1.0 TB (1000555581440 Bytes) (exactly 1954210120 512-Byte-Units)\n   Disk / Partition UUID:  FAKE-UUID\n",
            "info /dev/disk3": "   Device / Media Name:  Portable Disk\n   Protocol:  USB\n   Disk Size:  500.1 GB (500107862016 Bytes)\n",
        },
    )
    computer.command("hostname", {}, default="FAKE-HOST")
    return computer


def mac_apple_silicon(awk: str | None = None) -> Computer:
    computer = Computer(awk)
    computer.command("uname", {"-s": "Darwin", "-m": "x86_64"})  # A shell under Rosetta says x86_64.
    computer.command(
        "sysctl",
        {
            "-n machdep.cpu.brand_string": "Apple M2 Pro",
            "-n hw.physicalcpu": "10",
            "-n hw.logicalcpu": "10",
            "-n hw.model": "Mac14,12",
            "-n hw.memsize": "17179869184",
            "-n kern.hv_vmm_present": "1",
            "-n hw.optional.arm64": "1",
        },
    )
    computer.command("sw_vers", {"-productVersion": "15.1"})
    computer.command(
        "system_profiler",
        {
            "SPDisplaysDataType": "Graphics/Displays:\n\n    Apple M2 Pro:\n\n      Chipset Model: Apple M2 Pro\n      Vendor: sppci_vendor_Apple\n",
            "SPMemoryDataType": "Memory:\n\n      Memory: 16 GB\n      Type: LPDDR5\n      Manufacturer: Samsung\n",
        },
    )
    computer.command("diskutil", {})
    return computer


def freebsd(awk: str | None = None) -> Computer:
    computer = Computer(awk)
    computer.command("uname", {"-s": "FreeBSD", "-m": "amd64"})
    computer.command(
        "sysctl",
        {
            "-n hw.model": "AMD Ryzen 7 5800X 8-Core Processor",
            "-n kern.smp.cores": "8",
            "-n hw.ncpu": "16",
            "-n hw.physmem": "34359738368",
            "-n kern.vm_guest": "none",
            "-n security.jail.jailed": "0",
        },
    )
    computer.command("freebsd-version", {"-u": "14.1-RELEASE"})
    computer.command(
        "pciconf",
        {
            "-l": "hostb0@pci0:0:0:0: class=0x060000 vendor=0x1022 device=0x1480 subvendor=0x1022 subdevice=0x1480\n"
            "vgapci0@pci0:9:0:0: class=0x030000 rev=0xa1 hdr=0x00 vendor=0x10de device=0x2204 subvendor=0x1458 subdevice=0x4037\n"
            "vgapci1@pci0:10:0:0: class=0x030000 card=0x40371458 chip=0x22041 rev=0xa1\n"
        },
    )
    computer.command(
        "kenv",
        {"-q smbios.planar.maker": "Gigabyte Technology Co., Ltd.", "-q smbios.planar.product": "X570 AORUS ELITE", "-q smbios.planar.serial": "FAKE-SERIAL"},
    )
    computer.command(
        "geom",
        {
            "disk list": "Geom name: nda0\nProviders:\n1. Name: nda0\n   Mediasize: 1000204886016 (932G)\n   descr: Samsung SSD 980 1TB\n   lunid: FAKE-WWN\n   ident: FAKE-SERIAL\n   rotationrate: 0\n\n"
            "Geom name: ada0\nProviders:\n1. Name: ada0\n   Mediasize: 4000787030016 (3.6T)\n   descr: WDC WD40EFRX\n   ident: FAKE-SERIAL\n   rotationrate: 5400\n\n"
            "Geom name: cd0\nProviders:\n1. Name: cd0\n   descr: DVD\n"
        },
    )
    computer.command("pkg", {})
    return computer


class Report:
    def __init__(self, stdout: str):
        self.text = stdout
        self.fields = dict(line.split("=", 1) for line in stdout.splitlines() if "=" in line)


class Base(unittest.TestCase):
    def setUp(self) -> None:
        self.computers: list[Computer] = []

    def tearDown(self) -> None:
        for computer in self.computers:
            computer.close()

    def keep(self, computer: Computer) -> Computer:
        self.computers.append(computer)
        return computer

    def scan(self, computer: Computer, *arguments: str, shell: list[str] | None = None) -> subprocess.CompletedProcess:
        result = self.keep(computer).run(*arguments, shell=shell)
        for value in PLANTED:
            self.assertNotIn(value, result.stdout, f"the report carries {value}")
            self.assertNotIn(value, result.stderr, f"the notes carry {value}")
        return result


class Reports(Base):
    def test_desktop_report_is_exact_under_every_shell_and_awk(self) -> None:
        for shell in available_shells():
            # The made-up desktop is x86_64. A shell with a uname of its own reports this machine's processor instead, so on any
            # other processor (GitHub's ARM runner) its report cannot be the made-up one; x86-64 runners still check it in full.
            own_uname = runs_own_uname(shell) and os.uname().machine != "x86_64"
            for awk in available_awks():
                with self.subTest(shell=shell[0], awk=awk):
                    if own_uname:
                        self.skipTest(f"{shell[0]} runs its own uname, which says this {os.uname().machine} machine, not the made-up x86_64")
                    result = self.scan(desktop(awk), shell=shell)
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(result.stdout, DESKTOP_REPORT)
                    self.assertEqual(problems(result.stdout), [])

    def test_hybrid_cpu_counts_real_cores(self) -> None:
        computer = desktop()
        for number in range(8, 12):  # Four more threads on four cores of their own, as efficiency cores are.
            computer.file(f"sys/devices/system/cpu/cpu{number}/topology/physical_package_id", "0\n")
            computer.file(f"sys/devices/system/cpu/cpu{number}/topology/core_id", f"{number + 8}\n")
        report = Report(self.scan(computer).stdout)
        self.assertEqual(report.fields["cpu.cores"], "8")
        self.assertEqual(report.fields["cpu.threads"], "12")

    def test_minimal_linux_uses_sys_and_says_what_it_would_take(self) -> None:
        result = self.scan(minimal_linux())
        self.assertEqual(problems(result.stdout), [])
        report = Report(result.stdout)
        self.assertEqual(report.fields["tested"], "no")
        self.assertEqual(report.fields["cpu.model"], "AMD Ryzen 5 5600G with Radeon Graphics")
        self.assertEqual(report.fields["cpu.threads"], "2")
        self.assertEqual(report.fields["cpu.cores"], "1")
        self.assertEqual(report.fields["gpu.0.pci"], "1002:1638")
        self.assertNotIn("gpu.0.model", report.fields)
        models = {report.fields["storage.0.model"], report.fields["storage.1.model"]}
        self.assertEqual(models, {"ST4000DM004-2CV1", "WD Blue SN570"})
        self.assertNotIn("storage.2.model", report.fields)
        self.assertEqual(report.fields["capability.memory_modules"], "unavailable")
        self.assertEqual(report.fields["capability.memory_modules.reason"], "needs-root")
        self.assertIn("sudo apk add dmidecode", result.stderr.replace("doas", "sudo"))
        self.assertIn("apk add pciutils", result.stderr)
        self.assertIn("--with-root", result.stderr)
        self.assertIn("Untested on this system", result.stderr)

    def test_with_root_reads_modules_through_dmidecode_only_when_asked(self) -> None:
        computer = minimal_linux()
        computer.command("dmidecode", {"-t 17": DMIDECODE}, default="FAKE-SERIAL")
        computer.command("id", {"-u": "1000"})
        marker = computer.root / "sudo-was-used"
        sudo = computer.bin / "sudo"
        sudo.write_text(f'#!/bin/sh\nprintf x > {shlex.quote(str(marker))}\nexec "$@"\n')
        sudo.chmod(0o755)
        self.keep(computer)

        without = subprocess.run(
            [*available_shells()[0], str(SCRIPT)],
            env={"MYSETUP_SCAN_TEST_ROOT": str(computer.root), "PATH": "/nonexistent"},
            capture_output=True,
            text=True,
        )
        self.assertFalse(marker.exists(), "sudo was used without --with-root")
        self.assertNotIn("memory.0.part", without.stdout)
        self.assertIn("sh mysetup-scan.sh --with-root", without.stderr)

        result = subprocess.run(
            [*available_shells()[0], str(SCRIPT), "--with-root"],
            env={"MYSETUP_SCAN_TEST_ROOT": str(computer.root), "PATH": "/nonexistent"},
            capture_output=True,
            text=True,
        )
        self.assertTrue(marker.exists(), "--with-root did not use sudo")
        self.assertEqual(problems(result.stdout), [])
        for value in PLANTED:
            self.assertNotIn(value, result.stdout)
        report = Report(result.stdout)
        self.assertEqual(report.fields["memory.0.size_mb"], "16384")
        self.assertEqual(report.fields["memory.0.maker"], "G Skill Intl")
        self.assertEqual(report.fields["memory.0.part"], "F4-3200C16-16GTZ")
        self.assertEqual(report.fields["memory.0.speed_mts"], "3200")
        self.assertEqual(report.fields["memory.0.configured_speed_mts"], "3000")
        self.assertEqual(report.fields["memory.1.size_mb"], "8192")
        self.assertEqual(report.fields["memory.1.part"], "KVR16S11/8")
        self.assertEqual(report.fields["memory.1.configured_speed_mts"], "1333")
        self.assertEqual(report.fields["memory.1.form"], "SODIMM")
        self.assertNotIn("memory.2.size_mb", report.fields)
        self.assertEqual(report.fields["capability.memory_modules"], "available")

    def test_raspberry_pi(self) -> None:
        result = self.scan(raspberry_pi())
        self.assertEqual(problems(result.stdout), [])
        report = Report(result.stdout)
        self.assertEqual(report.fields["os.architecture"], "arm64")
        self.assertEqual(report.fields["tested"], "no")
        self.assertEqual(report.fields["cpu.model"], "Cortex-A72")
        self.assertEqual(report.fields["cpu.vendor"], "ARM")
        self.assertEqual(report.fields["motherboard.model"], "Raspberry Pi 4 Model B Rev 1.4")
        self.assertEqual(report.fields["gpu.0.model"], "brcm,bcm2711-vc5")
        self.assertEqual(report.fields["storage.0.model"], "SC64G")
        self.assertEqual(report.fields["storage.0.interface"], "mmc")
        self.assertNotIn("storage.1.model", report.fields)
        self.assertEqual(report.fields["capability.memory_modules.reason"], "not-exposed")
        self.assertNotIn("dmidecode", result.stderr)

    def test_intel_mac(self) -> None:
        result = self.scan(mac_intel())
        self.assertEqual(problems(result.stdout), [])
        report = Report(result.stdout)
        self.assertEqual(report.fields["os.family"], "macos")
        self.assertEqual(report.fields["os.version"], "14.5")
        self.assertEqual(report.fields["tested"], "no")
        self.assertEqual(report.fields["environment"], "physical")
        self.assertEqual(report.fields["cpu.vendor"], "Intel")
        self.assertEqual(report.fields["cpu.cores"], "6")
        self.assertEqual(report.fields["gpu.0.pci"], "8086:3e9b")
        self.assertEqual(report.fields["gpu.0.vendor"], "Intel")
        self.assertEqual(report.fields["gpu.1.model"], "AMD Radeon Pro 5300M")
        self.assertEqual(report.fields["motherboard.model"], "MacBookPro16,1")
        self.assertEqual(report.fields["memory.total_mb"], "16384")
        self.assertEqual(report.fields["memory.0.size_mb"], "8192")
        self.assertEqual(report.fields["memory.0.part"], "0x484D")
        self.assertNotIn("memory.1.size_mb", report.fields)
        self.assertEqual(report.fields["storage.0.model"], "APPLE SSD AP1024N")
        self.assertEqual(report.fields["storage.0.size_bytes"], "1000555581440")
        self.assertEqual(report.fields["storage.0.interface"], "nvme")
        self.assertEqual(report.fields["storage.0.media"], "ssd")
        self.assertEqual(report.fields["storage.1.interface"], "usb")
        self.assertIn("Untested on this system (macos/x86-64)", result.stderr)

    def test_apple_silicon_under_rosetta_and_in_a_virtual_machine(self) -> None:
        result = self.scan(mac_apple_silicon())
        self.assertEqual(problems(result.stdout), [])
        report = Report(result.stdout)
        self.assertEqual(report.fields["os.architecture"], "arm64")
        self.assertEqual(report.fields["environment"], "virtual_machine")
        self.assertEqual(report.fields["cpu.vendor"], "Apple")
        self.assertEqual(report.fields["gpu.0.vendor"], "Apple")
        self.assertEqual(report.fields["memory.0.size_mb"], "16384")
        self.assertEqual(report.fields["memory.0.type"], "LPDDR5")
        self.assertEqual(report.fields["capability.storage.reason"], "no-source")
        self.assertIn("virtual machine", result.stderr)

    def test_freebsd(self) -> None:
        result = self.scan(freebsd())
        self.assertEqual(problems(result.stdout), [])
        report = Report(result.stdout)
        self.assertEqual(report.fields["os.family"], "freebsd")
        self.assertEqual(report.fields["os.architecture"], "x86-64")
        self.assertEqual(report.fields["os.version"], "14.1-RELEASE")
        self.assertEqual(report.fields["cpu.cores"], "8")
        self.assertEqual(report.fields["gpu.0.pci"], "10de:2204")
        self.assertEqual(report.fields["gpu.0.subsystem"], "1458:4037")
        self.assertEqual(report.fields["motherboard.model"], "X570 AORUS ELITE")
        self.assertEqual(report.fields["storage.0.interface"], "nvme")
        self.assertEqual(report.fields["storage.1.media"], "hdd")
        self.assertNotIn("storage.2.model", report.fields)
        self.assertEqual(report.fields["capability.memory_modules.reason"], "needs-root")
        self.assertIn("sudo pkg install dmidecode", result.stderr)

    def test_other_systems_give_the_basics_and_say_the_rest(self) -> None:
        for system, family, answers in (
            ("OpenBSD", "openbsd", {"-n hw.model": "Intel(R) Xeon(R) CPU E5-2670", "-n hw.ncpuonline": "16", "-n hw.physmem": "8589934592", "-n hw.vendor": "Supermicro", "-n hw.product": "X9DRi-LN4+"}),
            ("NetBSD", "netbsd", {"-n machdep.cpu_brand": "Intel(R) Xeon(R) CPU E5-2670", "-n hw.ncpuonline": "16", "-n hw.physmem64": "8589934592", "-n machdep.dmi.board-vendor": "Supermicro", "-n machdep.dmi.board-product": "X9DRi-LN4+"}),
        ):
            with self.subTest(system=system):
                computer = Computer()
                computer.command("uname", {"-s": system, "-m": "amd64", "-r": "7.5"})
                computer.command("sysctl", answers, default="FAKE-SERIAL", status=1)
                result = self.scan(computer)
                self.assertEqual(problems(result.stdout), [])
                report = Report(result.stdout)
                self.assertEqual(report.fields["os.family"], family)
                self.assertEqual(report.fields["environment"], "unknown")
                self.assertEqual(report.fields["cpu.threads"], "16")
                self.assertEqual(report.fields["memory.total_mb"], "8192")
                self.assertEqual(report.fields["motherboard.model"], "X9DRi-LN4+")
                self.assertEqual(report.fields["capability.gpu.reason"], "no-source")
                self.assertEqual(report.fields["capability.storage.reason"], "no-source")

    def test_illumos(self) -> None:
        computer = Computer()
        computer.command("uname", {"-s": "SunOS", "-m": "i86pc", "-r": "5.11"})
        computer.command("psrinfo", {"-pv": "The physical processor has 4 cores and 8 virtual processors\n  Intel(R) Xeon(R) CPU E3-1230\n"}, default="0 on-line\n1 on-line")
        computer.command("prtconf", {"-m": "16384"})
        computer.command("smbios", {"-t SMB_TYPE_BASEBOARD": "  Manufacturer: Supermicro\n  Product: X10SLM+-F\n  Serial Number: FAKE-SERIAL\n"})
        result = self.scan(computer)
        self.assertEqual(problems(result.stdout), [])
        report = Report(result.stdout)
        self.assertEqual(report.fields["os.family"], "illumos")
        self.assertEqual(report.fields["os.architecture"], "x86-64")
        self.assertEqual(report.fields["cpu.model"], "Intel(R) Xeon(R) CPU E3-1230")
        self.assertEqual(report.fields["cpu.threads"], "2")
        self.assertEqual(report.fields["memory.total_mb"], "16384")
        self.assertEqual(report.fields["motherboard.vendor"], "Supermicro")

    def test_an_unknown_system_still_gets_a_whole_report(self) -> None:
        computer = Computer()
        computer.command("uname", {"-s": "Plan9", "-m": "sparc"})
        result = self.scan(computer)
        self.assertEqual(result.returncode, 0)
        self.assertEqual(problems(result.stdout), [])
        report = Report(result.stdout)
        self.assertEqual(report.fields["os.family"], "other")
        self.assertEqual(report.fields["os.architecture"], "other")
        self.assertEqual(report.fields["capability.cpu.reason"], "no-source")


class Environments(Base):
    def environment(self, computer: Computer) -> str:
        return Report(self.scan(computer).stdout).fields["environment"]

    def test_every_answer_of_systemd_detect_virt(self) -> None:
        for answer, expected in (("kvm", "virtual_machine"), ("qemu", "virtual_machine"), ("vmware", "virtual_machine"), ("microsoft", "virtual_machine"),
                                 ("lxc", "container"), ("docker", "container"), ("podman", "container"), ("systemd-nspawn", "container"), ("none", "physical")):
            with self.subTest(answer=answer):
                computer = desktop()
                computer.command("systemd-detect-virt", {}, default=answer)
                self.assertEqual(self.environment(computer), expected)

    def test_without_systemd_the_cpu_flags_decide(self) -> None:
        computer = minimal_linux()
        self.assertEqual(self.environment(computer), "physical")
        computer = minimal_linux()
        computer.file("proc/cpuinfo", "processor : 0\nmodel name : Fake\nflags : fpu hypervisor sse\n")
        self.assertEqual(self.environment(computer), "virtual_machine")
        computer = minimal_linux()
        computer.file(".dockerenv", "")
        self.assertEqual(self.environment(computer), "container")

    def test_no_evidence_is_unknown_not_physical(self) -> None:
        computer = raspberry_pi()
        (computer.bin / "systemd-detect-virt").unlink()
        (computer.root / "proc/device-tree/model").unlink()
        self.assertEqual(self.environment(computer), "unknown")
        computer = raspberry_pi()
        (computer.bin / "systemd-detect-virt").unlink()
        computer.file("proc/device-tree/model", b"linux,dummy-virt\x00")
        self.assertEqual(self.environment(computer), "unknown")

    def test_a_container_is_not_told_to_install_dmidecode(self) -> None:
        computer = desktop()
        computer.command("systemd-detect-virt", {}, default="lxc")
        computer.command("udevadm", {})
        result = self.scan(computer)
        self.assertEqual(Report(result.stdout).fields["capability.memory_modules.reason"], "not-exposed")
        self.assertNotIn("dmidecode", result.stderr)
        self.assertIn("container", result.stderr)

    def test_wsl_is_sent_to_windows_with_no_report(self) -> None:
        for how in ("osrelease", "detect"):
            with self.subTest(how=how):
                computer = desktop()
                if how == "osrelease":
                    computer.file("proc/sys/kernel/osrelease", "6.6.87.2-microsoft-standard-WSL2\n")
                else:
                    computer.command("systemd-detect-virt", {}, default="wsl")
                result = self.scan(computer)
                self.assertEqual(result.returncode, 4)
                self.assertEqual(result.stdout, "")
                self.assertIn("mysetup-scan.ps1", result.stderr)

    def test_shells_on_windows_are_sent_to_powershell(self) -> None:
        for system in ("MINGW64_NT-10.0-26100", "MSYS_NT-10.0", "CYGWIN_NT-10.0"):
            with self.subTest(system=system):
                computer = Computer()
                computer.command("uname", {"-s": system, "-m": "x86_64"})
                result = self.scan(computer)
                self.assertEqual(result.returncode, 4)
                self.assertEqual(result.stdout, "")
                self.assertIn("mysetup-scan.ps1", result.stderr)


class Values(Base):
    def board(self, name: str) -> Report:
        computer = desktop()
        computer.file("sys/class/dmi/id/board_name", name + "\n")
        computer.file("sys/class/dmi/id/board_vendor", "Maker\n")
        return Report(self.scan(computer).stdout)

    def test_placeholders_are_dropped(self) -> None:
        for name in ("To Be Filled By O.E.M.", "Default string", "NOT APPLICABLE", "None", "0000000", "0x0000", "xxxxxxxx", "---", "BaseBoard Product Name", "Unknown"):
            with self.subTest(name=name):
                self.assertNotIn("motherboard.model", self.board(name).fields)

    def test_values_shaped_like_addresses_or_ids_are_dropped(self) -> None:
        for name in ("de:ad:be:ef:00:01", "DE-AD-BE-EF-00-01", "12345678-1234-1234-1234-123456789abc", "10.20.30.40", "someone@example.org", "/home/someone/x", "C:\\Users\\someone"):
            with self.subTest(name=name):
                self.assertNotIn("motherboard.model", self.board(name).fields)

    def test_real_names_with_awkward_characters_survive(self) -> None:
        for name in ("Intel(R) Core(TM) i7 CPU 950 @ 3.07GHz", "X58A-GD65 (MS-7522)", "Navi 21 [Radeon RX 6800/6800 XT / 6900 XT]", "A=B", "1066", "ROG STRIX X670E-E GAMING WIFI"):
            with self.subTest(name=name):
                self.assertEqual(self.board(name).fields["motherboard.model"], name)

    def test_control_characters_and_escapes_are_removed(self) -> None:
        computer = desktop()
        computer.file("sys/class/dmi/id/board_name", b"Bad\x1b[31mName\x07\xc3\xa9 \t  Two\n")
        report = Report(self.scan(computer).stdout)
        self.assertEqual(report.fields["motherboard.model"], "Bad[31mName Two")

    def test_long_values_are_cut(self) -> None:
        self.assertEqual(self.board("A" * 400).fields["motherboard.model"], "A" * 120)

    def test_hostile_names_are_printed_and_never_run(self) -> None:
        computer = desktop()
        computer.link("touch")
        canary = computer.root / "canary"
        hostile = "$(touch canary) `touch canary` ; touch canary"
        computer.file("sys/class/dmi/id/board_name", hostile + "\n")
        computer.file("etc/os-release", "ID=debian$(touch canary)\nVERSION_ID=`touch canary`\n")
        computer.command("lsblk", {LSBLK_ARGUMENTS: 'NAME="sda" TYPE="disk" MODEL="$(touch canary)" SIZE="100" TRAN="sata" ROTA="0"\n'})
        computer.command("udevadm", {"info -q property -p /devices/virtual/dmi/id": "MEMORY_DEVICE_0_SIZE=1048576\nMEMORY_DEVICE_0_PART_NUMBER=$(touch canary)\n"})
        computer.command("lscpu", {}, default="Model name: $(touch canary)\n")
        result = self.scan(computer)
        self.assertFalse(canary.exists(), "a value was run as a command")
        report = Report(result.stdout)
        self.assertEqual(report.fields["motherboard.model"], hostile)
        self.assertNotIn("os.name", report.fields)
        self.assertNotIn("os.version", report.fields)

    def test_an_item_cap_holds(self) -> None:
        computer = desktop()
        computer.command("lsblk", {LSBLK_ARGUMENTS: "".join(f'NAME="sd{i}" TYPE="disk" MODEL="Disk {i}" SIZE="100" TRAN="sata" ROTA="0"\n' for i in range(70))})
        result = self.scan(computer)
        self.assertEqual(problems(result.stdout), [])
        report = Report(result.stdout)
        self.assertIn("storage.63.model", report.fields)
        self.assertNotIn("storage.64.model", report.fields)
        self.assertIn("Only the first 64 disks", result.stderr)


class Behaviour(Base):
    def test_help_version_and_refusals(self) -> None:
        computer = self.keep(desktop())
        self.assertIn("--with-root", computer.run("--help").stdout)
        self.assertIn("0.1.0", computer.run("--version").stdout)
        refused = computer.run("--nonsense")
        self.assertEqual(refused.returncode, 2)
        self.assertEqual(refused.stdout, "")

    def test_a_host_without_awk_is_told_so(self) -> None:
        computer = self.keep(desktop())
        (computer.bin / "awk").unlink()
        result = computer.run()
        self.assertEqual(result.returncode, 3)
        self.assertIn("no awk", result.stderr)
        self.assertEqual(result.stdout, "")

    def test_the_report_is_on_standard_output_and_the_notes_are_not(self) -> None:
        result = self.scan(minimal_linux())
        self.assertTrue(result.stdout.startswith("mysetup-report 1\n"))
        self.assertTrue(result.stdout.endswith("mysetup-report-end\n"))
        self.assertNotIn("Untested", result.stdout)
        self.assertIn("sends nothing anywhere", result.stderr)

    def test_the_script_uses_no_network_runs_no_text_and_writes_nothing(self) -> None:
        source = SCRIPT.read_text()
        code = "\n".join(line for line in source.splitlines() if not line.lstrip().startswith("#"))
        for word in ("curl", "wget", "fetch", "nc", "ssh", "scp", "telnet", "eval", "mktemp", "tee", "rm", "mv", "cp", "chmod"):
            self.assertIsNone(re.search(rf"(?<![\w.-]){word}(?![\w.-])", code), f"the script uses {word}")
        for text in ("/dev/tcp", "/dev/udp", "/tmp"):
            self.assertNotIn(text, code, f"the script uses {text}")
        self.assertIsNone(re.search(r"(^|[;&|]\s*)(\.|source)\s+[\"$/\w]", code, re.M), "the script sources a file")
        # Redirections to a path, a variable or a file name. "> 0" and "> last" are comparisons inside awk.
        writes = re.findall(r"(?<![0-9&])>>?\s*(?:/[^\s)]*|\"?\$[^\s)]*|\w+\.\w+)", code)
        self.assertEqual([w for w in writes if "/dev/null" not in w], [], "the script writes a file")

    def test_the_script_never_asks_for_identifying_sources(self) -> None:
        source = SCRIPT.read_text()
        code = "\n".join(line for line in source.splitlines() if not line.lstrip().startswith("#"))
        for word in ("board_serial", "product_serial", "product_uuid", "chassis_serial", "SERIAL", "WWN", "hostname", "whoami", "ifconfig", "ip addr", "uname -n", "uname -a"):
            self.assertNotIn(word, code, f"the script asks for {word}")


class Validator(unittest.TestCase):
    def test_it_finds_what_it_should(self) -> None:
        good = DESKTOP_REPORT
        self.assertEqual(problems(good), [])
        self.assertEqual(problems("notes before\r\n" + good.replace("\n", "\r\n") + "notes after"), [])
        for changed in (
            good.replace("mysetup-report-end\n", ""),
            good.replace("cpu.cores=4", "cpu.serial=4"),
            good.replace("cpu.cores=4", "cpu.cores=four"),
            good.replace("gpu.0.pci=10de:1b06", "gpu.0.pci=10DE:1B06"),
            good.replace("environment=physical", "environment=laptop"),
            good.replace("motherboard.model=PRIME Z270-K", "motherboard.model=de:ad:be:ef:00:01"),
            good.replace("memory.1.size_mb", "memory.3.size_mb"),
            good.replace("capability.displays.reason=not-collected\n", ""),
        ):
            self.assertNotEqual(problems(changed), [], changed[:40])


if __name__ == "__main__":
    os.chdir(HERE.parent)
    unittest.main(verbosity=1)
