package scan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"strings"
)

// pciNames is the PCI ID Repository's list of names (pci.ids), as the system keeps it for lspci. The app reads the system's
// copy if there is one and brings none of its own; without it, graphics are listed by PCI ID only.
type pciNames struct {
	data []byte
}

// The places distributions keep pci.ids, compressed or not.
var pciIDsFiles = []string{"usr/share/hwdata/pci.ids", "usr/share/misc/pci.ids", "usr/share/pci.ids", "usr/local/share/pciids/pci.ids",
	"usr/share/hwdata/pci.ids.gz", "usr/share/misc/pci.ids.gz", "usr/share/pci.ids.gz"}

const maxPCIIDs = 16 * 1024 * 1024

func openPCIIDs(fsys fs.FS) *pciNames {
	for _, name := range pciIDsFiles {
		data := readFile(fsys, name)
		if data == nil {
			continue
		}
		if strings.HasSuffix(name, ".gz") {
			reader, err := gzip.NewReader(bytes.NewReader(data))
			if err != nil {
				continue
			}
			data, err = io.ReadAll(io.LimitReader(reader, maxPCIIDs))
			if err != nil {
				continue
			}
		}
		if len(data) > 0 && len(data) <= maxPCIIDs {
			return &pciNames{data: data}
		}
	}

	return nil
}

// lookup is the vendor's and the device's names, as lspci shows them ("NVIDIA Corporation", "TU102 [GeForce RTX 3090]");
// empty where the list does not know them.
func (p *pciNames) lookup(vendor, device string) (string, string) {
	var vendorName, deviceName string
	inVendor := false
	scanner := bufio.NewScanner(bytes.NewReader(p.data))
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "" || line[0] == '#':
			continue
		case line[0] != '\t':
			if inVendor {
				return vendorName, deviceName
			}
			// Device classes follow the vendors, as "C 03  Display controller".
			if strings.HasPrefix(line, "C ") {
				return vendorName, deviceName
			}
			if len(line) > 6 && strings.EqualFold(line[:4], vendor) && line[4:6] == "  " {
				vendorName, inVendor = strings.TrimSpace(line[6:]), true
			}
		case inVendor && !strings.HasPrefix(line, "\t\t"):
			if len(line) > 7 && strings.EqualFold(line[1:5], device) && line[5:7] == "  " {
				deviceName = strings.TrimSpace(line[7:])
			}
		}
	}

	return vendorName, deviceName
}
