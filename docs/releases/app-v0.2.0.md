Version 0.2.0 of the MySetup scan app, for Windows and Linux: **it sends your report to your build with a code from your build
page.** It reads your computer's hardware (the computer's own model, processor, graphics, motherboard, memory and storage, by name
only) and shows the report on a page in your web browser, or in the terminal with `--cli`. You check every part on
[MySetup](https://mysetup.is) before anything is saved.

**What it sends, and when:** only when you type a code from your build page, only to mysetup.is over HTTPS, and only the report you
see on the page or in the terminal. The code works once, for 10 minutes, and ties the report to your build. Without a code it
sends nothing, and you can copy and paste the report as before. It writes no files and installs nothing.

**New since 0.1.0**

- **Send it to your build:** on the app's page, type the code in the box under **Send it to your build** and click **Send to my
  build**, then **Open the review**; with `--cli` in a terminal, the app asks for the code after the report, and Enter without
  one gives the copy-and-paste steps. A report the app sends can be marked **Verified** on your build, where a pasted one is
  Detected at most.
- **The computer's own model:** the report now names the computer itself, as its firmware does (a laptop's or a prebuilt's maker
  and model, family, version and SKU, and its chassis, such as `notebook` or `desktop`), on a physical computer only. Its serial
  number, UUID and asset tag are never read. A built computer's placeholders ("System Product Name") are left out.

## Last: send it with a code

1. In your web browser, go to [mysetup.is/setups](https://mysetup.is/setups) and sign in if it asks.
2. Click your build. No build yet? Click **Create setup** first.
3. Scroll down to **Hardware** and find **Scan this computer**. Open **With the MySetup app** and click **Get a code**. The code
   works once, for 10 minutes.
4. Type the code into the app: on its page in the box under **Send it to your build**, then click **Send to my build**; in the
   terminal where it asks, then press **Enter**. It sends the report you see to mysetup.is, and nothing else.
5. Open the review: on the page, click **Open the review**; in the terminal, open the address it prints. Check each part, then
   click **Add these parts**. Nothing changes in your build until you do.

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
([v0.2.0](https://github.com/eddinn/mysetup-scanner/releases/tag/v0.2.0)) instead: Smart App Control blocks apps that are not
code-signed yet.

</details>

<details>
<summary><b>Linux: download, extract and double-click</b></summary>

1. Download **mysetup-scan-linux-x86-64.tar.gz** below. On an ARM computer, such as a Raspberry Pi 4 or 5 with a 64-bit system,
   download **mysetup-scan-linux-arm64.tar.gz** instead; its names below end in `arm64`.
2. Open your Downloads folder in your file manager (on Ubuntu, the **Files** app).
3. Double-click **mysetup-scan-linux-x86-64.tar.gz**. Ubuntu's Files unpacks it into a new folder,
   **mysetup-scan-linux-x86-64**. If your file manager shows what is inside the file instead, right-click the file and click
   **Extract Here** (some file managers call it **Extract**).
4. Double-click the folder **mysetup-scan-linux-x86-64** to open it, then double-click **mysetup-scan** in it. If a box asks
   whether to run it, click **Run** (some file managers call it **Execute**).
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
   The app then asks for a code: get one as in **Last: send it with a code** above, type it and press **Enter**. Or press
   **Enter** without a code, select the report's lines with the mouse, press **Ctrl+C**, and follow the steps to paste it.

</details>

<details>
<summary><b>Linux: in a terminal, or over SSH (<code>--cli</code>)</b></summary>

1. Open a terminal: press **Ctrl**, **Alt** and **T** together, or search your applications for **Terminal**. Over SSH, your SSH
   session is the terminal.
2. Copy each line, paste it into the terminal with **Ctrl+Shift+V**, and press **Enter** after each. On ARM, write `arm64` instead
   of `x86-64`; if you already downloaded the file with your browser, skip the `curl` line:

   ```sh
   cd "$(xdg-user-dir DOWNLOAD 2>/dev/null || echo ~/Downloads)"
   curl -fLO https://github.com/eddinn/mysetup-scanner/releases/download/app-v0.2.0/mysetup-scan-linux-x86-64.tar.gz
   tar -xzf mysetup-scan-linux-x86-64.tar.gz
   ./mysetup-scan-linux-x86-64/mysetup-scan --cli
   ```

3. The report is printed in the terminal, from the line `mysetup-report 1` down to `mysetup-report-end`, with the steps under it.
   The app then asks for a code: get one as in **Last: send it with a code** above, type it and press **Enter**. Or press
   **Enter** without a code, select the report's lines with the mouse, press **Ctrl+Shift+C**, and follow the steps to paste it.

With `sudo`, it also reads memory modules on systems without udev.

</details>

There is no Mac app yet; on a Mac, use the script ([v0.2.0](https://github.com/eddinn/mysetup-scanner/releases/tag/v0.2.0)).

| Option | Does |
|---|---|
| `--cli` | Prints the report in the terminal instead of opening the page |
| `--no-browser` | Serves the page without opening a browser, and prints its address to open yourself |
| `--help`, `--version` | Shows the options, or the version (`0.2.0`) |

## The page is served from your computer only

It listens on 127.0.0.1 alone, at an address with a random one-time token, and refuses any request without it. It also refuses
another site's name for 127.0.0.1, and a request from another site to end it. A content policy lets the page load nothing from
anywhere else. Details are in [docs/app.md](https://github.com/eddinn/mysetup-scanner/blob/app-v0.2.0/docs/app.md).

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

On real machines:

- **Sending**, on the operator's laptop (Ubuntu 26.04, an MSI GF63 Thin), with a test build of this version sending to MySetup's
  staging site (2026-11-13): the code was taken, the report arrived on the build, and the review showed the laptop itself, its
  parts and their marks.
- **A Linux desktop**, Ubuntu 26.04 with GNOME on a laptop, as a member would, from the build page on mysetup.is (with
  rc.1): the page offered the Linux download by itself; extracted and double-clicked, the app opened its page in the browser; Copy
  report and Done worked, and the report was valid.
- **Four Linux servers**, Debian 13 x86-64: the extracted program kept its permission to run, and gave valid reports with `--cli`
  as an ordinary user and as root, and when started with no terminal and no desktop.
- **Windows 11** x86-64: valid reports with `--cli`. A double-click, the page and running without administrator rights were
  tried on earlier test releases, and the Windows app works as they did. Starting again without administrator rights after being
  run as administrator has not been tried yet.

The ARM builds ran on GitHub's ARM runners (Windows 11 and Ubuntu 24.04, virtual machines), not yet on a real ARM computer, and
their reports say `tested=no`: please check such a report before you paste it, and tell us how it went. The app also ran on
GitHub's Windows and macOS runners and the project's own Linux runner, which shows it runs there and gives a valid report, not
that real hardware reads correctly.

In a virtual machine or a container the report says so, and the app suggests running it on the host to include your real hardware.
Inside WSL it declines and sends you to the Windows app.

## What a report never contains

Usernames, hostnames, IP or MAC addresses, motherboard or system UUIDs, machine GUIDs, Windows SIDs, product keys, serial numbers of
the computer, its chassis or any part, asset tags, file paths, file names, network names or other software. The report format is in
[docs/report-format.md](https://github.com/eddinn/mysetup-scanner/blob/app-v0.2.0/docs/report-format.md).

Licence: Apache-2.0.
