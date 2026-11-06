# MySetup scan

Tells [MySetup](https://mysetup.is) what hardware your computer has. MySetup offers three ways to scan, on your build page:

| | Browser | Script | App |
|---|---|---|---|
| Needs | Nothing | Nothing installed | A download |
| Detail | Least | Full | Full |
| Best mark | Detected | Detected | Verified |

This repository holds the script, which is ready to try. The app, a Go program, is designed but not built yet.

## The script

It prints a text report. You read it, then copy it and paste it on your build page. It only reads: it uses no network, writes
no files, installs nothing and does not ask for root unless you say so. Please read it before you run it; it is meant to be
read.

Download the file for your system and run it. Releases will carry checksums to compare it with.

**Linux, macOS, the BSDs and other Unix-like systems:**

```sh
sh mysetup-scan.sh
```

**Windows, in PowerShell:**

```powershell
powershell -ExecutionPolicy Bypass -File .\mysetup-scan.ps1
```

Windows blocks unsigned scripts from the internet by default. `-ExecutionPolicy Bypass` lifts that for this one run and changes
no setting. On a managed computer, Group Policy may still refuse it.

There is deliberately no pipe-to-shell one-liner: it would skip the reading.

Then copy everything from `mysetup-report 1` to `mysetup-report-end`. The format is in
[docs/report-format.md](docs/report-format.md).

## Requirements

| System | Reads with | To install | Tested on a real machine |
|---|---|---|---|
| Windows x86-64 | Windows' own classes | Nothing | Yes |
| Windows on ARM | Windows' own classes | Nothing | No |
| Linux x86-64 with systemd | `/sys`, `lscpu`, `lsblk`, `udevadm`, `lspci` | Nothing; `pciutils` adds graphics names | Yes |
| Linux on ARM, such as a Raspberry Pi | The same, and the device tree | Nothing; `pciutils` adds graphics names | No |
| Linux without systemd, such as Alpine, Void or some NAS | `/sys`, `/proc` | `dmidecode` and root for memory modules | No |
| macOS | `sysctl`, `system_profiler`, `diskutil` | Nothing | No |
| FreeBSD | `sysctl`, `pciconf`, `kenv`, `geom` | `dmidecode` and root for memory modules | No |
| OpenBSD, NetBSD, illumos | `sysctl` or their own tools | `dmidecode` and root for memory modules | No: the basics only |

The Unix script itself needs only `sh`, `awk`, `tr`, `uname` and `id`. Everything else is optional and tried in order. When
something is missing, the script prints the exact install command for your system and leaves it to you. It installs nothing.

`--with-root` uses `dmidecode` through `sudo` or `doas` to read memory modules where they cannot be read otherwise. It is never
used without being asked for, and it is used for nothing else.

Until a real machine of a system has passed, the script says "untested on this system" there and the report says `tested=no`.
Please check such a report before you paste it, and tell us how it went: that is how a system gets listed.

Inside WSL the script sends you to the Windows script, because WSL sees virtual hardware. In a virtual machine or a container it
reports what it was given and says so.

## What it never reads or sends

Usernames, hostnames, IP or MAC addresses, motherboard or system UUIDs, machine GUIDs, Windows SIDs, product keys, disk serial
numbers, file paths, file names, network names or other software.

Some sources hold such things beside what the report wants. So sources are read by exact key, and only the fields named in one
function can be printed. The tests build made-up computers whose sources are full of planted serial numbers, hostnames and
addresses, and fail if any get through.

## Development

```sh
python3 tests/test_scan_sh.py
python3 tests/validate_report.py report.txt
```

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File tests\test_scan_ps1.ps1
```

`tools/measure-matches.php` measures how often real reports match the MySetup catalogue, with the site's own matcher.

## Licence

Apache-2.0.
