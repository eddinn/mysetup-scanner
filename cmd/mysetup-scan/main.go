// mysetup-scan reads this computer's hardware into a report for the member to paste on their MySetup build page, the same report
// the scripts print (docs/report-format.md). It only reads: nothing is sent anywhere, no files are written, nothing is installed.
//
// By default it shows the report on a page in the member's own browser, served from this computer only, with buttons to copy or
// save it (internal/page). With --cli it prints the report in the terminal instead, as it also does where no browser can be
// opened: over SSH, with no desktop, or run as root or administrator. It needs no administrator or root; run with them on Linux
// and the BSDs, it can also read what the system keeps from ordinary users, such as memory modules on a system without udev.
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eddinn/mysetup-scanner/internal/page"
	"github.com/eddinn/mysetup-scanner/internal/scan"
)

const (
	// pageLife is how long the page is served at most; pageGone is how long after its tab stopped checking in the app ends.
	pageLife = 15 * time.Minute
	pageGone = 90 * time.Second
)

type options struct {
	cli       bool
	noBrowser bool
	// detached is set only by the app itself, when Windows opened a console window for it alone and it started again without
	// one (detach): it has no terminal to write to, so it tells the member of a problem in a message box.
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

	graphical, why := !opts.cli, ""
	if graphical && !opts.noBrowser {
		graphical, why = page.Desktop()
	}
	// Double-clicked on Windows, the app would keep a console window open behind the browser: it starts again without one.
	if graphical && !opts.detached && !opts.noBrowser && detach() {
		return 0
	}

	h := scan.Read()
	if h.Environment == "wsl" {
		message := "This is WSL, which sees virtual hardware. Please run the Windows app (mysetup-scan.exe) in Windows instead. Nothing was reported."
		fmt.Fprintln(stderr, message)

		return 4
	}
	text := h.Report().Text()
	if graphical {
		return showPage(h, text, opts, stderr)
	}

	return printReport(h, text, why, stdout, stderr)
}

// printReport prints the report in the terminal: the report itself to stdout, everything said around it to stderr.
func printReport(h scan.Hardware, text, why string, stdout, stderr io.Writer) int {
	// Double-clicked on Windows, the window would close the moment the app ends: it stays open, with the report copied.
	defer keepWindowOpen(text, stderr)
	fmt.Fprintf(stderr, "MySetup scan %s. It reads hardware names and sends nothing anywhere.\n", scan.FullVersion())
	if why != "" {
		fmt.Fprintln(stderr, why)
	}
	fmt.Fprintln(stderr, `Copy everything from "mysetup-report 1" to "mysetup-report-end" and paste it on your build page.`)
	fmt.Fprintln(stderr)
	fmt.Fprint(stdout, text)
	if len(h.Notes) > 0 {
		fmt.Fprintln(stderr)
		for _, note := range h.Notes {
			fmt.Fprintln(stderr, note)
		}
	}

	return 0
}

// showPage serves the report's page on this computer only and opens it in the member's browser, until the member is done.
func showPage(h scan.Hardware, text string, opts options, stderr io.Writer) int {
	say := func(message string, important bool) {
		fmt.Fprintln(stderr, message)
		if important && opts.detached {
			tell(message)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		say("The app could not start its page on this computer ("+err.Error()+"). Run it with --cli to print the report instead.", true)

		return 1
	}
	server, err := page.New(page.Content{Version: scan.FullVersion(), Report: text, Notes: h.Notes}, listener, pageLife, pageGone)
	if err != nil {
		_ = listener.Close()
		say("The app could not start its page on this computer ("+err.Error()+"). Run it with --cli to print the report instead.", true)

		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	served := make(chan error, 1)
	go func() { served <- server.Serve(ctx, listener) }()

	say(fmt.Sprintf("MySetup scan %s. It reads hardware names and sends nothing anywhere.", scan.FullVersion()), false)
	switch {
	case opts.noBrowser:
		say("Open this address in your browser on this computer: "+server.URL(), false)
	case page.Open(server.URL()) != nil:
		say("Your browser did not open. Open this address in it on this computer: "+server.URL(), true)
	default:
		say("Your report is open in your browser. If it is not, open this address on this computer: "+server.URL(), false)
	}
	say("The app stops when you click Done on the page, close its tab, or press Ctrl+C, and after 15 minutes at most.", false)

	if err := <-served; err != nil {
		say("The page stopped with an error ("+err.Error()+"). Run the app with --cli to print the report instead.", true)

		return 1
	}

	return 0
}

func usage(w io.Writer) {
	fmt.Fprintf(w, "MySetup scan %s: reads this computer's hardware into a report to paste on your build page.\n\n", scan.FullVersion())
	fmt.Fprintln(w, "Usage: mysetup-scan [--cli | --no-browser] [--help] [--version]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  (no option)   show the report on a page in your browser, with buttons to copy or save it")
	fmt.Fprintln(w, "  --cli         print the report in this terminal instead")
	fmt.Fprintln(w, "  --no-browser  serve the page without opening a browser, and print its address to open yourself")
	fmt.Fprintln(w, "  --help        show this help")
	fmt.Fprintln(w, "  --version     show the version")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "It only reads: nothing is sent anywhere, no files are written, nothing is installed. The page is served")
	fmt.Fprintln(w, "on this computer only (127.0.0.1) and stops when you click Done, close its tab, or after 15 minutes.")
	fmt.Fprintln(w, "Where no browser can be opened (over SSH, with no desktop, or run as root or administrator), the app")
	fmt.Fprintln(w, "prints the report in the terminal. It needs no administrator or root; run with sudo on Linux and the BSDs,")
	fmt.Fprintln(w, "it can also read what the system keeps from ordinary users.")
}
