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
	procGetShellWindow        = user32.NewProc("GetShellWindow")
	procFreeConsole           = kernel32.NewProc("FreeConsole")
	advapi32                  = windows.NewLazySystemDLL("advapi32.dll")
	procCreateProcessWithTokW = advapi32.NewProc("CreateProcessWithTokenW")
)

// unelevate, run as administrator, starts the app again as the signed-in user without administrator rights, with the token of the
// desktop's own shell (Explorer), and tells this one to end, so the page and the browser run as that user: Windows needs no
// administrator for the report. With UAC off the shell is elevated too, and with no shell there is nobody to start it as; then
// the app goes on as it is, and opens the browser through Explorer (page.Session.Open).
func unelevate() bool {
	if !windows.GetCurrentProcessToken().IsElevated() {
		return false
	}
	shell, _, _ := procGetShellWindow.Call()
	if shell == 0 {
		return false
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(windows.HWND(shell), &pid); err != nil || pid == 0 {
		return false
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false
	}
	defer func() { _ = windows.CloseHandle(process) }()
	var token windows.Token
	if windows.OpenProcessToken(process, windows.TOKEN_DUPLICATE, &token) != nil {
		return false
	}
	defer func() { _ = token.Close() }()
	var primary windows.Token
	access := uint32(windows.TOKEN_QUERY | windows.TOKEN_DUPLICATE | windows.TOKEN_ASSIGN_PRIMARY | windows.TOKEN_ADJUST_DEFAULT | windows.TOKEN_ADJUST_SESSIONID)
	if windows.DuplicateTokenEx(token, access, nil, windows.SecurityImpersonation, windows.TokenPrimary, &primary) != nil {
		return false
	}
	defer func() { _ = primary.Close() }()
	if primary.IsElevated() {
		return false
	}
	self, err := os.Executable()
	if err != nil {
		return false
	}
	app, err := windows.UTF16PtrFromString(self)
	if err != nil {
		return false
	}
	// Without a console of its own, or, where Windows does not allow that here, with one that is never shown.
	for _, flags := range []uint32{windows.DETACHED_PROCESS, windows.CREATE_NEW_CONSOLE} {
		line, err := windows.UTF16FromString(windows.EscapeArg(self) + " --detached")
		if err != nil {
			return false
		}
		startup := windows.StartupInfo{Flags: windows.STARTF_USESHOWWINDOW, ShowWindow: windows.SW_HIDE}
		startup.Cb = uint32(unsafe.Sizeof(startup))
		var started windows.ProcessInformation
		ok, _, _ := procCreateProcessWithTokW.Call(uintptr(primary), 0, uintptr(unsafe.Pointer(app)), uintptr(unsafe.Pointer(&line[0])),
			uintptr(flags), 0, 0, uintptr(unsafe.Pointer(&startup)), uintptr(unsafe.Pointer(&started)))
		if ok != 0 {
			_ = windows.CloseHandle(started.Thread)
			_ = windows.CloseHandle(started.Process)

			return true
		}
	}

	return false
}

// leaveOwnConsole, in the app started again by detach or unelevate, lets go of a console Windows gave it alone, so no window
// stays behind the page.
func leaveOwnConsole() {
	var processes [2]uint32
	if n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&processes[0])), 2); n == 1 {
		procFreeConsole.Call()
	}
}

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

// ownWindow is true when Windows opened a console window for this program alone, as on a double-click, and the program reads
// and writes that window: it would close the moment the program ends. Started by another program that takes its input or output
// (WSL, or a script redirecting it), it is false even when that program gave it a console of its own, so it never waits there
// for an Enter nobody can see.
func ownWindow() bool {
	for _, f := range []*os.File{os.Stdin, os.Stdout} {
		var mode uint32
		if windows.GetConsoleMode(windows.Handle(f.Fd()), &mode) != nil {
			return false
		}
	}
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
		fmt.Fprintln(stderr, "The report is already copied for you, so you can skip step 1.")
	} else {
		fmt.Fprintln(stderr, "The report could not be copied for you, so do step 1 yourself.")
	}
	fmt.Fprintln(stderr, "When you have pasted it on your build page, press Enter here to close this window.")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
