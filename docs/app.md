# The app

A Go program that reads the same report as the scripts ([report-format.md](report-format.md)), with `source=app/VERSION`, and
shows it on a page in your own browser, or in the terminal with `--cli`. The design and the operator's decisions are in MySetup's
`docs/scanner/design.md` ("The app: the operator's decisions"). It is built in stages. Stage 1 is here: reading, and the two front
ends. The site side (codes, upload, server checks), sending from the page, and code signing come in the next stages. Its current
test release is `app-v0.1.0-beta.4`. The member's steps, for each system, are in the [README](../README.md).

## The page and the terminal

| How it is started | What it does |
|---|---|
| Double-clicked, or run with no option | Opens a page in your default browser with the report and six numbered steps to add it to the build: Copy report, open mysetup.is, the build, Scan this computer, paste, Done |
| `mysetup-scan --cli` | Prints the report in the terminal, always: the report on standard output, and everything said around it, with the same steps, on standard error |
| `mysetup-scan --no-browser` | Serves the page without opening a browser, and prints its address to open yourself |
| There is no desktop | Prints the report in the terminal with the steps, and says why: over SSH or with no desktop (no `DISPLAY` or `WAYLAND_DISPLAY`), or no `xdg-open` |

Run with sudo or as administrator, the page still opens in a browser (the operator's rule, 2026-11-07), but neither the page nor the
browser ever runs elevated:

- **Linux, the BSDs, macOS:** the app reads first, as root. Then it finds the person who ran sudo, doas, su or pkexec. On Linux that
  is the first process above the app that holds no root at all, read from `/proc`: sudo itself keeps the person's real user
  beside root's effective one, with root's group, so it does not count. Elsewhere it is sudo's `SUDO_UID` or doas's `DOAS_USER`.
  The app gives up root for good to that person's user, group and groups (it checks that root cannot be taken back), then serves
  the page and starts the browser in that person's desktop with their own environment. A person without a desktop (over SSH) gets
  the report in the terminal; nobody else's desktop is ever used. Only a desktop that is root's own opens as root.
- **Windows:** the app starts itself again as the signed-in user, without administrator rights, with the token of the desktop's own
  shell, and the elevated copy ends. If that cannot be done (UAC turned off, or no shell), it hands the page's address to Explorer,
  which opens it in the user's browser. The report needs no administrator on Windows.

On Linux and the BSDs root can add memory modules (see below), and the app says so when it would.

Double-clicked on Windows, Windows opens a console window for the app alone. The app starts again without one and that window
closes, so only the browser page is left. This works the same in the classic console and in Windows Terminal. If it cannot start
again, it carries on in the window. Without a console it has nowhere to print, so it reports a problem in a message box. Run with
`--cli` in a window of its own, it copies the report to the clipboard and waits for Enter before the window closes. It counts a
window as its own only when it is alone on the console and its input and output are that console. Another program that takes
its output (WSL, or a script redirecting it) may give it a console of its own, but there it never waits: `app-v0.1.0-beta.2`
waited there for an Enter nobody could see.

The page ends when you click Done, about 90 seconds after its tab closes (the open page checks in every 20 seconds), on Ctrl+C,
and after 15 minutes at most. The app does not linger.

The page is an attack surface and is hardened as the design asks ([internal/page](../internal/page/page.go)):

- It listens on 127.0.0.1 only, on a port the system picks, and each run has its own random 256-bit token in every address.
  Without the token, every request is refused.
- The `Host` header must be exactly `127.0.0.1:port`, so a site that points its own name at 127.0.0.1 (DNS rebinding) gets
  `421 Misdirected Request`.
- Done, the one request that changes anything, is a POST accepted only with the page's own `Origin` (and `Sec-Fetch-Site:
  same-origin` where the browser sends it). Another site's request cannot end the app.
- A content policy lets the page load nothing from anywhere else: `default-src 'none'`, with its one style and one script allowed
  by a fresh nonce for each response, and no framing. It is also sent with `nosniff`, `no-referrer`, `no-store` and same-origin
  opener and resource policies.
- The report and notes are written through Go's `html/template`, which escapes them.
- The server has read, write and header time limits, and a 16 KB header limit.

## Every system, nothing to install

One file per system and processor, built without C (`CGO_ENABLED=0`), so it starts as it is: no runtime, no PowerShell, no Python,
no package, and no third-party code (only Go and its official `golang.org/x/sys`). It runs without administrator or root. On Linux
and the BSDs, root lets it also read what the system keeps from ordinary users. It never asks for root by itself; it says what more
root would read.

| System | How it reads | Root or administrator adds |
|---|---|---|
| Windows | The firmware's SMBIOS table (`GetSystemFirmwareTable`), the processor's registry entry, the display adapters SetupAPI lists, each disk's storage queries through a handle with no access rights | Nothing known |
| Linux | `/proc`, `/sys` (PCI, DMI, block devices), `os-release`, udev's database; the system's own `pci.ids` for graphics names where it has one | Memory modules where udev is missing, from the firmware table in `/sys/firmware/dmi/tables` |
| macOS | `sysctl` directly; Apple's own `system_profiler` and `diskutil` | Nothing expected |
| FreeBSD | `sysctl` directly; the base system's `kenv`, `pciconf` and `geom` | Memory modules, from the firmware table in `/dev/mem` |
| OpenBSD, NetBSD | `sysctl` directly | Memory modules where the system hands the firmware table to root |
| illumos | Its own `psrinfo`, `prtconf` and `smbios` | As FreeBSD |

A system's own tool is run only from `/usr/sbin`, `/usr/bin`, `/sbin` or `/bin`, never found through `PATH`, with fixed arguments,
no shell, a time limit and a size limit on what it prints. The browser opener is the system's own too: `ShellExecute` on Windows,
`/usr/bin/open` on macOS, and `xdg-open` from `/usr/bin`, `/usr/local/bin` or `/bin` elsewhere. Sources that also hold serial
numbers (udev, the firmware table, the disk descriptors, `system_profiler`, `geom`) are read by exact key or offset. The report is
written through one function that names every field ([internal/report](../internal/report/report.go)), as in the scripts.

In a virtual machine or a container the report says so, and the app tells the member to run it on the host to include the real
hardware. Inside WSL it declines and sends the member to the Windows app.

## Tested

A system is marked tested once a real machine of it has passed (`tested=yes`). On 2026-11-07 the app's reports were identical to the
scripts' on four real machines, apart from `source`:

- Windows 11 x86-64 (Intel Core i9-13900KF, ASUS ROG MAXIMUS Z790 HERO, two DDR5 modules, two NVMe disks), without administrator rights.
- Linux x86-64, Debian 13 with udev, three servers (pve0, pve1, pve2), run as `nobody`.

The front ends were tried on the same day:

- On that Windows computer: a double-click closed its window in 0.1 seconds, served the page on 127.0.0.1 a second later, and the
  browser loaded it. With `--no-browser`, the page's download matched `--cli` byte for byte. Another site's Done and a wrong token
  were refused, and the page's own Done ended the app with exit code 0.
- On that Windows computer, for beta.3, each way of starting `--cli`: from PowerShell and from WSL with its output piped it ended
  by itself, as it did in a console of its own with its output redirected to a file. In a window of its own it waited for Enter
  with the report copied.
- For beta.4, as root under sudo on a Linux desktop (WSL's, through a test, since the app itself declines WSL): the app found
  the person who ran sudo, gave up root, and opened the page through a stand-in opener that ran as uid and gid 1000 with the
  person's own groups, desktop and home. That test first caught sudo's root group being taken. Starting again without
  administrator rights on Windows needs a UAC prompt, so it waits for the operator's own try.
- On pve1 over SSH: as root and as `nobody` with no desktop, the app printed the report with its reason, and also with a display but
  no `xdg-open`. With `--no-browser` as `nobody`, it listened on 127.0.0.1 only and refused a wrong token, another site's Done, and
  a rebinding host name. The page's own Done, Ctrl+C and SIGTERM each ended it with exit code 0.

The other systems are built and vetted for every target, and run on GitHub's macOS runner, but no real machine of them has passed
yet. The paths that need root (the firmware table on Linux without udev, `/dev/mem` on FreeBSD) are tested on a made-up table only.

## Build and test

Go 1.27 or newer:

```sh
go vet ./...
go test ./...
CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags='-s -w -buildid=' -o mysetup-scan ./cmd/mysetup-scan
./mysetup-scan --cli > report.txt
python3 -B tests/validate_report.py report.txt
./mysetup-scan
```

The last line opens the page. For another system, set `GOOS` and `GOARCH`, such as `GOOS=windows GOARCH=amd64` for
`mysetup-scan.exe`. The workflow [`app.yml`](../.github/workflows/app.yml) vets every target. On Linux, Windows and macOS it runs
the tests, runs the app with `--cli` and checks its report, and serves the page with `--no-browser`. It checks the page as a browser
would use it: the download matches the report, another site's Done is refused, and the page's own Done ends the app. It builds every
target twice and fails if the bytes differ, and keeps the builds as a workflow artifact for 14 days.

## Releases

A release starts from a signed tag `app-v<version>`, such as `app-v0.1.0-beta.1`, matching `Version` and `Prerelease` in
[internal/scan/hardware.go](../internal/scan/hardware.go), with its notes in `docs/releases/`. The workflow
[`app-release.yml`](../.github/workflows/app-release.yml) checks the version. It builds Windows and Linux for x86-64 and ARM
twice and fails if the bytes differ. It attaches the four files and `SHA256SUMS`, attests them, and drafts the release, which a
person reads and publishes. A tag with a pre-release part, such as `-beta.1`, becomes a pre-release, which never takes the
repository's "latest release" from the scripts. The app is not code-signed yet, so Windows SmartScreen warns about it.

Anyone can rebuild a release and compare the bytes. Use the Go version in `go.mod` (1.27.3) and the release's tag, from any kind
of clone or a source archive:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags='-s -w -buildid=' -o mysetup-scan-linux-x86-64 ./cmd/mysetup-scan
```

`-buildvcs=false` keeps the clone's state out of the file. Since Go 1.24, a build otherwise stamps a version from the nearest tag,
which a shallow clone does not have. The first test tag, `app-v0.1.0-beta.1`, was built without it and was never published; its
files rebuild only from a shallow clone (it was deleted on the operator's word). `app-v0.1.0-beta.2` is the first test release,
`app-v0.1.0-beta.3` replaced it, and `app-v0.1.0-beta.4` replaces that.
