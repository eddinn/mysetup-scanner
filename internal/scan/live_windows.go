//go:build windows

package scan

import (
	"encoding/binary"
	"fmt"
	"os"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	kernel32                   = windows.NewLazySystemDLL("kernel32.dll")
	procGetSystemFirmwareTable = kernel32.NewProc("GetSystemFirmwareTable")
	procGlobalMemoryStatusEx   = kernel32.NewProc("GlobalMemoryStatusEx")
	displayClass               = windows.GUID{Data1: 0x4d36e968, Data2: 0xe325, Data3: 0x11ce, Data4: [8]byte{0xbf, 0xc1, 0x08, 0x00, 0x2b, 0xe1, 0x03, 0x18}}
)

const (
	ioctlStorageQueryProperty   = 0x2D1400
	ioctlDiskGetDriveGeometryEx = 0x700A0
	storageDeviceProperty       = 0
	storageSeekPenaltyProperty  = 7
	maxDisks                    = 64
)

// Read reads this computer.
func Read() Hardware {
	h := ReadWindows(WindowsSource{
		Firmware: firmwareTable(), CPUName: processorValue("ProcessorNameString"), CPUVendor: processorValue("VendorIdentifier"),
		LogicalProcessors: runtime.NumCPU(), Adapters: displayAdapters(), Disks: physicalDisks(), TotalMemory: totalMemory(),
		Version: osVersion(), Architecture: architecture(),
	})
	h.FinalNotes()

	return h
}

// firmwareTable is the SMBIOS table Windows keeps for any program ('RSMB'), without its 8-byte header.
func firmwareTable() []byte {
	const rsmb = 0x52534D42
	size, _, _ := procGetSystemFirmwareTable.Call(rsmb, 0, 0, 0)
	if size == 0 || size > 1<<20 {
		return nil
	}
	buf := make([]byte, size)
	got, _, _ := procGetSystemFirmwareTable.Call(rsmb, 0, uintptr(unsafe.Pointer(&buf[0])), size)
	if got < 8 || got > size {
		return nil
	}
	length := int(binary.LittleEndian.Uint32(buf[4:8]))
	if 8+length > int(got) {
		return nil
	}

	return buf[8 : 8+length]
}

func processorValue(name string) string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	value, _, err := key.GetStringValue(name)
	if err != nil {
		return ""
	}

	return value
}

// displayAdapters lists the present display adapters with their instance IDs and descriptions.
func displayAdapters() []WindowsAdapter {
	set, err := windows.SetupDiGetClassDevsEx(&displayClass, "", 0, windows.DIGCF_PRESENT, 0, "")
	if err != nil {
		return nil
	}
	defer set.Close()
	var adapters []WindowsAdapter
	for i := 0; i < maxDisks; i++ {
		info, err := set.EnumDeviceInfo(i)
		if err != nil {
			break
		}
		id, err := set.DeviceInstanceID(info)
		if err != nil {
			continue
		}
		name, _ := set.DeviceRegistryProperty(info, windows.SPDRP_DEVICEDESC)
		description, _ := name.(string)
		adapters = append(adapters, WindowsAdapter{InstanceID: id, Name: description})
	}

	return adapters
}

// physicalDisks asks each disk for its product name, bus, seek penalty and size, through a handle opened with no access rights,
// which needs no administrator. The descriptor's serial number is never read.
func physicalDisks() []WindowsDisk {
	var disks []WindowsDisk
	for n := 0; n < maxDisks; n++ {
		name, _ := windows.UTF16PtrFromString(fmt.Sprintf(`\\.\PhysicalDrive%d`, n))
		handle, err := windows.CreateFile(name, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			continue
		}
		disks = append(disks, queryDisk(handle))
		windows.CloseHandle(handle)
	}

	return disks
}

func queryDisk(handle windows.Handle) WindowsDisk {
	var d WindowsDisk
	query := make([]byte, 12)
	out := make([]byte, 1024)
	var got uint32
	if windows.DeviceIoControl(handle, ioctlStorageQueryProperty, &query[0], 12, &out[0], uint32(len(out)), &got, nil) == nil && got >= 32 {
		vendor := descriptorString(out[:got], binary.LittleEndian.Uint32(out[12:]))
		product := descriptorString(out[:got], binary.LittleEndian.Uint32(out[16:]))
		d.Bus = int(binary.LittleEndian.Uint32(out[28:]))
		d.Model = product
		if vendor != "" && !strings.EqualFold(vendor, "ATA") && !strings.EqualFold(vendor, "NVMe") && !strings.HasPrefix(strings.ToLower(product), strings.ToLower(vendor)) {
			d.Model = vendor + " " + product
		}
	}
	binary.LittleEndian.PutUint32(query[0:], storageSeekPenaltyProperty)
	penalty := make([]byte, 12)
	if windows.DeviceIoControl(handle, ioctlStorageQueryProperty, &query[0], 12, &penalty[0], uint32(len(penalty)), &got, nil) == nil && got >= 9 {
		seeks := penalty[8] != 0
		d.SeekPenalty = &seeks
	}
	geometry := make([]byte, 256)
	if windows.DeviceIoControl(handle, ioctlDiskGetDriveGeometryEx, nil, 0, &geometry[0], uint32(len(geometry)), &got, nil) == nil && got >= 32 {
		d.Bytes = int64(binary.LittleEndian.Uint64(geometry[24:]))
	}

	return d
}

// descriptorString is the NUL-terminated string at an offset of the device descriptor; offset 0 means none.
func descriptorString(buf []byte, offset uint32) string {
	if offset == 0 || int(offset) >= len(buf) {
		return ""
	}
	end := int(offset)
	for end < len(buf) && buf[end] != 0 {
		end++
	}

	return strings.TrimSpace(string(buf[offset:end]))
}

func totalMemory() int64 {
	var status struct {
		Length, MemoryLoad                     uint32
		TotalPhys, AvailPhys, TotalPageFile    uint64
		AvailPageFile, TotalVirtual, AvailVirt uint64
		AvailExtendedVirtual                   uint64
	}
	status.Length = uint32(unsafe.Sizeof(status))
	if ok, _, _ := procGlobalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status))); ok == 0 {
		return 0
	}

	return int64(status.TotalPhys)
}

func osVersion() string {
	v := windows.RtlGetVersion()

	return fmt.Sprintf("%d.%d.%d", v.MajorVersion, v.MinorVersion, v.BuildNumber)
}

func architecture() string {
	if a := os.Getenv("PROCESSOR_ARCHITEW6432"); a != "" {
		return a
	}

	return os.Getenv("PROCESSOR_ARCHITECTURE")
}
