# The app

A Go program that prints the same report as the scripts ([report-format.md](report-format.md)), with `source=app/VERSION`. The
design and the operator's decisions are in MySetup's `docs/scanner/design.md` ("The app: the operator's decisions"). It is built
in stages; this is **stage 1, reading**: the report in the terminal. The site side (codes, upload, server checks), the browser page
and sending, and releases come in the next stages.

## Every system, nothing to install

One file per system and processor, built without C (`CGO_ENABLED=0`), so it starts as it is: no runtime, no PowerShell, no Python,
no package, and no third-party code (only Go and its official `golang.org/x/sys`). It runs without administrator or root; run with
them, it can also read what the system keeps from ordinary users. It never asks for them by itself: it says what more it would read.

| System | How it reads | Root or administrator adds |
|---|---|---|
| Windows | The firmware's SMBIOS table (`GetSystemFirmwareTable`), the processor's registry entry, the display adapters SetupAPI lists, each disk's storage queries through a handle with no access rights | Nothing known |
| Linux | `/proc`, `/sys` (PCI, DMI, block devices), `os-release`, udev's database; the system's own `pci.ids` for graphics names where it has one | Memory modules where udev is missing, from the firmware table in `/sys/firmware/dmi/tables` |
| macOS | `sysctl` directly; Apple's own `system_profiler` and `diskutil` | Nothing expected |
| FreeBSD | `sysctl` directly; the base system's `kenv`, `pciconf` and `geom` | Memory modules, from the firmware table in `/dev/mem` |
| OpenBSD, NetBSD | `sysctl` directly | Memory modules where the system hands the firmware table to root |
| illumos | Its own `psrinfo`, `prtconf` and `smbios` | As FreeBSD |

A system's own tool is run only from `/usr/sbin`, `/usr/bin`, `/sbin` or `/bin`, never found through `PATH`, with fixed arguments,
no shell, a time limit and a size limit on what it prints. Sources that also hold serial numbers (udev, the firmware table, the disk
descriptors, `system_profiler`, `geom`) are read by exact key or offset, and the report is written through one function that names
every field ([internal/report](../internal/report/report.go)), as in the scripts.

In a virtual machine or a container the report says so, and the app tells the member to run it on the host to include the real
hardware. Inside WSL it declines and sends the member to the Windows app.

## Tested

A system is marked tested once a real machine of it has passed (`tested=yes`). On 2026-11-07 the app's reports were identical to the
scripts' on four real machines, apart from `source`:

- Windows 11 x86-64 (Intel Core i9-13900KF, ASUS ROG MAXIMUS Z790 HERO, two DDR5 modules, two NVMe disks), without administrator rights.
- Linux x86-64, Debian 13 with udev, three servers (pve0, pve1, pve2), run as `nobody`.

The others are built and vetted for every target, and run on GitHub's macOS runner, but no real machine of them has passed yet. The
paths that need root (the firmware table on Linux without udev, `/dev/mem` on FreeBSD) are tested on a made-up table only.

## Build and test

Go 1.27 or newer:

```sh
go vet ./...
go test ./...
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w -buildid=' -o mysetup-scan ./cmd/mysetup-scan
./mysetup-scan > report.txt
python3 -B tests/validate_report.py report.txt
```

For another system, set `GOOS` and `GOARCH`, such as `GOOS=windows GOARCH=amd64` for `mysetup-scan.exe`. The workflow
[`app.yml`](../.github/workflows/app.yml) vets every target, runs the tests on Linux, Windows and macOS, runs the app on each runner
and checks its report, builds every target twice and fails if the bytes differ, and keeps the builds as a workflow artifact for 14
days.
