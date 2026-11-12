# MySetup scan

Tells [MySetup](https://mysetup.is) what hardware your computer has. MySetup offers three ways to scan, on your build page:

| | Browser | Script | App |
|---|---|---|---|
| Needs | Nothing | Nothing installed | A download |
| Detail | Least | Full | Full |
| Best mark | Detected | Detected | Verified, once sending is built |

This repository holds the app and the script. Both read the same report of your computer's hardware: the processor, graphics,
motherboard, memory and storage, by name only. You check it on your build page before anything is saved.

- **The app** is the easiest: download it, double-click it, and your report opens on a page in your web browser with a Copy
  button. Its release for Windows and Linux is
  [app-v0.1.0](https://github.com/eddinn/mysetup-scanner/releases/tag/app-v0.1.0).
- **The script** is a text file you can read before you run it, for every system including macOS. Its release is
  [v0.1.0](https://github.com/eddinn/mysetup-scanner/releases/tag/v0.1.0).

Neither sends anything anywhere. You copy the report and paste it on your build page ([the last steps](#last-paste-it-on-your-build)).
Sending it straight to your build with a code from your build page comes in a later version of the app.

Below, click a line to open its steps.

## The app, step by step

One file with nothing to install. It only reads: it sends nothing, writes no files and installs nothing. Its page is served from
your own computer only, and the app stops when you click Done, close the page, or after 15 minutes.

There are two ways to start it. Pick one and follow its steps to the end:

- **Double-click it** (the usual way): your report opens on a page in your web browser, with a Copy button and the steps.
- **In a terminal, with `--cli`**: the report is printed in the terminal, with the steps. This is the way on a server or over SSH.

<details>
<summary><b>Windows: download and double-click</b></summary>

1. Download the app: [mysetup-scan-windows-x86-64.exe](https://github.com/eddinn/mysetup-scanner/releases/download/app-v0.1.0/mysetup-scan-windows-x86-64.exe).
   On a Windows computer with an ARM processor (such as a Snapdragon laptop), download
   [mysetup-scan-windows-arm64.exe](https://github.com/eddinn/mysetup-scanner/releases/download/app-v0.1.0/mysetup-scan-windows-arm64.exe) instead.
2. If your browser says the file "isn't commonly downloaded", keep it:
   - in Edge, point at the warning, click the three dots `...`, click **Keep**, then **Show more**, then **Keep anyway**;
   - in Chrome, click **Keep**.
3. Open your Downloads folder: press the Windows key and **E** together, then click **Downloads** on the left.
4. Double-click **mysetup-scan-windows-x86-64**.
5. Windows shows a blue box, "Windows protected your PC". Click **More info**, then **Run anyway**. Windows says this because the app
   is new and not yet code-signed.
6. A page opens in your web browser with your report. Follow the steps on that page.

If Windows says **Smart App Control** blocked the app and offers no way to run it, use [the script](#the-script-step-by-step)
instead: Smart App Control blocks apps that are not code-signed yet.

</details>

<details>
<summary><b>Linux: download, extract and double-click</b></summary>

1. Download the app: [mysetup-scan-linux-x86-64.tar.gz](https://github.com/eddinn/mysetup-scanner/releases/download/app-v0.1.0/mysetup-scan-linux-x86-64.tar.gz).
   On an ARM computer, such as a Raspberry Pi 4 or 5 with a 64-bit system, download
   [mysetup-scan-linux-arm64.tar.gz](https://github.com/eddinn/mysetup-scanner/releases/download/app-v0.1.0/mysetup-scan-linux-arm64.tar.gz) instead; its names below end in `arm64`.
2. Open your Downloads folder in your file manager (on Ubuntu, the **Files** app).
3. Right-click **mysetup-scan-linux-x86-64.tar.gz** and click **Extract Here** (some file managers call it **Extract**). A folder
   **mysetup-scan-linux-x86-64** appears.
4. Double-click that folder to open it, then double-click **mysetup-scan** in it. If a box asks whether to run it, click **Run**
   (some file managers call it **Execute**).
5. A page opens in your web browser with your report. Follow the steps on that page.

The program in the download is already allowed to run; you do not need to change anything. If nothing happens at step 4, your file
manager does not start programs: follow **Linux: in a terminal** below instead. If something goes wrong, the app says so in a
message box.

</details>

<details>
<summary><b>Windows: in a terminal (<code>--cli</code>)</b></summary>

1. Download the app: [mysetup-scan-windows-x86-64.exe](https://github.com/eddinn/mysetup-scanner/releases/download/app-v0.1.0/mysetup-scan-windows-x86-64.exe) (on ARM,
   [mysetup-scan-windows-arm64.exe](https://github.com/eddinn/mysetup-scanner/releases/download/app-v0.1.0/mysetup-scan-windows-arm64.exe), and use that name in step 4). If your browser warns
   about it, keep it as in step 2 of **Windows: download and double-click**.
2. Open PowerShell: press the Windows key, type **PowerShell**, and press **Enter**. A window opens where you can type.
3. Copy this line, paste it into PowerShell (a right-click in the window pastes), and press **Enter**. It takes PowerShell to your
   Downloads folder, wherever Windows keeps it:

   ```powershell
   cd (New-Object -ComObject Shell.Application).NameSpace('shell:Downloads').Self.Path
   ```

4. Do the same with this line, which starts the app:

   ```powershell
   .\mysetup-scan-windows-x86-64.exe --cli
   ```

   If a blue box says "Windows protected your PC", click **More info**, then **Run anyway**.
5. The report is printed in the window, from the line `mysetup-report 1` down to `mysetup-report-end`, with the steps under it.
   Select those lines with the mouse and press **Ctrl+C** to copy them. Go on with [the last steps](#last-paste-it-on-your-build).

</details>

<details>
<summary><b>Linux: in a terminal, or over SSH (<code>--cli</code>)</b></summary>

1. Open a terminal, the window where you type commands: press **Ctrl**, **Alt** and **T** together. That works on Ubuntu, Linux Mint
   and most others; if nothing opens, open your list of applications and search for **Terminal**. Over SSH, your SSH session is
   the terminal.
2. Copy each line below, paste it into the terminal with **Ctrl+Shift+V** (in a terminal, plain Ctrl+V does not paste), and press
   **Enter** after each. The first goes to your Downloads folder, whatever it is called in your language. The second downloads
   the app. The third unpacks it. The fourth starts it:

   ```sh
   cd "$(xdg-user-dir DOWNLOAD 2>/dev/null || echo ~/Downloads)"
   curl -fLO https://github.com/eddinn/mysetup-scanner/releases/download/app-v0.1.0/mysetup-scan-linux-x86-64.tar.gz
   tar -xzf mysetup-scan-linux-x86-64.tar.gz
   ./mysetup-scan-linux-x86-64/mysetup-scan --cli
   ```

   On an ARM computer, write `arm64` instead of `x86-64` in the last three lines. If you already downloaded the file with your
   browser, skip the `curl` line.
3. The report is printed in the terminal, from the line `mysetup-report 1` down to `mysetup-report-end`, with the steps under it.
   Select those lines with the mouse and press **Ctrl+Shift+C** to copy them. Go on with
   [the last steps](#last-paste-it-on-your-build).

Started with `sudo` (`sudo ./mysetup-scan-linux-x86-64/mysetup-scan --cli`), it also reads memory modules on systems without udev.

</details>

<details>
<summary><b>Mac</b></summary>

There is no Mac app yet. Use [the script](#the-script-step-by-step) (**On a Mac, in Terminal**).

</details>

| Option | Does |
|---|---|
| `--cli` | Prints the report in the terminal instead of opening the page |
| `--no-browser` | Serves the page without opening a browser, and prints its address to open yourself |
| `--help`, `--version` | Shows the options, or the version |

Started with `sudo` without `--cli`, the app reads first, then gives up root and opens the page in your browser as you. On a
computer with no desktop, such as a server you reach over SSH, it prints the report in the terminal, as `--cli` does.

### Check an app download

Each release carries `SHA256SUMS` and GitHub's attestation that its files were built from this repository at that tag:

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify mysetup-scan-linux-x86-64.tar.gz --repo eddinn/mysetup-scanner
```

Anyone can also rebuild the files and compare the bytes ([docs/app.md](docs/app.md#releases)). How the app reads each system, how
its page is hardened, and what has been tested are in [docs/app.md](docs/app.md).

## The script, step by step

A text file that prints the report. Please read it before you run it; it is meant to be read. It only reads: it uses no network,
writes no files, installs nothing and does not ask for root unless you say so. There is deliberately no pipe-to-shell one-liner,
since that would skip the reading. Your build page on MySetup serves the same files as the
[release](https://github.com/eddinn/mysetup-scanner/releases/tag/v0.1.0).

<details>
<summary><b>Windows, in PowerShell</b></summary>

1. Download [mysetup-scan.ps1](https://github.com/eddinn/mysetup-scanner/releases/download/v0.1.0/mysetup-scan.ps1).
2. Open PowerShell: press the Windows key, type **PowerShell**, and press **Enter**. A window opens where you can type.
3. Copy this line, paste it into PowerShell (a right-click in the window pastes), and press **Enter**. It takes PowerShell to
   your Downloads folder, wherever Windows keeps it:

   ```powershell
   cd (New-Object -ComObject Shell.Application).NameSpace('shell:Downloads').Self.Path
   ```

4. Do the same with this line. It runs the script and copies its report for you, so the window shows nothing; that is right:

   ```powershell
   powershell -ExecutionPolicy Bypass -File .\mysetup-scan.ps1 | Set-Clipboard
   ```

   Windows blocks unsigned scripts from the internet by default. `-ExecutionPolicy Bypass` lifts that for this one run and changes
   no setting. A work computer may still refuse it.
5. The report is now copied. Go on with [the last steps](#last-paste-it-on-your-build).

</details>

<details>
<summary><b>Mac, in Terminal</b></summary>

1. Download [mysetup-scan.sh](https://github.com/eddinn/mysetup-scanner/releases/download/v0.1.0/mysetup-scan.sh).
2. Open Terminal: press **Cmd** and the **space bar** together, type **Terminal**, and press **Return**.
3. Copy this line, paste it into Terminal with **Cmd+V**, and press **Return**. It takes Terminal to your Downloads folder:

   ```sh
   cd ~/Downloads
   ```

4. Do the same with this line. It runs the script and copies its report for you:

   ```sh
   sh mysetup-scan.sh | pbcopy
   ```

5. The report is now copied, even though Terminal still shows a line saying to copy it. Go on with
   [the last steps](#last-paste-it-on-your-build).

</details>

<details>
<summary><b>Linux and other Unix-like systems</b></summary>

1. Download [mysetup-scan.sh](https://github.com/eddinn/mysetup-scanner/releases/download/v0.1.0/mysetup-scan.sh).
2. Open a terminal: press **Ctrl**, **Alt** and **T** together, or search your applications for **Terminal**.
3. Copy each line, paste it into the terminal with **Ctrl+Shift+V**, and press **Enter**. The first goes to your Downloads folder;
   the second runs the script and puts its report in a file there; the third opens that file in your text editor:

   ```sh
   cd "$(xdg-user-dir DOWNLOAD 2>/dev/null || echo ~/Downloads)"
   sh mysetup-scan.sh > mysetup-report.txt
   xdg-open mysetup-report.txt
   ```

4. In the text editor, press **Ctrl+A** to select everything, then **Ctrl+C** to copy it. Go on with
   [the last steps](#last-paste-it-on-your-build).

If the script says something it could use is missing, it prints the command to install it and leaves that to you. If it says
"untested on this system", check the report before you paste it.

</details>

<details>
<summary><b>A computer with no desktop, over SSH</b></summary>

Download and run it there, then copy the report from your own terminal window:

```sh
curl -fLO https://mysetup.is/scan-script/mysetup-scan.sh
sh mysetup-scan.sh
```

</details>

### Check a script download

Each release carries the two scripts, `SHA256SUMS`, and GitHub's attestation that these files were built from this repository at
that release's tag:

```sh
sha256sum -c SHA256SUMS --ignore-missing
gh attestation verify mysetup-scan.sh --repo eddinn/mysetup-scanner
```

MySetup's build page shows each file's SHA-256 too. The scripts are not code-signed yet. The report format is in
[docs/report-format.md](docs/report-format.md).

## Last: paste it on your build

1. Go to [mysetup.is/setups](https://mysetup.is/setups) in your web browser and sign in if it asks.
2. Click your build. No build yet? Click **Create setup** first.
3. Scroll down to **Hardware** and find **Scan this computer**.
4. Click in the box **Paste your report here** and paste: **Ctrl+V** (on a Mac **Cmd+V**).
5. Click **Review the report**. Check each part, then click **Add these parts**. Nothing changes in your build until you do.

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
`shellcheck` and PSScriptAnalyzer (`Install-Module PSScriptAnalyzer -Scope CurrentUser`). The same steps run for every change to the
scripts or tests, in two places. This repository lives on MySetup's own Git server, and GitHub is its mirror, which receives every
push. The lint, the tests, the run on Linux and the runs in containers of eight other distributions are on that server's own runner
([`.forgejo/workflows/scripts.yml`](.forgejo/workflows/scripts.yml)). Windows and macOS, and Windows and Linux on ARM processors,
which need GitHub's runners, run there ([`.github/workflows/scripts.yml`](.github/workflows/scripts.yml)).

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
