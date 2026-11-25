Version 0.3.0 of the MySetup scan app, for Windows and Linux: **one click sends your report, and the same browser tab takes you to
your build.** No code to fetch or type any more. It reads your computer's hardware (the computer's own model, processor, graphics,
motherboard, memory and storage, by name only) and shows the report on a page in your web browser, or in the terminal with
`--cli`. You check every part on [MySetup](https://mysetup.is) before anything is saved.

**What it sends, and when:** only when you click **Send to mysetup.is** on its page, or type **y** where the terminal asks; only to
mysetup.is over HTTPS; and only the report you see. MySetup answers with a one-time link that works once, for 30 minutes, and
shows the scan before you choose a build. A report nobody claims is deleted within 40 minutes. It writes no files and installs
nothing.

**New since 0.2.0**

- **No code:** the code box is gone. Click **Send to mysetup.is**, and the app's own tab goes on to MySetup: "Add your scan to a
  build", with the build whose **Scan this computer** you opened last already chosen. Signed out? Sign in, and you come back to
  the same scan.
- **In the terminal:** with `--cli`, type **y** to send; the app prints the link to open on this computer or another. Enter alone
  copies and pastes as before, so nobody sends by habit.
- **On Windows** the link is also copied, so a stray Enter that closes the window does not lose it.

## Last: send it to your build

1. Double-click the app (or run it in a terminal with `--cli`).
2. Check the report on its page, then click **Send to mysetup.is** (in the terminal, type **y**).
3. The tab goes on to MySetup. Sign in if it asks, check that the scan is yours, and choose the build.
4. Click **Review this scan**, check each part, then click **Add these parts**. Nothing changes in your build until you do.

Each file comes with `SHA256SUMS` and GitHub's attestation that it was built from this repository at this tag. The app is not
code-signed yet, so Windows SmartScreen warns about it: click **More info**, then **Run anyway**.
