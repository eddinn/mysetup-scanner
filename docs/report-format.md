# Report format, version 1

What `scripts/mysetup-scan.sh`, `scripts/mysetup-scan.ps1` and the app (`cmd/mysetup-scan`) print. The member pastes it on their MySetup build page, so
it is plain text a person can read before sending. `tests/validate_report.py` checks a report against this page.

```
mysetup-report 1
source=script-sh/0.2.0
tested=yes
os.family=linux
os.name=debian
os.version=13
os.architecture=x86-64
environment=physical
system.vendor=ASUS
system.chassis=desktop
cpu.vendor=AMD
cpu.model=AMD Ryzen 7 7800X3D 8-Core Processor
cpu.cores=8
cpu.threads=16
gpu.0.pci=10de:2204
gpu.0.subsystem=1458:4037
gpu.0.vendor=NVIDIA Corporation
gpu.0.model=TU102 [GeForce RTX 3090]
motherboard.vendor=ASUSTeK COMPUTER INC.
motherboard.model=ROG STRIX X670E-E
memory.total_mb=31890
memory.0.size_mb=16384
memory.0.maker=Corsair
memory.0.part=CMK32GX5M2B6000C30
memory.0.speed_mts=6000
memory.0.configured_speed_mts=6000
memory.0.type=DDR5
memory.0.form=DIMM
storage.0.model=Samsung SSD 990 PRO 2TB
storage.0.size_bytes=2000398934016
storage.0.interface=nvme
storage.0.media=ssd
capability.cpu=available
capability.gpu=available
capability.motherboard=available
capability.memory_modules=available
capability.configured_memory=available
capability.storage=available
capability.displays=unavailable
capability.displays.reason=not-collected
mysetup-report-end
```

The values above are made up.

## Rules

- The first line is `mysetup-report 1`, where 1 is this format's version, and the last is `mysetup-report-end`. A reader
  ignores anything outside those two lines, such as the script's notes copied along with them, and refuses a report
  without both: it was cut short.
- Between them, each line is `field=value`, split at the first `=`. Line endings may be LF or CRLF.
- Only the fields below exist. A reader refuses a report with any other field (ADR-005). A missing field means that
  value is not known; it is never replaced by a guess (ADR-003).
- Values are printable ASCII, at most 120 characters. Anything else is removed before printing.
- `N` counts from 0, with no gaps within a part, and at most 64 of each part. The same `N` is the same device.

## Fields

| Field | Value |
|---|---|
| `source` | `script-sh/VERSION`, `script-ps1/VERSION`, or `app/VERSION` for the app |
| `tested` | `yes` once a real machine of this system has passed, otherwise `no` |
| `os.family` | `linux`, `macos`, `windows`, `freebsd`, `openbsd`, `netbsd`, `illumos` or `other` |
| `os.name` | The distribution or system, such as `debian`; a word |
| `os.version` | Such as `13`; a word |
| `os.architecture` | `x86-64`, `arm64`, `x86`, `arm`, `riscv64`, `ppc64le`, `ppc64`, `s390x` or `other` |
| `environment` | `physical`, `virtual_machine`, `container`, `wsl` or `unknown`, as `scanner_reports.environment_type` |
| `system.vendor`, `system.model`, `system.family`, `system.version`, `system.sku` | Text: the computer's own maker and model as its firmware names them (SMBIOS type 1; Linux's `sys_vendor`, `product_name`, `product_family`, `product_version` and `product_sku`), such as a laptop's or a prebuilt's model. Its serial number and UUID are never read. A built computer's firmware usually leaves them at a placeholder ("System Product Name"), which is left out. Since 0.2.0 (the operator, 2026-11-13: "we need to match models") |
| `system.chassis` | A word: the chassis type as SMBIOS names it (DSP0134 7.4.1), such as `desktop`, `tower`, `mini-pc`, `laptop`, `notebook`, `convertible`, `all-in-one` or `rack-mount-chassis`; left out when it says other or unknown. Since 0.2.0 |
| `cpu.vendor`, `cpu.model` | Text |
| `cpu.cores`, `cpu.threads` | Whole number |
| `gpu.N.pci` | PCI vendor and device, four lower-case hex digits each: `10de:2204` |
| `gpu.N.subsystem` | PCI subsystem vendor and device, the same way. This tells apart cards with the same chip. |
| `gpu.N.vendor`, `gpu.N.model` | Text. On Linux they come from `lspci`, if installed, so `model` may name the chip. |
| `motherboard.vendor`, `motherboard.model` | Text |
| `memory.total_mb` | What the system reports as usable, which is less than what is installed |
| `memory.N.size_mb` | Whole number; empty slots are not listed |
| `memory.N.maker`, `memory.N.part` | Text. A maker may be a JEDEC code. |
| `memory.N.speed_mts`, `memory.N.configured_speed_mts` | Whole number, the rated and the running speed |
| `memory.N.type`, `memory.N.form` | A word, such as `DDR5` and `DIMM` |
| `storage.N.model` | Text. Without udev, as on hosts with no systemd, Linux cuts SATA names to 16 characters. |
| `storage.N.size_bytes` | Whole number |
| `storage.N.interface` | A word, such as `nvme`, `sata`, `usb`, `sas`, `mmc`, `virtio` |
| `storage.N.media` | `ssd` or `hdd`; left out where the system cannot tell, as over USB |
| `capability.NAME` | `available` or `unavailable` |
| `capability.NAME.reason` | Given when unavailable: see below |

`NAME` is `cpu`, `gpu`, `motherboard`, `memory_modules`, `configured_memory`, `storage` or `displays`, as in ADR-003.

| Reason | Meaning |
|---|---|
| `needs-root` | Readable with `--with-root` |
| `not-exposed` | The system does not offer it, as with memory soldered to an ARM board |
| `no-source` | Nothing the script tried could read it |
| `not-collected` | Not part of this report version. Displays are left out: their identifying data also holds a serial number. |

## What it never carries

Usernames, hostnames, IP or MAC addresses, motherboard or system UUIDs, machine GUIDs, Windows SIDs, product keys, serial
numbers of the computer, its chassis or any part, asset tags, file paths, file names, network names, browser identifiers or
other software (ADR-005).

The scripts and the app read their sources by exact key, and print only through one function that names every field. As a second
boundary, that function drops a value shaped like a MAC or IP address, a UUID, an email address or a home directory. The
server is the third, and treats a report as untrusted: anyone can edit it, which is why a script's parts are Detected at
most.
