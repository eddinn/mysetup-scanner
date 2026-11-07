//go:build windows

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	user32                    = windows.NewLazySystemDLL("user32.dll")
	procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	procGlobalAlloc           = kernel32.NewProc("GlobalAlloc")
	procGlobalFree            = kernel32.NewProc("GlobalFree")
	procGlobalLock            = kernel32.NewProc("GlobalLock")
	procGlobalUnlock          = kernel32.NewProc("GlobalUnlock")
	procRtlMoveMemory         = kernel32.NewProc("RtlMoveMemory")
	procOpenClipboard         = user32.NewProc("OpenClipboard")
	procEmptyClipboard        = user32.NewProc("EmptyClipboard")
	procSetClipboardData      = user32.NewProc("SetClipboardData")
	procCloseClipboard        = user32.NewProc("CloseClipboard")
)

// detach, in a console window Windows opened for the app alone (a double-click), starts the app again without a console and
// tells this one to end, so the window closes and only the browser page is left. It works the same in the classic console and
// in Windows Terminal, whichever opened the window. When it cannot start the app again, the app goes on here, window and all.
func detach() bool {
	if !ownWindow() {
		return false
	}
	self, err := os.Executable()
	if err != nil {
		return false
	}
	cmd := exec.Command(self, "--detached")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		return false
	}
	_ = cmd.Process.Release()

	return true
}

// tell shows a message in a box of its own, for the app started again without a console (detach), which has nowhere else to say it.
func tell(message string) {
	const mbOK, mbIconInformation = 0x0, 0x40
	text, err := windows.UTF16PtrFromString(message)
	if err != nil {
		return
	}
	title, _ := windows.UTF16PtrFromString("MySetup scan")
	_, _ = windows.MessageBox(0, text, title, mbOK|mbIconInformation)
}

// ownWindow is true when Windows opened a console window for this program alone, as on a double-click: the window would close
// the moment the program ends.
func ownWindow() bool {
	var processes [2]uint32
	n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&processes[0])), 2)

	return n == 1
}

// copyToClipboard puts the report on the clipboard as text, for the member to paste. It writes nothing to disk.
func copyToClipboard(text string) bool {
	const cfUnicodeText, gmemMoveable = 13, 0x0002
	data := utf16.Encode([]rune(text + "\x00"))
	size := uintptr(len(data) * 2)
	if ok, _, _ := procOpenClipboard.Call(0); ok == 0 {
		return false
	}
	defer procCloseClipboard.Call()
	procEmptyClipboard.Call()
	handle, _, _ := procGlobalAlloc.Call(gmemMoveable, size)
	if handle == 0 {
		return false
	}
	pointer, _, _ := procGlobalLock.Call(handle)
	if pointer == 0 {
		procGlobalFree.Call(handle)

		return false
	}
	// Copied by Windows itself, so the locked memory is never turned into a Go pointer.
	procRtlMoveMemory.Call(pointer, uintptr(unsafe.Pointer(&data[0])), size)
	procGlobalUnlock.Call(handle)
	// On success the clipboard owns the memory; otherwise it is still ours to free.
	if done, _, _ := procSetClipboardData.Call(cfUnicodeText, handle); done == 0 {
		procGlobalFree.Call(handle)

		return false
	}

	return true
}

// keepWindowOpen, in a window of its own, copies the report to the clipboard, says so, and waits for Enter before the window closes.
func keepWindowOpen(report string, stderr io.Writer) {
	if !ownWindow() {
		return
	}
	fmt.Fprintln(stderr)
	if copyToClipboard(report) {
		fmt.Fprintln(stderr, "The report is copied: paste it on your build page.")
	} else {
		fmt.Fprintln(stderr, `Select the report from "mysetup-report 1" to "mysetup-report-end" and copy it.`)
	}
	fmt.Fprintln(stderr, "Press Enter to close this window.")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
