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

// answering stands in for MySetup: it records each code and answers with the next of its answers.
func answering(answers ...error) (func(context.Context, string) (string, error), *[]string) {
	var codes []string
	return func(_ context.Context, code string) (string, error) {
		codes = append(codes, code)
		err := answers[len(codes)-1]
		if err != nil {
			return "", err
		}

		return "https://mysetup.is/setups/01M7/components/scan/01M8", nil
	}, &codes
}

func ask(typed string, sendReport func(context.Context, string) (string, error)) (bool, string) {
	var out bytes.Buffer
	sent := askAndSend(bufio.NewReader(strings.NewReader(typed)), &out, sendReport)

	return sent, out.String()
}

func TestATypedCodeSendsTheReportAndGivesTheReviewLink(t *testing.T) {
	sendReport, codes := answering(nil)
	sent, out := ask("  k7qd-m2xp \n", sendReport)
	if !sent || len(*codes) != 1 || (*codes)[0] != "k7qd-m2xp" {
		t.Fatalf("sent %v, codes %v", sent, *codes)
	}
	for _, want := range []string{
		"send it to your build in 3 steps", "go to " + buildsURL() + " and sign in", `click "Get a code"`,
		"sends the report above to " + siteName() + ", and nothing else", "press Enter without a code",
		"Sent. Open this address in your web browser to review it:\n  https://mysetup.is/setups/01M7/components/scan/01M8\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the terminal lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "copy and paste the report into your build") {
		t.Error("a sent report still gives the copy steps")
	}
}

func TestEnterWithoutACodeSendsNothing(t *testing.T) {
	for _, typed := range []string{"\n", "   \n", "", "\nK7QD-M2XP\n"} {
		sendReport, codes := answering()
		sent, out := ask(typed, sendReport)
		if sent || len(*codes) != 0 || !strings.Contains(out, "Nothing was sent.\n\nWhat to do now: copy and paste the report into your build in 5 steps.") {
			t.Errorf("%q: sent %v, codes %v:\n%s", typed, sent, *codes, out)
		}
	}
}

func TestAWrongCodeOrAnErrorIsSaidAndTheMemberMayTryAgain(t *testing.T) {
	sendReport, codes := answering(&send.Refused{Message: "This code is wrong, was already used or has expired."},
		errors.New("MySetup could not be reached (no route to host)"), nil)
	sent, out := ask("AAAA-AAAA\nBBBB-BBBB\nCCCC-CCCC\n", sendReport)
	if !sent || len(*codes) != 3 {
		t.Fatalf("sent %v, codes %v", sent, *codes)
	}
	for _, want := range []string{
		"This code is wrong, was already used or has expired.\nType the code again",
		"The report was not sent: MySetup could not be reached (no route to host).\nType the code again", "Sent.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the terminal lacks %q:\n%s", want, out)
		}
	}
}

func TestItStopsAskingAfterAFewTries(t *testing.T) {
	refused := &send.Refused{Message: "This code is wrong, was already used or has expired."}
	sendReport, codes := answering(refused, refused, refused, refused, refused)
	sent, out := ask(strings.Repeat("AAAA-AAAA\n", 9), sendReport)
	if sent || len(*codes) != tries || !strings.HasSuffix(out, "Click \"Review the report\", check each part, then click \"Add these parts\".\n") ||
		strings.Count(out, "Type the code again") != tries-1 {
		t.Fatalf("sent %v, codes %v:\n%s", sent, *codes, out)
	}
}
