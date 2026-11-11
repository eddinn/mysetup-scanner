The release candidate of the MySetup scan app 0.1.0, for Windows and Linux, replacing `app-v0.1.0-beta.4`. It reads your
computer's hardware (processor, graphics, motherboard, memory and storage, by name only) and shows the report on a page in your web
browser, with six numbered steps to add it to your build on [MySetup](https://mysetup.is). You check every part there before
anything is saved.

It is one file with nothing to install. It only reads: it sends nothing anywhere, writes no files and installs nothing.

**This version cannot send the report by itself.** You copy it and paste it on your build page; the page tells you how, step by
step. Sending it straight to your build with a code from your build page comes in a later version.

**Changed since beta.4**

- On Linux the download is now a `.tar.gz` with one folder in it, and the program in that folder is already allowed to run. You
  extract it and double-click the program: no terminal and no `chmod`.
- Started with no terminal on Linux (a double-click), the app says what goes wrong in your desktop's own message box, where
  before it said nothing.
- The terminal way is `--cli`, on every system.
- Every download is built by one script, `tools/build-release.sh`, which the project's CI also runs for every change. CI also
  checks the Linux download as you extract it.

## Two ways to start it

- **Double-click it** (the usual way): your report opens on a page in your web browser, with a Copy button and the steps.
- **In a terminal, with `--cli`**: the report is printed in the terminal, with the steps. This is the way on a server or over SSH.

Pick one and follow its steps to the end.

<details>
<summary><b>Windows: download and double-click</b></summary>

1. Download **mysetup-scan-windows-x86-64.exe** below. On a Windows computer with an ARM processor (such as a Snapdragon laptop),
   download **mysetup-scan-windows-arm64.exe** instead.
2. If your browser says the file "isn't commonly downloaded", keep it: in Edge, point at the warning, click the three dots `...`,
   click **Keep**, then **Show more**, then **Keep anyway**; in Chrome, click **Keep**.
3. Open your Downloads folder: press the Windows key and **E** together, then click **Downloads** on the left.
4. Double-click **mysetup-scan-windows-x86-64**.
5. Windows shows a blue box, "Windows protected your PC". Click **More info**, then **Run anyway**. Windows says this because the
   app is new and not yet code-signed.
6. A page opens in your web browser with your report. Follow the steps on that page.

If Windows says **Smart App Control** blocked the app and offers no way to run it, use the script
([v0.1.0](https://github.com/eddinn/mysetup-scanner/releases/tag/v0.1.0)) instead: Smart App Control blocks apps that are not
code-signed yet.

</details>

<details>
<summary><b>Linux: download, extract and double-click</b></summary>

1. Download **mysetup-scan-linux-x86-64.tar.gz** below. On an ARM computer, such as a Raspberry Pi 4 or 5 with a 64-bit system,
   download **mysetup-scan-linux-arm64.tar.gz** instead; its names below end in `arm64`.
2. Open your Downloads folder in your file manager (on Ubuntu, the **Files** app).
3. Right-click **mysetup-scan-linux-x86-64.tar.gz** and click **Extract Here** (some file managers call it **Extract**). A folder
   **mysetup-scan-linux-x86-64** appears.
4. Double-click that folder to open it, then double-click **mysetup-scan** in it. If a box asks whether to run it, click **Run**
   (some file managers call it **Execute**).
5. A page opens in your web browser with your report. Follow the steps on that page.

If nothing happens at step 4, your file manager does not start programs: follow **Linux: in a terminal** below instead.

</details>

<details>
<summary><b>Windows: in a terminal (<code>--cli</code>)</b></summary>

1. Download **mysetup-scan-windows-x86-64.exe** below (on ARM, **mysetup-scan-windows-arm64.exe**, and use that name in step 4).
2. Open PowerShell: press the Windows key, type **PowerShell**, and press **Enter**.
3. Copy this line, paste it into PowerShell (a right-click in the window pastes), and press **Enter**. It takes PowerShell to your
   Downloads folder:

   ```powershell
   cd (New-Object -ComObject Shell.Application).NameSpace('shell:Downloads').Self.Path
   ```

4. Do the same with this line, which starts the app. If a blue box says "Windows protected your PC", click **More info**, then
   **Run anyway**:

   ```powershell
   .\mysetup-scan-windows-x86-64.exe --cli
   ```

5. The report is printed in the window, from the line `mysetup-report 1` down to `mysetup-report-end`, with the steps under it.
   Select those lines with the mouse, press **Ctrl+C**, and follow the steps.

</details>

<details>
<summary><b>Linux: in a terminal, or over SSH (<code>--cli</code>)</b></summary>

1. Open a terminal: press **Ctrl**, **Alt** and **T** together, or search your applications for **Terminal**. Over SSH, your SSH
   session is the terminal.
2. Copy each line, paste it into the terminal with **Ctrl+Shift+V**, and press **Enter** after each. On ARM, write `arm64` instead
   of `x86-64`; if you already downloaded the file with your browser, skip the `curl` line:

   ```sh
   cd "$(xdg-user-dir DOWNLOAD 2>/dev/null || echo ~/Downloads)"
   curl -fLO https://github.com/eddinn/mysetup-scanner/releases/download/app-v0.1.0-rc.1/mysetup-scan-linux-x86-64.tar.gz
   tar -xzf mysetup-scan-linux-x86-64.tar.gz
   ./mysetup-scan-linux-x86-64/mysetup-scan --cli
   ```

3. The report is printed in the terminal, from the line `mysetup-report 1` down to `mysetup-report-end`, with the steps under it.
   Select those lines with the mouse, press **Ctrl+Shift+C**, and follow the steps.

With `sudo`, it also reads memory modules on systems without udev.

</details>

There is no Mac app yet; on a Mac, use the script ([v0.1.0](https://github.com/eddinn/mysetup-scanner/releases/tag/v0.1.0)).

| Option | Does |
|---|---|
| `--cli` | Prints the report in the terminal instead of opening the page |
| `--no-browser` | Serves the page without opening a browser, and prints its address to open yourself |
| `--help`, `--version` | Shows the options, or the version (`0.1.0-rc.1`) |

## The page is served from your computer only

It listens on 127.0.0.1 alone, at an address with a random one-time token, and refuses any request without it. It also refuses
another site's name for 127.0.0.1, and a request from another site to end it. A content policy lets the page load nothing from
anywhere else. Details are in [docs/app.md](https://github.com/eddinn/mysetup-scanner/blob/app-v0.1.0-rc.1/docs/app.md).

## Check a download

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify mysetup-scan-linux-x86-64.tar.gz --repo eddinn/mysetup-scanner
```

The attestation records that the file was built from this repository at this tag. Each file was built twice and the release was
refused unless the bytes were the same. You can also rebuild the files yourself and compare them, with Go 1.27.3, bash, GNU tar
and gzip, from a clone at this tag:

```sh
tools/build-release.sh dist
cat dist/SHA256SUMS
```

From the source archive (`.tar.gz`) instead, which has no Git history, run
`SOURCE_DATE_EPOCH=$(stat -c %Y go.mod) tools/build-release.sh dist`.

## Where it has been tested

This release candidate, on real machines: four Linux x86-64 servers (Debian 13), where the extracted program kept its permission to
run and gave valid reports with `--cli` as an ordinary user and as root, and when started with no terminal and no desktop; and
Windows 11 x86-64 with `--cli`. The Windows app works as beta.4 did, where a double-click, the page and running without
administrator rights were tried; starting again without administrator rights after being run as administrator has not been tried
yet. A double-click on a Linux desktop has not been tried on a real machine yet. The ARM builds have not run on a real machine
yet, and their reports say `tested=no`: please check such a report before you paste it, and tell us how it went. The app also ran
on GitHub's Linux, Windows and macOS runners and the project's own Linux runner, which shows it runs there and gives a valid report,
not that real hardware reads correctly.

In a virtual machine or a container the report says so, and the app suggests running it on the host to include your real hardware.
Inside WSL it declines and sends you to the Windows app.

## What a report never contains

Usernames, hostnames, IP or MAC addresses, motherboard or system UUIDs, machine GUIDs, Windows SIDs, product keys, disk serial
numbers, file paths, file names, network names or other software. The report format is in
[docs/report-format.md](https://github.com/eddinn/mysetup-scanner/blob/app-v0.1.0-rc.1/docs/report-format.md).

Licence: Apache-2.0.
