// mysetup-scan prints a report of this computer's hardware for the member to paste on their MySetup build page, the same report
// the scripts print (docs/report-format.md). It only reads: no network, no files written, nothing installed. It runs without
// administrator or root; run with them, it can also read what the system keeps from ordinary users, such as memory modules on
// a Linux system without udev. Stage 1 of the app (docs/app.md): the report in the terminal; the browser page and sending come
// later.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/eddinn/mysetup-scanner/internal/scan"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	for _, arg := range args {
		switch arg {
		case "--help", "-h":
			usage(stdout)

			return 0
		case "--version":
			fmt.Fprintln(stdout, "mysetup-scan "+scan.Version)

			return 0
		default:
			usage(stderr)

			return 2
		}
	}

	h := scan.Read()
	if h.Environment == "wsl" {
		fmt.Fprintln(stderr, "This is WSL, which sees virtual hardware. Please run the Windows app (mysetup-scan.exe) in Windows instead. Nothing was reported.")

		return 4
	}
	fmt.Fprintf(stderr, "MySetup scan %s. It reads hardware names and sends nothing anywhere.\n", scan.Version)
	fmt.Fprintln(stderr, `Copy everything from "mysetup-report 1" to "mysetup-report-end" and paste it on your build page.`)
	fmt.Fprintln(stderr)
	text := h.Report().Text()
	fmt.Fprint(stdout, text)
	if len(h.Notes) > 0 {
		fmt.Fprintln(stderr)
		for _, note := range h.Notes {
			fmt.Fprintln(stderr, note)
		}
	}

	return 0
}

func usage(w io.Writer) {
	fmt.Fprintf(w, "MySetup scan %s: prints a report of this computer's hardware to paste on your build page.\n\n", scan.Version)
	fmt.Fprintln(w, "Usage: mysetup-scan [--help] [--version]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "It only reads: no network, no files written, nothing installed. It needs no administrator or root;")
	fmt.Fprintln(w, "run with sudo (or as administrator), it can also read what the system keeps from ordinary users.")
}
