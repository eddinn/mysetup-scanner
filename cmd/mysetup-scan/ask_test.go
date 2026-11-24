package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/eddinn/mysetup-scanner/internal/send"
)

const link = "https://mysetup.is/scan/claim/AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_AbCdE"

// answering stands in for MySetup: it counts each send and answers with the next of its answers.
func answering(answers ...error) (func(context.Context) (string, error), *int) {
	sends := 0
	return func(_ context.Context) (string, error) {
		sends++
		if err := answers[sends-1]; err != nil {
			return "", err
		}

		return link, nil
	}, &sends
}

func ask(typed string, sendReport func(context.Context) (string, error)) (bool, string) {
	var out bytes.Buffer
	sent := askAndSend(bufio.NewReader(strings.NewReader(typed)), &out, sendReport)

	return sent, out.String()
}

func TestEnterSendsTheReportAndPrintsTheOneTimeLink(t *testing.T) {
	for _, typed := range []string{"\n", "y\n", " YES \n"} {
		sendReport, sends := answering(nil)
		sent, out := ask(typed, sendReport)
		if !sent || *sends != 1 {
			t.Fatalf("%q: sent %v, sends %d", typed, sent, *sends)
		}
		for _, want := range []string{
			"send it to your build in 4 steps", "sends the report above to " + siteName() + ", and nothing else", "type n and press Enter",
			"Sent. Open this link in your web browser, on this computer or another, to add the report to your build:\n  " + link + "\n",
			"It works once, for 30 minutes.", `click "Review this scan"`,
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

func TestNoOrTheInputEndingSendsNothing(t *testing.T) {
	for _, typed := range []string{"n\n", " No \n", "", "n\n\n"} {
		sendReport, sends := answering()
		sent, out := ask(typed, sendReport)
		if sent || *sends != 0 || !strings.Contains(out, "Nothing was sent.\n\nWhat to do now: copy and paste the report into your build in 5 steps.") {
			t.Errorf("%q: sent %v, sends %d:\n%s", typed, sent, *sends, out)
		}
	}
}

func TestAnErrorIsSaidAndTheMemberMayTryAgain(t *testing.T) {
	sendReport, sends := answering(&send.Refused{Message: "Sending is busy right now."},
		errors.New("MySetup could not be reached (no route to host)"), nil)
	sent, out := ask("\nmaybe\n\n\n", sendReport)
	if !sent || *sends != 3 {
		t.Fatalf("sent %v, sends %d:\n%s", sent, *sends, out)
	}
	for _, want := range []string{
		"Sending is busy right now.\nSend it now?", "Press Enter to send it, or type n and press Enter.\n",
		"The report was not sent: MySetup could not be reached (no route to host).\nSend it now?", "Sent.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the terminal lacks %q:\n%s", want, out)
		}
	}
}

func TestItStopsAskingAfterAFewTries(t *testing.T) {
	refused := &send.Refused{Message: "Sending is busy right now."}
	sendReport, sends := answering(refused, refused, refused)
	sent, out := ask(strings.Repeat("\n", 9), sendReport)
	if sent || *sends != tries || !strings.HasSuffix(out, "Click \"Review the report\", check each part, then click \"Add these parts\".\n") {
		t.Fatalf("sent %v, sends %d:\n%s", sent, *sends, out)
	}
}
