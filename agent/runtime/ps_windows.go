//go:build windows

package runtime

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func runBuiltinPS() (int32, string, string) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return 1, "", "process list: " + err.Error()
	}
	defer windows.CloseHandle(snap)

	var pe windows.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	if err := windows.Process32First(snap, &pe); err != nil {
		return 1, "", "process list: " + err.Error()
	}

	var b strings.Builder
	b.WriteString("PID\tPPID\tTHREADS\tNAME\n")
	for {
		name := windows.UTF16ToString(pe.ExeFile[:])
		fmt.Fprintf(&b, "%d\t%d\t%d\t%s\n", pe.ProcessID, pe.ParentProcessID, pe.Threads, name)
		if err := windows.Process32Next(snap, &pe); err != nil {
			break
		}
	}
	return 0, b.String(), ""
}
