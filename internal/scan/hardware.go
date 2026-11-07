// Package scan reads this computer's hardware and writes it as a MySetup report (docs/report-format.md), the same report the
// scripts print. Each system has a reader that works on what the system hands it (files, firmware tables, the output of the
// system's own tools), so the tests can give it a made-up computer; the live sources are in the live_*.go files.
package scan

import (
	"fmt"
	"strings"

	"github.com/eddinn/mysetup-scanner/internal/report"
)

// Version is the app's version, as the report's source names it.
const Version = "0.1.0"

// Source is what the report's source field says: the app and its version.
const Source = "app/" + Version

// Hardware is everything a reader found. Empty means unknown.
type Hardware struct {
	Family, OSName, OSVersion, Architecture, Environment string
	// Tested is true once a real machine of this system and architecture has passed with the app.
	Tested bool

	CPU    CPU
	GPUs   []GPU
	Board  Board
	Memory Memory
	Disks  []Disk

	// Why a part could not be read, when it could not: needs-root, not-exposed or no-source (the report's reasons).
	ModulesReason, ConfiguredReason string

	// Notes for the member, printed beside the report and never in it.
	Notes []string
}

type CPU struct {
	Vendor, Model  string
	Cores, Threads int
}

type GPU struct {
	PCI, Subsystem, Vendor, Model string
}

type Board struct {
	Vendor, Model string
}

type Memory struct {
	TotalMB int64
	Modules []Module
}

type Module struct {
	SizeMB         int64
	Maker, Part    string
	Speed, Running int
	Type, Form     string
}

type Disk struct {
	Model            string
	Bytes            int64
	Interface, Media string
}

// Note adds a note for the member, once.
func (h *Hardware) Note(format string, args ...any) {
	note := fmt.Sprintf(format, args...)
	for _, existing := range h.Notes {
		if existing == note {
			return
		}
	}
	h.Notes = append(h.Notes, note)
}

// Report writes the hardware in the report's order. Parts beyond report.MaxItems are left out, with a note.
func (h *Hardware) Report() *report.Report {
	r := &report.Report{}
	r.Set("source", Source)
	r.Set("tested", map[bool]string{true: "yes", false: "no"}[h.Tested])
	r.Set("os.family", h.Family)
	r.Set("os.name", h.OSName)
	r.Set("os.version", h.OSVersion)
	r.Set("os.architecture", h.Architecture)
	r.Set("environment", h.Environment)

	cpu := "no-source"
	r.Set("cpu.vendor", NormalCPUVendor(h.CPU.Vendor))
	r.Set("cpu.model", h.CPU.Model)
	r.SetNumber("cpu.cores", int64(h.CPU.Cores))
	r.SetNumber("cpu.threads", int64(h.CPU.Threads))
	if h.CPU.Model != "" || h.CPU.Threads > 0 {
		cpu = "available"
	}

	gpu := "no-source"
	for n := range h.limit(len(h.GPUs), "graphics adapters") {
		item := h.GPUs[n]
		r.Set(fmt.Sprintf("gpu.%d.pci", n), item.PCI)
		r.Set(fmt.Sprintf("gpu.%d.subsystem", n), item.Subsystem)
		r.Set(fmt.Sprintf("gpu.%d.vendor", n), item.Vendor)
		r.Set(fmt.Sprintf("gpu.%d.model", n), item.Model)
		gpu = "available"
	}

	board := "no-source"
	vendor, model := report.CleanText(h.Board.Vendor), report.CleanText(h.Board.Model)
	r.Set("motherboard.vendor", vendor)
	r.Set("motherboard.model", model)
	if vendor != "" || model != "" {
		board = "available"
	}

	r.SetNumber("memory.total_mb", h.Memory.TotalMB)
	modules, configured := orDefault(h.ModulesReason, "no-source"), orDefault(h.ConfiguredReason, "no-source")
	for n := range h.limit(len(h.Memory.Modules), "memory modules") {
		m := h.Memory.Modules[n]
		r.SetNumber(fmt.Sprintf("memory.%d.size_mb", n), m.SizeMB)
		r.Set(fmt.Sprintf("memory.%d.maker", n), m.Maker)
		r.Set(fmt.Sprintf("memory.%d.part", n), m.Part)
		r.SetNumber(fmt.Sprintf("memory.%d.speed_mts", n), int64(m.Speed))
		r.SetNumber(fmt.Sprintf("memory.%d.configured_speed_mts", n), int64(m.Running))
		r.Set(fmt.Sprintf("memory.%d.type", n), m.Type)
		r.Set(fmt.Sprintf("memory.%d.form", n), m.Form)
		modules = "available"
		if m.Running > 0 {
			configured = "available"
		}
	}

	storage := "no-source"
	for n := range h.limit(len(h.Disks), "disks") {
		d := h.Disks[n]
		r.Set(fmt.Sprintf("storage.%d.model", n), d.Model)
		r.SetNumber(fmt.Sprintf("storage.%d.size_bytes", n), d.Bytes)
		r.Set(fmt.Sprintf("storage.%d.interface", n), d.Interface)
		r.Set(fmt.Sprintf("storage.%d.media", n), d.Media)
		storage = "available"
	}

	r.Capability("cpu", cpu)
	r.Capability("gpu", gpu)
	r.Capability("motherboard", board)
	r.Capability("memory_modules", modules)
	r.Capability("configured_memory", configured)
	r.Capability("storage", storage)
	r.Capability("displays", "not-collected")

	return r
}

// limit is the indexes of a part's devices to list, at most report.MaxItems, with a note when some are left out.
func (h *Hardware) limit(count int, what string) []struct{} {
	if count > report.MaxItems {
		h.Note("Only the first %d %s are listed.", report.MaxItems, what)
		count = report.MaxItems
	}

	return make([]struct{}, count)
}

// FinalNotes adds the notes every report of this kind carries: an untested system, and a computer that is not the real one.
func (h *Hardware) FinalNotes() {
	if !h.Tested {
		h.Note("Untested on this system (%s/%s). Parts may be missing or wrong, so please check the report before you paste it.",
			h.Family, h.Architecture)
	}
	switch h.Environment {
	case "virtual_machine":
		h.Note("This looks like a virtual machine, so the report shows what it was given, not the real computer. " +
			"To include your real hardware, run the app on the host instead.")
	case "container":
		h.Note("This looks like a container, so the report shows what it was given, not the real computer. " +
			"To include your real hardware, run the app on the host instead.")
	}
}

// NormalArchitecture names a machine type as the report does.
func NormalArchitecture(machine string) string {
	switch m := strings.ToLower(machine); {
	case m == "x86_64" || m == "amd64" || m == "i86pc":
		return "x86-64"
	case m == "aarch64" || m == "arm64":
		return "arm64"
	case len(m) == 4 && m[0] == 'i' && strings.HasSuffix(m, "86"), m == "386", m == "x86":
		return "x86"
	case strings.HasPrefix(m, "arm"):
		return "arm"
	case m == "riscv64" || m == "ppc64le" || m == "ppc64" || m == "s390x":
		return m
	}

	return "other"
}

// NormalCPUVendor names the processor's maker as people do.
func NormalCPUVendor(vendor string) string {
	switch vendor {
	case "GenuineIntel":
		return "Intel"
	case "AuthenticAMD":
		return "AMD"
	}

	return vendor
}

func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}
