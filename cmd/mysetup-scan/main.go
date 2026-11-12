// mysetup-scan reads this computer's hardware into a report for the member's MySetup build, the same report the scripts print
// (docs/report-format.md). It sends the report to MySetup only when the member types a code from their build page, and only
// there (internal/send); nothing else is sent, no files are written, nothing is installed.
//
// By default it shows the report on a page in the member's own browser, served from this computer only, with a box for the code,
// buttons to copy or save the report instead, and numbered steps for what to do next (internal/page). That holds run with sudo or as administrator too: the app reads
// first, then gives up root for the person who ran it (Linux, the BSDs, macOS), or starts itself again without administrator
// rights (Windows), so neither the page nor the browser runs elevated. Only where there is no desktop at all (over SSH, for
// example) does it print the report in the terminal instead, and say why. With --cli it prints the report in the terminal, always.
// In the terminal it asks for the code after the report, when a member can see the question and answer it.
// Run with root on Linux and the BSDs, it can also read what the system keeps from ordinary users, such as memory modules on a
// system without udev.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/eddinn/mysetup-scanner/internal/page"
	"github.com/eddinn/mysetup-scanner/internal/scan"
	"github.com/eddinn/mysetup-scanner/internal/send"
)

const (
	// pageLife is how long the page is served at most; pageGone is how long after its tab stopped checking in the app ends.
	pageLife = 15 * time.Minute
	pageGone = 90 * time.Second
	// tries is how many codes the terminal takes before it gives the steps to copy and paste instead.
	tries = 5
)

// input is what the member types in the terminal, read through one buffer: the code, and the Enter that closes a window of its
// own on Windows.
var input = bufio.NewReader(os.Stdin)

// buildsURL is where a member finds their builds on the MySetup the app sends to.
func buildsURL() string { return send.Site() + "/setups" }

// siteName is that MySetup's name as the member reads it, such as mysetup.is.
func siteName() string {
	if parsed, err := url.Parse(send.Site()); err == nil && parsed.Host != "" {
		return parsed.Host
	}

	return send.Site()
}

// about is the line the app starts with: its version, and what it sends where. A development build says so.
func about() string {
	line := fmt.Sprintf("MySetup scan %s. It reads hardware names, and sends the report to %s only when you type a code from your build page.",
		scan.FullVersion(), siteName())
	if send.Development {
		line += " This is a development build, for tests only."
	}

	return line
}

type options struct {
	cli       bool
	noBrowser bool
	// detached is set only by the app itself, when it started again without a console (detach) or without administrator rights
	// (unelevate): it has no terminal to write to, so it tells the member of a problem in a message box.
	detached bool
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	var opts options
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			usage(stdout)

			return 0
		case "--version":
			fmt.Fprintln(stdout, "mysetup-scan "+scan.FullVersion())

			return 0
		case "--cli":
			opts.cli = true
		case "--no-browser":
			opts.noBrowser = true
		case "--detached":
			opts.detached = true
		default:
			usage(stderr)

			return 2
		}
	}
	if opts.cli && opts.noBrowser {
		fmt.Fprintln(stderr, "--cli prints the report in the terminal, so it takes no --no-browser.")

		return 2
	}
	graphical := !opts.cli
	// Double-clicked on Linux or another Unix-like system, the app has no terminal to write to: it says what goes wrong in the
	// desktop's own message box, as it does on Windows.
	if graphical && !opts.detached && startedWithoutTerminal() {
		opts.detached = true
	}
	if opts.detached {
		leaveOwnConsole()
	}
	// On Windows, run as administrator, the app starts again without those rights; double-clicked, it starts again without the
	// console window Windows opened for it. Either way only the browser page is left.
	if graphical && !opts.detached && !opts.noBrowser && (unelevate() || detach()) {
		return 0
	}

	h := scan.Read()
	if h.Environment == "wsl" {
		const wsl = "This is WSL, which sees virtual hardware. Please run the Windows app (mysetup-scan.exe) in Windows instead. Nothing was reported."
		fmt.Fprintln(stderr, wsl)
		if opts.detached {
			tell(wsl)
		}

		return 4
	}
	text := h.Report().Text()
	if !graphical {
		return printReport(h, text, "", stdout, stderr)
	}
	session, why := page.FindSession()
	if session == nil && !opts.noBrowser {
		if opts.detached {
			tell("MySetup scan found no desktop to open its page in (" + why + "). Open a terminal in the folder with the app " +
				"and run it there with --cli: ./mysetup-scan --cli")
		}

		return printReport(h, text, why, stdout, stderr)
	}

	return showPage(h, text, session, opts, stdout, stderr)
}

// printReport prints the report in the terminal: the report itself to stdout, everything said around it to stderr. Then, where a
// member can answer, it asks for the code to send it with.
func printReport(h scan.Hardware, text, why string, stdout, stderr io.Writer) int {
	sent := false
	// Double-clicked on Windows, the window would close the moment the app ends: it stays open, with the report copied unless it
	// was sent.
	defer func() { keepWindowOpen(text, sent, stderr) }()
	fmt.Fprintln(stderr, about())
	if why != "" {
		fmt.Fprintln(stderr, why)
	}
	fmt.Fprintln(stderr)
	fmt.Fprint(stdout, text)
	fmt.Fprintln(stderr)
	for _, note := range h.Notes {
		fmt.Fprintln(stderr, note)
	}
	if len(h.Notes) > 0 {
		fmt.Fprintln(stderr)
	}
	if !canAsk() {
		fmt.Fprint(stderr, copySteps())
		fmt.Fprintln(stderr, "To send it straight to your build with a code instead, run the app in a terminal, where it can ask for the code.")

		return 0
	}
	sent = askAndSend(input, stderr, sender(text))

	return 0
}

// sender sends this report, and only this one, with a code the member typed.
func sender(text string) func(context.Context, string) (string, error) {
	return func(ctx context.Context, code string) (string, error) {
		return send.Report(ctx, code, text, scan.FullVersion())
	}
}

// askAndSend gives the steps to send the report and asks for the code: typing it is the member's approval, and Enter without one
// sends nothing and gives the steps to copy and paste instead (MySetup's docs/scanner/design.md, Pairing). It says what came of
// each try, and after a few it stops asking. It says whether the report was sent.
func askAndSend(in *bufio.Reader, out io.Writer, sendReport func(context.Context, string) (string, error)) bool {
	fmt.Fprint(out, sendSteps())
	for try := 1; try <= tries; try++ {
		fmt.Fprint(out, "Code: ")
		line, err := in.ReadString('\n')
		code := strings.TrimSpace(line)
		if code == "" {
			if err != nil {
				// The input ended (Ctrl+D) with no line of its own.
				fmt.Fprintln(out)
			}

			break
		}
		fmt.Fprintln(out, "Sending the report...")
		link, err := sendReport(context.Background(), code)
		if err == nil {
			fmt.Fprintf(out, "\nSent. Open this address in your web browser to review it:\n  %s\n", link)
			fmt.Fprintln(out, `Check each part, then click "Add these parts". Nothing changes in your build until you do.`)

			return true
		}
		var refused *send.Refused
		if errors.As(err, &refused) {
			fmt.Fprintln(out, refused.Message)
		} else {
			fmt.Fprintln(out, "The report was not sent: "+err.Error()+".")
		}
		if try < tries {
			fmt.Fprintln(out, "Type the code again, or a new one, or press Enter without a code to copy and paste the report instead.")
		}
	}
	fmt.Fprint(out, "\nNothing was sent.\n\n"+copySteps())

	return false
}

// sendSteps say, for someone who has never done it, how the report gets from the terminal to their build with a code.
func sendSteps() string {
	return `What to do now: send it to your build in 3 steps.
  1. In your web browser, go to ` + buildsURL() + ` and sign in if it asks.
     Click your build. No build yet? Click "Create setup" first.
  2. Scroll down to "Hardware" and find "Scan this computer". Open "With the MySetup app" and click "Get a code".
     The code works once, for 10 minutes.
  3. Type the code here and press Enter. The app sends the report above to ` + siteName() + `, and nothing else.
To copy and paste the report instead, press Enter without a code.

`
}

// copySteps say, for someone who has never done it, how the report gets from the terminal to their build by copy and paste.
func copySteps() string {
	return `What to do now: copy and paste the report into your build in 5 steps.
  1. Copy the report above: everything from the line "mysetup-report 1" down to the line "mysetup-report-end".
     Select it with the mouse, then copy it: in most Linux terminals press Ctrl+Shift+C, on a Mac press Cmd+C,
     in Windows Terminal or PowerShell press Ctrl+C.
  2. In your web browser, go to ` + buildsURL() + ` and sign in if it asks.
  3. Click your build. No build yet? Click "Create setup" first.
  4. Scroll down to "Hardware" and find "Scan this computer". Click in the box "Paste your report here"
     and paste: press Ctrl+V (Cmd+V on a Mac).
  5. Click "Review the report", check each part, then click "Add these parts".
`
}

// showPage serves the report's page on this computer only and opens it in the member's browser, until the member is done. Run as
// root, it first gives up root for the person who ran it (session.Leave), before anything is served or opened.
func showPage(h scan.Hardware, text string, session *page.Session, opts options, stdout, stderr io.Writer) int {
	say := func(message string, important bool) {
		fmt.Fprintln(stderr, message)
		if important && opts.detached {
			tell(message)
		}
	}
	if session != nil {
		if err := session.Leave(); err != nil {
			say("The app could not give up root ("+err.Error()+"), so it does not open a page; here is the report in the terminal instead.", true)

			return printReport(h, text, "", stdout, stderr)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		say("The app could not start its page on this computer ("+err.Error()+"). Run it again with --cli to get the report in the terminal instead.", true)

		return 1
	}
	content := page.Content{Version: scan.FullVersion(), Report: text, Notes: h.Notes, BuildsURL: buildsURL(), Site: siteName(), Send: sender(text)}
	server, err := page.New(content, listener, pageLife, pageGone)
	if err != nil {
		_ = listener.Close()
		say("The app could not start its page on this computer ("+err.Error()+"). Run it again with --cli to get the report in the terminal instead.", true)

		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	served := make(chan error, 1)
	go func() { served <- server.Serve(ctx, listener) }()

	say(about(), false)
	switch {
	case opts.noBrowser || session == nil:
		say("Open this address in your web browser on this computer: "+server.URL(), false)
	case session.Open(server.URL()) != nil:
		say("Your web browser did not open by itself. Open this address in it, on this computer: "+server.URL(), true)
	default:
		say("Your report is now open in your web browser. If you do not see it, open this address in your browser: "+server.URL(), false)
	}
	say("Follow the steps on that page. The app stops when you click Done there, close that tab, or press Ctrl+C here, and after 15 minutes at most.", false)

	if err := <-served; err != nil {
		say("The page stopped with an error ("+err.Error()+"). Run the app again with --cli to get the report in the terminal instead.", true)

		return 1
	}

	return 0
}

func usage(w io.Writer) {
	fmt.Fprintf(w, "MySetup scan %s: reads this computer's hardware into a report for your build on %s.\n\n", scan.FullVersion(), siteName())
	fmt.Fprintln(w, "Usage: mysetup-scan [--cli | --no-browser] [--help] [--version]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  (no option)   show the report on a page in your web browser, with the steps to send it to your build")
	fmt.Fprintln(w, "                with a code from your build page, or to copy or save it instead")
	fmt.Fprintln(w, "  --cli         print the report in this terminal instead, with the same steps, and ask for the code here")
	fmt.Fprintln(w, "  --no-browser  serve the page without opening a browser, and print its address to open yourself")
	fmt.Fprintln(w, "  --help        show this help")
	fmt.Fprintln(w, "  --version     show the version")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "It sends the report to %s only when you type a code from your build page there, and nothing else\n", siteName())
	fmt.Fprintln(w, "anywhere; it writes no files and installs nothing. The page is served on this computer only (127.0.0.1)")
	fmt.Fprintln(w, "and stops when you click Done, close its tab, or after 15 minutes.")
	fmt.Fprintln(w, "Run with sudo or as administrator, it still opens your browser as you, never as root or administrator.")
	fmt.Fprintln(w, "Where there is no desktop at all (over SSH, for example), it prints the report in the terminal instead.")
	fmt.Fprintln(w, "On Linux and the BSDs, sudo lets it also read what the system keeps from ordinary users, such as memory")
	fmt.Fprintln(w, "modules on a system without udev.")
}
