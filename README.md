# MySetup scan

Tells [MySetup](https://mysetup.is) what hardware your computer has. MySetup offers three ways to scan, on your build page:

| | Browser | Script | App |
|---|---|---|---|
| Needs | Nothing | Nothing installed | A download |
| Detail | Least | Full | Full |
| Best mark | Detected | Detected | Verified, once sending is built |

This repository holds the script and the app. The script's first release is
[v0.1.0](https://github.com/eddinn/mysetup-scanner/releases/tag/v0.1.0) (November 2026). The app, a Go program, is being built in
stages, and its first test release is the pre-release
[app-v0.1.0-beta.2](https://github.com/eddinn/mysetup-scanner/releases/tag/app-v0.1.0-beta.2); see [The app](#the-app-beta).

## The script

It prints a text report. You read it, then copy it and paste it on your build page. It only reads: it uses no network, writes
no files, installs nothing and does not ask for root unless you say so. Please read it before you run it; it is meant to be
read.

Download the file for your system from the [latest release](https://github.com/eddinn/mysetup-scanner/releases/latest), or from
your build page on MySetup, which serves the same files and links to the release of the version it serves, and run it.

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

Then copy everything from `mysetup-report 1` to `mysetup-report-end` and paste it in the Hardware section of your build on
MySetup, where you review every part before anything is saved. The format is in [docs/report-format.md](docs/report-format.md).

### Check a download

Each release carries the two scripts, `SHA256SUMS`, and GitHub's attestation that these files were built from this repository at
that release's tag:

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify mysetup-scan.sh --repo eddinn/mysetup-scanner
```

MySetup's build page shows each file's SHA-256 too. The scripts are not code-signed yet.

## The app (beta)

One file with nothing to install: no runtime, no PowerShell, no Python. It reads the same report as the script, and shows it on a
page in your own browser, with buttons to copy it or save it as a file. The page is served from your computer only and ends when
you click Done, close its tab, or after 15 minutes. With `--cli` the app prints the report in the terminal instead, as it also does
where no browser can be opened, such as over SSH. It only reads: nothing is sent anywhere, no files are written, and nothing is
installed.

Test builds for Windows and Linux, on x86-64 and ARM, are in the
[pre-release](https://github.com/eddinn/mysetup-scanner/releases/tag/app-v0.1.0-beta.2):

- **Windows:** download `mysetup-scan-windows-x86-64.exe` (or `-arm64.exe`) and double-click it. The app is not code-signed yet, so
  SmartScreen warns about it: choose "More info", then "Run anyway".
- **Linux:** download `mysetup-scan-linux-x86-64` (or `-arm64`), then `chmod +x mysetup-scan-linux-x86-64` and
  `./mysetup-scan-linux-x86-64`. Add `--cli` for the terminal.

Check a download the same way as the scripts, with the release's `SHA256SUMS` and
`gh attestation verify mysetup-scan-linux-x86-64 --repo eddinn/mysetup-scanner`. Other systems build from source. How the app
reads each system, how its page is hardened, and what has been tested are in [docs/app.md](docs/app.md).

## What the script needs

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

The scripts need only what they check for themselves. The tests need Python 3 (its standard library only), and the linters
`shellcheck` and PSScriptAnalyzer (`Install-Module PSScriptAnalyzer -Scope CurrentUser`). The workflow
[`scripts.yml`](.github/workflows/scripts.yml) runs the same steps on Linux, Windows and macOS for every change to the scripts or
tests, and on Linux also in containers of eight other distributions.

**Linux, macOS and other Unix-like systems:**

```sh
shellcheck -s sh scripts/mysetup-scan.sh
python3 -B tests/test_scan_sh.py
sh scripts/mysetup-scan.sh > report.txt
python3 -B tests/validate_report.py report.txt
```

`shellcheck` checks the script as POSIX `sh`. `test_scan_sh.py` runs it on made-up computers: fake commands and files whose
sources are full of planted serial numbers, hostnames and addresses, none of which may reach the report; it runs under every shell
(dash, bash in POSIX mode, BusyBox sh, zsh, ksh, mksh, yash) and awk it finds. The last two lines scan the computer you are on and
check its report against [docs/report-format.md](docs/report-format.md); inside WSL the script declines, as it does for members.

**Windows, in PowerShell:**

```powershell
Invoke-ScriptAnalyzer -Path scripts\mysetup-scan.ps1 -Severity Warning,Error -ExcludeRule PSAvoidUsingWriteHost
powershell -NoProfile -ExecutionPolicy Bypass -File tests\test_scan_ps1.ps1 -ReportPath made-up.txt
python -B tests\validate_report.py made-up.txt
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\mysetup-scan.ps1 | Set-Content -Encoding ascii report.txt
python -B tests\validate_report.py report.txt
```

`test_scan_ps1.ps1` does the same with a made-up computer, by standing in for Windows' classes; `-ReportPath` keeps its report for
the check. The last two lines scan this computer with the members' own command.

`-B` keeps Python from writing bytecode into `tests/`; `report.txt` and `made-up.txt` are ignored by Git.

The app has its own steps in [docs/app.md](docs/app.md#build-and-test).

`tools/measure-matches.php` measures how often real reports' parts match the MySetup catalogue, through the site's own parser and
matcher; it runs in `php artisan tinker` in a checkout of the site (the file says how).

A release of the scripts starts from a signed tag `v<version>` matching the scripts' version, with its notes in `docs/releases/`.
The release workflow checks the version, attaches the two scripts and `SHA256SUMS`, attests them and drafts the release, which a
person reads and publishes. Run by hand on a branch, it does the checks and stops before the attestation and the release. The app's
releases start from a tag `app-v<version>` and have their own workflow ([docs/app.md](docs/app.md#releases)).

## Licence

Apache-2.0.
