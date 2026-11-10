The second test release of the MySetup scan app, for Windows and Linux, replacing `app-v0.1.0-beta.2`. It reads the same hardware report as the scripts (CPU,
graphics, motherboard, memory and storage) and shows it on a page in your own browser, with buttons to copy it or save it as a
file. You then paste it into the Hardware section of your build on [MySetup](https://mysetup.is), where you review every part
before anything is saved.

It is one file with nothing to install. It only reads: nothing is sent anywhere, no files are written, nothing is installed, and it
needs no administrator or root.

**Changed since beta.2:** on Windows, `mysetup-scan.exe --cli` started by another program that takes its output, such as WSL or a
script redirecting it to a file, no longer waits for an Enter nobody can see. A double-click still keeps its window open with the
report copied when the app cannot open a browser, and opens the page otherwise.

**A beta.** When this beta was released (2026-11-07), mysetup.is did not yet accept a report from the app (`source=app/0.1.0`). The
site update that accepts it is being tested. Until it is live, use the report to check what the app reads, or use the scripts.

## Files

| File | For |
|---|---|
| `mysetup-scan-windows-x86-64.exe` | Windows 10 and 11 on Intel or AMD |
| `mysetup-scan-windows-arm64.exe` | Windows on ARM |
| `mysetup-scan-linux-x86-64` | Linux on Intel or AMD |
| `mysetup-scan-linux-arm64` | Linux on ARM, such as a Raspberry Pi 4 or 5 with a 64-bit system |
| `SHA256SUMS` | Checksums of all four |

## Run it

- **Windows:** double-click the file. The app is not code-signed yet, so SmartScreen warns about it: choose "More info", then
  "Run anyway". The page opens in your browser and the app's window closes by itself.
- **Linux:** `chmod +x mysetup-scan-linux-x86-64`, then `./mysetup-scan-linux-x86-64`. With a desktop, the page opens in your
  browser. Over SSH, with no desktop, or run as root, the app prints the report in the terminal and says why.

The page ends when you click Done, about 90 seconds after its tab closes, or after 15 minutes at most. Options:

| Option | Does |
|---|---|
| `--cli` | Prints the report in the terminal instead of opening the page |
| `--no-browser` | Serves the page without opening a browser, and prints its address to open yourself |
| `--help`, `--version` | Shows the options, or the version (`0.1.0-beta.3`) |

On Linux, run with `sudo`, the app can also read memory modules on a system without udev. It says when that would add something.

## The page is served from your computer only

It listens on 127.0.0.1 alone, at an address with a random one-time token, and refuses any request without it. It also refuses
another site's name for 127.0.0.1, and a request from another site to end it. A content policy lets the page load nothing from
anywhere else. Details are in [docs/app.md](https://github.com/eddinn/mysetup-scanner/blob/app-v0.1.0-beta.3/docs/app.md).

## Check a download

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify mysetup-scan-linux-x86-64 --repo eddinn/mysetup-scanner
```

The attestation records that the file was built from this repository at this tag. Each file was built twice and the release was
refused unless the bytes were the same.

You can also rebuild a file yourself and compare it. With Go 1.27.3, from this tag (any clone, or the source archive):

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags='-s -w -buildid=' -o mysetup-scan-linux-x86-64 ./cmd/mysetup-scan
sha256sum mysetup-scan-linux-x86-64
```

For the other files, set `GOOS` to `windows` or `linux` and `GOARCH` to `amd64` or `arm64`.

## Where it has been tested

Real machines: Windows 11 x86-64 without administrator rights, and Linux x86-64 (Debian 13), where the app's reports were identical
to the scripts'. Both front ends were tried there too: a double-click on Windows, and the terminal over SSH. The ARM builds have not
run on a real machine yet, and their reports say `tested=no`: please check such a report before you paste it, and tell us how it
went. The app also ran on GitHub's Linux, Windows and macOS runners, which shows it runs there and gives a valid report, not that
real hardware reads correctly.

In a virtual machine or a container the report says so, and the app suggests running it on the host to include your real hardware.
Inside WSL it declines and sends you to the Windows app.

## What a report never contains

Usernames, hostnames, IP or MAC addresses, motherboard or system UUIDs, machine GUIDs, Windows SIDs, product keys, disk serial
numbers, file paths, file names, network names or other software. The report format is in
[docs/report-format.md](https://github.com/eddinn/mysetup-scanner/blob/app-v0.1.0-beta.3/docs/report-format.md).

Licence: Apache-2.0.
