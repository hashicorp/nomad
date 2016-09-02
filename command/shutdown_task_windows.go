package command

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
)

type ShutdownTaskCommand struct {
	Meta
}

func (e *ShutdownTaskCommand) Help() string {
	helpText := `
	This is a command used by Nomad internally to gracefully stop windows executor task."
	`
	return strings.TrimSpace(helpText)
}

func (e *ShutdownTaskCommand) Synopsis() string {
	return "internal - shutdown windows task"
}

func (e *ShutdownTaskCommand) Run(args []string) int {

	if len(args) == 0 {
		e.Ui.Error("task pid is not provided")
		return 1
	}
	pid, err := strconv.Atoi(args[0])
	if err != nil {
		e.Ui.Error("can't parse pid")
		return 2
	}
	if err := sendCtrlC(pid); err != nil {
		e.Ui.Error(fmt.Sprintf("can't send Ctrl+C to process %v", err))
		return 3
	}
	return 0
}

func sendCtrlC(pid int) error {
	var kernel32 = syscall.NewLazyDLL("Kernel32.dll")
	var freeConsole = kernel32.NewProc("FreeConsole")
	var attachConsole = kernel32.NewProc("AttachConsole")
	var setConsoleCtrlHandler = kernel32.NewProc("SetConsoleCtrlHandler")
	var generateConsoleCtrlEvent = kernel32.NewProc("GenerateConsoleCtrlEvent")
	// Close current console
	if r, _, err := freeConsole.Call(); r == 0 {
		return fmt.Errorf("Can't FreeConsole. Error code %v", err)
	}
	// Stach to job console
	if r, _, err := attachConsole.Call(uintptr(pid)); r == 0 {
		return fmt.Errorf("Can't AttachConsole. Error code %v", err)
	}
	// Disable Ctrl+C handling for our own program, so we don't "kill" ourselves
	if r, _, err := setConsoleCtrlHandler.Call(0, 1); r == 0 {
		return fmt.Errorf("Can't SetConsoleCtrlHandler. Error code %v", err)
	}

	if r, _, err := generateConsoleCtrlEvent.Call(uintptr(syscall.CTRL_C_EVENT), 0); r == 0 {
		return fmt.Errorf("Can't GenerateConsoleCtrlEvent. Error code %v", err)
	}
	return nil
}
