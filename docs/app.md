# The app

A Go program that reads the same report as the scripts ([report-format.md](report-format.md)), with `source=app/VERSION`, and
shows it on a page in your own browser, or in the terminal with `--cli`. The design and the operator's decisions are in MySetup's
`docs/scanner/design.md` ("The app: the operator's decisions"). It is built in stages. Stage 1 is reading and the two front ends.
Stage 2 is sending: the site's codes, upload route and checks are MySetup's, and the app sends its report with a code from the
build page ([Sending](#sending)), on the page and in the terminal, since 0.2.0 (not released yet: the operator's steps 4 and 5,
a test on staging and his approval, come first). Code signing comes after stage 2. The current release is `app-v0.1.0`, the first
stable one (2026-11-12), which cannot send. The member's steps, for each system and both ways of starting it (double-click, or a
terminal with `--cli`), are in the [README](../README.md); they describe the current release.

## The page and the terminal

| How it is started | What it does |
|---|---|
| Double-clicked, or run with no option | Opens a page in your default browser with the report and four numbered steps to send it to the build: get a code on the build page, type it and click Send to my build, open the review, Done. The six steps to copy and paste it instead are in a fold under them |
| `mysetup-scan --cli` | Prints the report in the terminal, always: the report on standard output, and everything said around it on standard error. Then, where it reads and writes a terminal, it gives the three steps to send it and asks for the code; Enter without one sends nothing and gives the five steps to copy and paste it. Where it cannot ask (its input redirected, or run by another program), it gives the copy steps and waits for nothing |
| `mysetup-scan --no-browser` | Serves the page without opening a browser, and prints its address to open yourself |
| There is no desktop | Prints the report in the terminal with the steps, and says why: over SSH or with no desktop (no `DISPLAY` or `WAYLAND_DISPLAY`), or no `xdg-open` |
| Double-clicked on Linux or another Unix-like system | As with no option. It has no terminal to print to, so what goes wrong is said in the desktop's own message box |

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

Started with no terminal on Linux and the other Unix-like systems (a double-click in a file manager), the app finds that its
error output is not a terminal. It then says what goes wrong in the desktop's own message box, with the first of GNOME's
`zenity`, KDE's `kdialog` or a `notify-send` notification that works, taken only from `/usr/bin`, `/usr/local/bin` or `/bin` and
never started as root. None of them is required; with none, the message goes where the error output goes, as before. It says so when it could not give up root, its page could not start, the browser did not open
(with the page's address to open), it found no desktop to open the page in (with the `--cli` way), or it is in WSL.

Double-clicked on Windows, Windows opens a console window for the app alone. The app starts again without one and that window
closes, so only the browser page is left. This works the same in the classic console and in Windows Terminal. If it cannot start
again, it carries on in the window. Without a console it has nowhere to print, so it reports a problem in a message box. Run with
`--cli` in a window of its own, it asks for the code as in any terminal, then, unless the report was sent, copies it to the
clipboard, and waits for Enter before the window closes. It counts a
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
- Done and Send, the two requests that do anything, are POSTs accepted only with the page's own `Origin` (and `Sec-Fetch-Site:
  same-origin` where the browser sends it). Another site's request cannot end the app or send anything. Send also needs
  `Content-Type: application/json`, which a form on another site cannot set without asking first, takes a body of 1 KB at most,
  and takes only the code from the page (see [Sending](#sending)).
- A content policy lets the page load nothing from anywhere else: `default-src 'none'`, with its one style and one script allowed
  by a fresh nonce for each response, and no framing. It is also sent with `nosniff`, `no-referrer`, `no-store` and same-origin
  opener and resource policies.
- The report and notes are written through Go's `html/template`, which escapes them.
- The server has read, write and header time limits, and a 16 KB header limit.

## Sending

The app sends its report only when the member types a code from their build page: typing it is the approval (MySetup's
`docs/scanner/design.md`, Pairing). Without a code nothing is sent, and the copy-and-paste way works as before.
[internal/send](../internal/send/send.go) does the sending:

- **Where:** a release build sends only to `https://mysetup.is/api/scanner/reports`, fixed in the program
  ([site_release.go](../internal/send/site_release.go)); nothing a member types or a page sends can change it. It follows no
  redirect, gives up after 30 seconds, and reads at most 64 KB of the answer.
- **What:** one HTTPS POST of JSON `{"code": "...", "report": "..."}` with the report exactly as the page and `--cli` show it, and
  the headers `Content-Type` and `Accept: application/json` and `User-Agent: MySetup-Scanner/<version>`. No cookie, no other
  data. A code is checked first: 8 letters and digits (spaces and dashes are ignored, as the site ignores them); anything else is
  never sent.
- **The answer:** `201` gives the review's address, taken only when it is on the same site and under `/setups/` (with no `..`
  or encoded slash to climb out of it). `422` is the
  site's own words for the member (a wrong, used or expired code, or a report it refuses), shown as they are, cut to 300
  printable characters. `429` is "too many tries", `413` "too large", `404` "sending is not open on MySetup yet" (the site's
  switch is off), and anything else an error with its status. A network error says MySetup could not be reached.
- **On the page:** the code goes from the page to the app (`POST send`, checked as above), and the app sends the report it
  serves, never anything from the page but the code. One send at a time. The page then shows a button to open the review.
- **In the terminal:** the app asks only when both its input and its error output are a terminal (on Windows, a console): run by
  a script, with its input redirected, or from WSL, it never waits for an answer nobody can give. It takes up to five codes,
  saying after each what came of it, and Enter without a code stops.

A development build, `go build -tags dev`, sends to MySetup's staging site instead, or to the site `MYSETUP_SCAN_SITE` names when
that is `https://staging.mysetup.is` or `http://127.0.0.1` or `http://localhost` with a port, for a test
([site_dev.go](../internal/send/site_dev.go)); any other value is ignored. It says it is a development build, and names the site
in its steps. `tools/build-release.sh` refuses a program that names staging or `MYSETUP_SCAN_SITE`, so a development build can
never become a download.

## Every system, nothing to install

One file per system and processor, built without C (`CGO_ENABLED=0`), so it starts as it is: no runtime, no PowerShell, no Python,
no package, and no third-party code (only Go and its official `golang.org/x/sys`). It runs without administrator or root. On Linux
and the BSDs, root lets it also read what the system keeps from ordinary users. It never asks for root by itself; it says what more
root would read.

| System | How it reads | Root or administrator adds |
|---|---|---|
| Windows | The firmware's SMBIOS table (`GetSystemFirmwareTable`), the processor's registry entry, the display adapters SetupAPI lists, each disk's storage queries through a handle with no access rights | Nothing known |
| Linux | `/proc`, `/sys` (PCI, DMI, block devices), `os-release`, udev's database; the system's own `pci.ids` for graphics names where it has one, and its own `lscpu` for the processor's name where `/proc/cpuinfo` gives none (ARM), as the script reads it | Memory modules where udev is missing, from the firmware table in `/sys/firmware/dmi/tables` |
| macOS | `sysctl` directly; Apple's own `system_profiler` and `diskutil` | Nothing expected |
| FreeBSD | `sysctl` directly; the base system's `kenv`, `pciconf` and `geom` | Memory modules, from the firmware table in `/dev/mem` |
| OpenBSD, NetBSD | `sysctl` directly | Memory modules where the system hands the firmware table to root |
| illumos | Its own `psrinfo`, `prtconf` and `smbios` | As FreeBSD |

A system's own tool is run only from `/usr/sbin`, `/usr/bin`, `/sbin` or `/bin`, never found through `PATH`, with fixed arguments,
no shell, a time limit and a size limit on what it prints. The browser opener is the system's own too: `ShellExecute` on Windows,
`/usr/bin/open` on macOS, and `xdg-open` from `/usr/bin`, `/usr/local/bin` or `/bin` elsewhere, as are the message tools for a
start with no terminal. Sources that also hold serial
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

On 2026-11-11 the release candidate `0.1.0-rc.1`, built by `tools/build-release.sh`, ran on five real machines:

- Linux x86-64, Debian 13 with udev, four servers (pve0, pve1, pve2 and the backup server pbs, with Intel Core i5-13600K,
  i7-7700K, Core Ultra 9 285H and i7 950 processors): the `.tar.gz` extracted with the program still executable, and the reports were valid with `--cli` as an ordinary
  user and as root, and started with no terminal and no desktop, which printed the report with its reason. Each ended with exit
  code 0.
- Windows 11 x86-64 (the computer above, Windows build 26300): `--version` and `--cli` gave a valid report, exit code 0. The
  Windows app works as beta.4 did; only its version changed.

On GitHub's ARM runners (virtual machines on Microsoft's Cobalt 100 processors) the ARM builds ran too: on Windows 11 and Ubuntu
24.04 for arm64 they gave valid reports, and the published Linux arm64 `.tar.gz` worked as extracted. That run found that the app
named no processor on Linux ARM, where `/proc/cpuinfo` gives none: it now asks the system's `lscpu`, as the script does. No real
ARM computer has run the app yet.

On 2026-11-12 the operator tried `0.1.0-rc.1` on a Linux desktop, his laptop with Ubuntu 26.04 and GNOME (an MSI laptop with an
Intel Core i7-9750H, Intel and NVIDIA graphics, two DDR4 modules and an NVMe disk), as a member would, from the build page on
mysetup.is. The page offered the download for Linux by itself. Extracted and double-clicked, the program started and opened its
page in a browser tab; Copy report and Done worked, and Done ended the app. A double-click on the `.tar.gz` in Ubuntu's Files
unpacked it into a folder of its own, so the README's steps use that, with right-click Extract for other file managers. The report was valid (`tested=yes`,
`physical`) and held nothing private.

On 2026-11-12 sending (0.2.0, not released) was checked on the operator's Windows 11 computer with `tools/check-send.sh` and a
Windows development build, run from WSL (whose network is mirrored with Windows, so both share 127.0.0.1) against the stand-in
site `tests/fake_site.py`: the page's send from another site was refused and reached no site, a wrong code came back in the
site's words, the right code sent exactly the report `--cli` printed and gave the review's address, and Done ended the app.
Started from WSL's terminal, the Windows program's input is a pipe, so `--cli` gave the copy steps and asked nothing.

The other systems are built and vetted for every target, and run on GitHub's macOS
runner, but no real machine of them has passed yet. The paths that need root (the firmware table on Linux without udev, `/dev/mem` on FreeBSD) are tested on a made-up table only.

## Build and test

Go 1.27 or newer:

```sh
go vet ./...
go test ./...
go test -tags dev ./...
CGO_ENABLED=0 go build -buildvcs=false -trimpath -ldflags='-s -w -buildid=' -o mysetup-scan ./cmd/mysetup-scan
./mysetup-scan --cli < /dev/null > report.txt
python3 -B tests/validate_report.py report.txt
./mysetup-scan
```

The last line opens the page. `< /dev/null` keeps `--cli` from asking for a code. Sending is checked end to end, on this
computer only, with a development build and a stand-in for MySetup (Python 3 and curl):

```sh
tools/check-send.sh
```

```text
The terminal asked for the code and sent the report.
Sending works end to end: refused from another site, the site's words for a wrong code, the same report for the right one.
```

The first line comes only on Linux, where `script` gives the app a terminal to ask in. From WSL, a Windows development build is
checked the same way (WSL itself is declined by the app):

```sh
GOOS=windows GOARCH=amd64 go build -tags dev -trimpath -o scan-dev.exe ./cmd/mysetup-scan
WSLENV=MYSETUP_SCAN_SITE tools/check-send.sh ./scan-dev.exe
``` For another system, set `GOOS` and `GOARCH`, such as `GOOS=windows GOARCH=amd64` for
`mysetup-scan.exe`. `tools/build-release.sh dist` builds the downloads exactly as a release does (see Releases). Two workflows check
every change. On MySetup's own Git server and its Linux runner, [`.forgejo/workflows/app.yml`](../.forgejo/workflows/app.yml) vets
every target and builds every target twice, failing if the bytes differ. It lints and runs `tools/build-release.sh`, checks the
Linux x86-64 `.tar.gz` as a member extracts it with `tools/check-linux-download.sh` (one folder, the program executable, its
version, a valid report), runs the development build's tests and `tools/check-send.sh`, and keeps the downloads as a workflow
artifact for 14 days. On GitHub, the mirror that receives every
push, [`.github/workflows/app.yml`](../.github/workflows/app.yml) covers what that server has no runners for: Windows and macOS,
and Windows and Linux on ARM processors, where it also checks the arm64 `.tar.gz` as extracted. GitHub's ARM runners are free
for public repositories; like its others they are virtual machines, so they show the ARM programs run and give valid reports,
not that real ARM hardware reads correctly. On each system the tests run, the app runs with `--cli` and its report is checked,
and the page is served with
`--no-browser` and checked as a browser would use it: the download matches the report, another site's Done is refused, and the
page's own Done ends the app. Then `tools/check-send.sh` checks sending there.

## Releases

A release starts from a signed tag `app-v<version>`, such as `app-v0.1.0`, matching `Version` and `Prerelease` in
[internal/scan/hardware.go](../internal/scan/hardware.go), with its notes in `docs/releases/`. The workflow
[`app-release.yml`](../.github/workflows/app-release.yml) checks the version and runs
[`tools/build-release.sh`](../tools/build-release.sh), which the homelab's CI also runs for every push. That script builds Windows
and Linux for x86-64 and ARM twice and fails if the bytes differ, or if a program names any MySetup but `https://mysetup.is` (a
development build). Windows gets the program itself
(`mysetup-scan-windows-x86-64.exe`). Linux gets a `.tar.gz` (`mysetup-scan-linux-x86-64.tar.gz`) holding one folder with the program
`mysetup-scan`, already executable, and a short `README.txt`. A browser download never keeps the permission to run, but an
extracted archive does, so the program can be double-clicked. The workflow attaches the four files and `SHA256SUMS`, attests them,
and drafts the release, which a person reads and publishes. A tag with a pre-release part, such as `-rc.1`, becomes a pre-release,
which never takes the repository's "latest release" from the scripts. The app is not code-signed yet (the operator's decision of
2026-11-11: unsigned until stage 2 is stable), so Windows SmartScreen warns about it.

Anyone can rebuild a release and compare the bytes. Use the Go version in `go.mod` (1.27.3), bash, GNU tar and gzip, and the
release's tag, from a clone of any depth:

```sh
git checkout app-v0.1.0
tools/build-release.sh dist
cat dist/SHA256SUMS
```

The archives' file times are the commit's, so the `.tar.gz` files are the same bytes too. A source archive has no Git history to
take that time from; GitHub's `.tar.gz` source archive keeps the commit's time as its files' times, so from it run
`SOURCE_DATE_EPOCH=$(stat -c %Y go.mod) tools/build-release.sh dist`. One program alone builds with:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags='-s -w -buildid=' -o mysetup-scan ./cmd/mysetup-scan
```

`-buildvcs=false` keeps the clone's state out of the file. Since Go 1.24, a build otherwise stamps a version from the nearest tag,
which a shallow clone does not have. The first test tag, `app-v0.1.0-beta.1`, was built without it and was never published; its
files rebuild only from a shallow clone (it was deleted on the operator's word). `app-v0.1.0-beta.2` is the first test release,
`app-v0.1.0-beta.3` and `app-v0.1.0-beta.4` replaced it in turn, and `app-v0.1.0-rc.1`, the first with the Linux `.tar.gz`, was
the release candidate. `app-v0.1.0` (2026-11-12) is the first stable release: rc.1 with the processor's name on Linux ARM.
