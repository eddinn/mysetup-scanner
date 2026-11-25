package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/eddinn/mysetup-scanner/internal/send"
)

const link = "https://mysetup.is/scan/claim/AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_AbCdE"

// answering stands in for MySetup: it counts each send and answers with the next of its answers.
func answering(answers ...error) (func(context.Context) (send.Link, error), *int) {
	sends := 0
	return func(_ context.Context) (send.Link, error) {
		sends++
		if err := answers[sends-1]; err != nil {
			return send.Link{}, err
		}

		return send.Link{URL: link, Expires: time.Date(2026, 11, 24, 23, 41, 0, 0, time.Local)}, nil
	}, &sends
}

func ask(typed string, sendReport func(context.Context) (send.Link, error)) (string, string) {
	var out bytes.Buffer
	sent := askAndSend(bufio.NewReader(strings.NewReader(typed)), &out, sendReport)

	return sent, out.String()
}

func TestOnlyYSendsTheReportAndPrintsTheOneTimeLink(t *testing.T) {
	// y and then the input ending still sends: the member typed y.
	for _, typed := range []string{"y\n", " YES \n", "y"} {
		sendReport, sends := answering(nil)
		sent, out := ask(typed, sendReport)
		if sent != link || *sends != 1 {
			t.Fatalf("%q: sent %q, sends %d", typed, sent, *sends)
		}
		for _, want := range []string{
			"send it to your build in 4 steps", "Type y and press Enter here. The app sends the report above to " + siteName() + ", and nothing else",
			"To copy and paste the report instead, press Enter alone.", "paste it into the browser's\n     address bar",
			"Sent. Open this link in your web browser, on this computer or another, to add the report to your build:\n  " + link + "\n",
			"It works once, until 23:41.", `click "Review this scan"`,
		} {
			if !strings.Contains(out, want) {
				t.Errorf("the terminal lacks %q:\n%s", want, out)
			}
		}
		if strings.Contains(out, "copy and paste the report into your build") || strings.Contains(out, "code") {
			t.Errorf("%q: a sent report gives the copy steps or speaks of a code:\n%s", typed, out)
		}
	}
}

func TestEnterAloneNoOrTheInputEndingSendsNothing(t *testing.T) {
	// Enter alone kept 0.2.0's meaning, copy and paste, so nobody sends by habit (Codex's review, 2026-11-24).
	for _, typed := range []string{"\n", "   \n", "n\n", " No \n", "", "maybe", "maybe\n", "\ny\n"} {
		sendReport, sends := answering()
		sent, out := ask(typed, sendReport)
		if sent != "" || *sends != 0 || !strings.Contains(out, "Nothing was sent.\n\nWhat to do now: copy and paste the report into your build in 5 steps.") {
			t.Errorf("%q: sent %q, sends %d:\n%s", typed, sent, *sends, out)
		}
	}
}

func TestAnErrorIsSaidAndTheMemberMayTryAgain(t *testing.T) {
	sendReport, sends := answering(&send.Refused{Message: "Sending is busy right now."},
		errors.New("MySetup could not be reached (no route to host)"), nil)
	sent, out := ask("y\nmaybe\ny\ny\n", sendReport)
	if sent != link || *sends != 3 {
		t.Fatalf("sent %q, sends %d:\n%s", sent, *sends, out)
	}
	for _, want := range []string{
		"Sending is busy right now.\nSend it to", "Type y and press Enter to send it, or press Enter alone to copy and paste it.\n",
		"The report was not sent: MySetup could not be reached (no route to host).\nSend it to", "Sent.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the terminal lacks %q:\n%s", want, out)
		}
	}
}

func TestItStopsAskingAfterAFewTries(t *testing.T) {
	refused := &send.Refused{Message: "Sending is busy right now."}
	sendReport, sends := answering(refused, refused, refused)
	sent, out := ask(strings.Repeat("y\n", 9), sendReport)
	if sent != "" || *sends != tries || !strings.HasSuffix(out, "Click \"Review the report\", check each part, then click \"Add these parts\".\n") {
		t.Fatalf("sent %q, sends %d:\n%s", sent, *sends, out)
	}
}
