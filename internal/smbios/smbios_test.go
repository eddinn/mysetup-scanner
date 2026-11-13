package smbios_test

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/eddinn/mysetup-scanner/internal/smbios"
	"github.com/eddinn/mysetup-scanner/internal/smbios/smbiostest"
)

func TestOnlyWhatTheReportNeedsIsRead(t *testing.T) {
	got := Parse(smbiostest.MadeUpTable())
	text := fmt.Sprintf("%+v", got)
	for _, planted := range []string{"FAKE-SERIAL", "FAKE-TAG", "FAKE-LOCATOR", "FAKE-UUID", "Rev 1.xx"} {
		if strings.Contains(text, planted) {
			t.Fatalf("%s was read: %s", planted, text)
		}
	}
	if got.BoardMaker != "ASUSTeK COMPUTER INC." || got.BoardModel != "ROG STRIX X670E-E" {
		t.Fatalf("board: %+v", got)
	}
	// The computer's own model, as a laptop names it, and its chassis type; its serial and UUID and the chassis's tag never.
	if got.SystemMaker != "Micro-Star International Co., Ltd." || got.SystemModel != "GL65 Leopard 9SCSR" || got.SystemVersion != "REV:1.0" ||
		got.SystemSKU != "16R3.1" || got.SystemFamily != "GL" || got.Chassis != "notebook" {
		t.Fatalf("system: %+v", got)
	}
	if len(got.Processors) != 1 || got.Processors[0] != (Processor{Cores: 8, Threads: 16}) {
		t.Fatalf("processors: %+v", got.Processors)
	}
	want := MemoryDevice{SizeMB: 32768, Maker: "Corsair", Part: "CMK64GX5M2B6000C30", Speed: 6000, Running: 6000, Type: "DDR5", Form: "DIMM"}
	if len(got.Memory) != 1 || got.Memory[0] != want {
		t.Fatalf("memory: %+v", got.Memory)
	}
}

func TestChassisTypesHaveTheirNames(t *testing.T) {
	for kind, want := range map[byte]string{0x03: "desktop", 0x09: "laptop", 0x0A: "notebook", 0x8A: "notebook", 0x17: "rack-mount-chassis",
		0x1F: "convertible", 0x23: "mini-pc", 0x01: "", 0x02: "", 0x7F: ""} {
		if got := ChassisName(kind); got != want {
			t.Errorf("%#x: %q, want %q", kind, got, want)
		}
	}
}

func TestATruncatedTableKeepsWhatCameBefore(t *testing.T) {
	table := smbiostest.MadeUpTable()
	got := Parse(table[:len(table)-60])
	if got.BoardModel != "ROG STRIX X670E-E" {
		t.Fatalf("board lost: %+v", got)
	}
	if len(Parse(nil).Memory) != 0 || len(Parse([]byte{17, 2, 0}).Memory) != 0 {
		t.Fatal("nothing should come of a broken table")
	}
}
