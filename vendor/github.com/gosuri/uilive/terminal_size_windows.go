// +build windows

package uilive

import (
	"os"
	"unsafe"
)

func getTermSize() (int, int) {
	out, err := os.Open("CONOUT$")
	if err != nil {
		return 0, 0
	}
	defer out.Close()

	var csbi consoleScreenBufferInfo
	ret, _, _ := procGetConsoleScreenBufferInfo.Call(out.Fd(), uintptr(unsafe.Pointer(&csbi)))
	if ret == 0 {
		return 0, 0
	}

	return int(csbi.window.right - csbi.window.left + 1), int(csbi.window.bottom - csbi.window.top + 1)
}
