// Package report writes the MySetup scan report (docs/report-format.md), the same text the scripts print. It is the only
// way anything reaches the report: a field the format does not name is a programming error, and every value is checked
// and cleaned here, as the scripts' emit() and Write-Field do.
package report

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version is the report format's version, the number on its first line.
const Version = 1

// MaxItems is the most devices of one kind a report lists.
const MaxItems = 64

// Report collects fields in the order they are written.
type Report struct {
	lines []string
}

var (
	textField = regexp.MustCompile(`^(cpu\.vendor|cpu\.model|motherboard\.vendor|motherboard\.model|gpu\.[0-9]+\.(vendor|model)|` +
		`memory\.[0-9]+\.(maker|part)|storage\.[0-9]+\.model)$`)
	numberField = regexp.MustCompile(`^(cpu\.cores|cpu\.threads|memory\.total_mb|memory\.[0-9]+\.(size_mb|speed_mts|configured_speed_mts)|` +
		`storage\.[0-9]+\.size_bytes)$`)
	pciField  = regexp.MustCompile(`^gpu\.[0-9]+\.(pci|subsystem)$`)
	wordField = regexp.MustCompile(`^(source|tested|os\.family|os\.name|os\.version|os\.architecture|environment|` +
		`memory\.[0-9]+\.(type|form)|storage\.[0-9]+\.(interface|media)|capability\.[a-z_]+(\.reason)?)$`)
	pciValue  = regexp.MustCompile(`^[0-9a-f]{4}:[0-9a-f]{4}$`)
	wordValue = regexp.MustCompile(`^[A-Za-z0-9._+/-]+$`)
	digits    = regexp.MustCompile(`^[0-9]+$`)
)

// Set writes a field. An empty or unacceptable value writes nothing: a missing field means unknown, never a guess.
func (r *Report) Set(field, value string) {
	switch {
	case textField.MatchString(field):
		value = CleanText(value)
	case numberField.MatchString(field):
		if !digits.MatchString(value) || strings.TrimLeft(value, "0") == "" {
			return
		}
	case pciField.MatchString(field):
		if !pciValue.MatchString(value) {
			return
		}
	case wordField.MatchString(field):
		if !wordValue.MatchString(value) {
			return
		}
		// What the memory tables say when they do not know.
		if strings.HasPrefix(field, "memory.") && (value == "Other" || value == "Unknown") {
			return
		}
	default:
		panic(fmt.Sprintf("report: no field named %q", field))
	}
	if value != "" {
		r.lines = append(r.lines, field+"="+value)
	}
}

// SetNumber writes a whole number; zero and less mean unknown.
func (r *Report) SetNumber(field string, value int64) {
	if value > 0 {
		r.Set(field, strconv.FormatInt(value, 10))
	}
}

// Capability writes whether a part could be read: "available", or the reason it could not.
func (r *Report) Capability(name, state string) {
	if state == "available" {
		r.Set("capability."+name, "available")

		return
	}
	r.Set("capability."+name, "unavailable")
	r.Set("capability."+name+".reason", state)
}

// Text is the report, from its first line to its last.
func (r *Report) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "mysetup-report %d\n", Version)
	for _, line := range r.lines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString("mysetup-report-end\n")

	return b.String()
}

var (
	placeholders = map[string]bool{}
	spaces       = regexp.MustCompile(`[ \t]+`)
	zeros        = regexp.MustCompile(`^(0+|0x0+|x+|[-. ]*)$`)
	partNumber   = regexp.MustCompile(`^((array[0-9]*_)?partnumber[0-9]*|partnum[0-9]*|manufacturer[0-9]*)$`)
	hh           = `[0-9a-f][0-9a-f]`
	mac          = regexp.MustCompile(hh + `[:-]` + hh + `[:-]` + hh + `[:-]` + hh + `[:-]` + hh + `[:-]` + hh)
	uuid         = regexp.MustCompile(hh + hh + hh + hh + `-` + hh + hh + `-` + hh + hh + `-` + hh + hh + `-` + hh + hh + hh + hh + hh + hh)
	ipv4         = regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+`)
	email        = regexp.MustCompile(`[^ ]@[^ ]`)
)

func init() {
	for _, word := range strings.Split("to be filled by o.e.m.|to be filled by oem|default string|not applicable|"+
		"not specified|not available|n/a|na|none|null|unknown|undefined|empty|[empty]|o.e.m.|oem|"+
		"system manufacturer|system product name|system version|baseboard product name|"+
		"base board product name|no dimm|nodimm|not installed|no module installed|dimm|module|"+
		"part number|serial number|1234567890|123456789012", "|") {
		placeholders[word] = true
	}
}

// CleanText keeps printable ASCII with single spaces, at most 120 characters. It returns nothing for a placeholder such as
// "To Be Filled By O.E.M.", or for anything shaped like an address, an ID or a home directory: a hardware name never looks
// like those. The same rules as the scripts' clean_text.
func CleanText(value string) string {
	var b strings.Builder
	for _, c := range value {
		if c >= ' ' && c <= '~' {
			b.WriteRune(c)
		}
	}
	text := strings.TrimSpace(spaces.ReplaceAllString(b.String(), " "))
	low := strings.ToLower(text)
	switch {
	case placeholders[low], zeros.MatchString(low), partNumber.MatchString(low), mac.MatchString(low), uuid.MatchString(low),
		ipv4.MatchString(low), email.MatchString(low), strings.Contains(low, "/home/"), strings.Contains(low, "/users/"),
		strings.Contains(low, `c:\`):
		return ""
	}
	if len(text) > 120 {
		text = text[:120]
	}

	return text
}
