The third test release of the MySetup scan app, for Windows and Linux, replacing `app-v0.1.0-beta.3`. It reads your computer's
hardware (processor, graphics, motherboard, memory and storage, by name only) and shows the report on a page in your web browser,
with six numbered steps to add it to your build on [MySetup](https://mysetup.is). You check every part there before anything is
saved.

It is one file with nothing to install. It only reads: it sends nothing anywhere, writes no files and installs nothing.

**This version cannot send the report by itself.** You copy it and paste it on your build page; the page tells you how, step by
step. Sending it straight to your build with a code from your build page comes in a later version.

**Changed since beta.3**

- The page shows six numbered steps, from Copy report to Done, with a button that opens your builds on mysetup.is in a new tab.
  The terminal (`--cli`) shows the same steps.
- Started with `sudo` or as administrator, the app now opens the page in your browser too, never as root or administrator. With
  sudo it reads first, then gives up root and opens the page as you. As administrator on Windows, it starts itself again without
  administrator rights.

## Step by step on Windows

1. Download **mysetup-scan-windows-x86-64.exe** below. On a Windows computer with an ARM processor (such as a Snapdragon laptop),
   download **mysetup-scan-windows-arm64.exe** instead.
2. If your browser says the file "isn't commonly downloaded", keep it: in Edge, point at the warning, click the three dots `...`,
   click **Keep**, then **Show more**, then **Keep anyway**; in Chrome, click **Keep**.
3. Open your Downloads folder: press the Windows key and **E** together, then click **Downloads** on the left.
4. Double-click **mysetup-scan-windows-x86-64**.
5. Windows shows a blue box, "Windows protected your PC". Click **More info**, then **Run anyway**. Windows says this because the
   app is new and not yet code-signed.
6. A page opens in your web browser with your report. Follow the steps on that page.

## Step by step on Linux

1. Download **mysetup-scan-linux-x86-64** below. On an ARM computer, such as a Raspberry Pi 4 or 5 with a 64-bit system,
   download **mysetup-scan-linux-arm64** instead, and use that name in steps 4 and 5.
2. Open a terminal, the window where you type commands: press **Ctrl**, **Alt** and **T** together. That works on Ubuntu, Linux
   Mint and most others; if nothing opens, open your list of applications and search for **Terminal**.
3. Copy this line, paste it into the terminal with **Ctrl+Shift+V** (in a terminal, plain Ctrl+V does not paste), and press
   **Enter**. It takes the terminal to your Downloads folder, whatever it is called in your language:

   ```sh
   cd "$(xdg-user-dir DOWNLOAD 2>/dev/null || echo ~/Downloads)"
   ```

4. Do the same with this line. It allows the file to run as a program, which Linux does not allow for a downloaded file until you
   say so. You only need to do it once:

   ```sh
   chmod +x mysetup-scan-linux-x86-64
   ```

5. Do the same with this line, which starts the app:

   ```sh
   ./mysetup-scan-linux-x86-64
   ```

6. A page opens in your web browser with your report. Follow the steps on that page.

On a computer with no desktop, such as a server you reach over SSH, the app prints the report in the terminal instead, with the
same steps. With `sudo`, it can also read memory modules on systems without udev.

| Option | Does |
|---|---|
| `--cli` | Prints the report in the terminal instead of opening the page |
| `--no-browser` | Serves the page without opening a browser, and prints its address to open yourself |
| `--help`, `--version` | Shows the options, or the version (`0.1.0-beta.4`) |

## The page is served from your computer only

It listens on 127.0.0.1 alone, at an address with a random one-time token, and refuses any request without it. It also refuses
another site's name for 127.0.0.1, and a request from another site to end it. A content policy lets the page load nothing from
anywhere else. Details are in [docs/app.md](https://github.com/eddinn/mysetup-scanner/blob/app-v0.1.0-beta.4/docs/app.md).

## Check a download

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify mysetup-scan-linux-x86-64 --repo eddinn/mysetup-scanner
```

The attestation records that the file was built from this repository at this tag. Each file was built twice and the release was
refused unless the bytes were the same. You can also rebuild a file yourself and compare it. With Go 1.27.3, from this tag (any
clone, or the source archive):

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -trimpath -ldflags='-s -w -buildid=' -o mysetup-scan-linux-x86-64 ./cmd/mysetup-scan
sha256sum mysetup-scan-linux-x86-64
```

For the other files, set `GOOS` to `windows` or `linux` and `GOARCH` to `amd64` or `arm64`.

## Where it has been tested

Real machines: Windows 11 x86-64 without administrator rights, and Linux x86-64 (Debian 13), where the app's reports were identical
to the scripts'. A double-click on Windows and the terminal over SSH were tried there too. Run as root under sudo on a Linux
desktop, a test showed the app giving up root and opening the page as the person who ran sudo, with their own groups, desktop and
home. Starting again without administrator rights on Windows has not been tried yet, since it needs a UAC prompt. The ARM builds have
not run on a real machine yet, and their reports say `tested=no`: please check such a report before you paste it, and tell us how it
went. The app also ran on GitHub's Linux, Windows and macOS runners, which shows it runs there and gives a valid report, not that
real hardware reads correctly.

In a virtual machine or a container the report says so, and the app suggests running it on the host to include your real hardware.
Inside WSL it declines and sends you to the Windows app.

## What a report never contains

Usernames, hostnames, IP or MAC addresses, motherboard or system UUIDs, machine GUIDs, Windows SIDs, product keys, disk serial
numbers, file paths, file names, network names or other software. The report format is in
[docs/report-format.md](https://github.com/eddinn/mysetup-scanner/blob/app-v0.1.0-beta.4/docs/report-format.md).

Licence: Apache-2.0.
