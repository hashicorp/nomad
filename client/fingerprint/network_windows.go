// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package fingerprint

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// linkSpeed returns link speed in Mb/s, or 0 when unable to determine it.
func (f *NetworkFingerprint) linkSpeed(device string) int {
	command := fmt.Sprintf("Get-NetAdapter -Name '%s' -ErrorAction Ignore | Select-Object -ExpandProperty 'Speed'", device)
	path := "powershell.exe"
	powershellParams := "-NoProfile"

	outBytes, err := exec.Command(path, powershellParams, command).Output()
	if err != nil {
		f.logger.Warn("failed to detect link speed", "device", device, "path", path, "command", command, "error", err)
		return 0
	}
	output := strings.TrimSpace(string(outBytes))

	value, err := strconv.Atoi(output)
	if err != nil {
		f.logger.Warn("unable to parse Speed value", "device", device, "value", output, "error", err)
		return 0
	}

	return value / 1000000
}
