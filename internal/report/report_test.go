package report

import (
	"strings"
	"testing"
)

func TestTextIsCleanedAndPlaceholdersAndAddressesDropped(t *testing.T) {
	cases := map[string]string{
		"  AMD  Ryzen 7  7800X3D ":             "AMD Ryzen 7 7800X3D",
		"AMD\tRyzen":                           "AMDRyzen", // As the scripts: a tab is not printable ASCII, and goes.
		"To Be Filled By O.E.M.":               "",
		"Default string":                       "",
		"0000000":                              "",
		"PartNumber3":                          "",
		"de:ad:be:ef:00:01":                    "",
		"12345678-1234-1234-1234-123456789abc": "",
		"10.20.30.40":                          "",
		"me@example.com":                       "",
		"/home/someone/x":                      "",
		`C:\Users\x`:                           "",
		"Kingston\u00e9 FURY":                  "Kingston FURY",
		strings.Repeat("x", 200):               "",
		strings.Repeat("ab", 100):              strings.Repeat("ab", 60),
	}
	for in, want := range cases {
		if got := CleanText(in); got != want {
			t.Errorf("CleanText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestOnlyNamedFieldsAndValidValuesAreWritten(t *testing.T) {
	r := &Report{}
	r.Set("cpu.model", "AMD Ryzen 7 7800X3D")
	r.Set("cpu.cores", "0")
	r.Set("cpu.threads", "16x")
	r.Set("gpu.0.pci", "10DE:2204")
	r.Set("gpu.1.pci", "10de:2204")
	r.Set("memory.0.type", "Unknown")
	r.Set("os.name", "debian sid")
	r.SetNumber("memory.total_mb", 31890)
	r.Capability("displays", "not-collected")
	want := "mysetup-report 1\ncpu.model=AMD Ryzen 7 7800X3D\ngpu.1.pci=10de:2204\nmemory.total_mb=31890\n" +
		"capability.displays=unavailable\ncapability.displays.reason=not-collected\nmysetup-report-end\n"
	if got := r.Text(); got != want {
		t.Fatalf("report:\n%s\nwant:\n%s", got, want)
	}

	defer func() {
		if recover() == nil {
			t.Fatal("a field the format does not name was written")
		}
	}()
	r.Set("system.serial", "FAKE-SERIAL")
}
