// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build windows
// +build windows

package testtask

import (
	"fmt"
	"os"
)

func executeProcessGroup(gid string) {
	fmt.Fprintf(os.Stderr, "TODO: implement process groups are on windows\n")
	fmt.Fprintf(os.Stderr, "TODO: see https://github.com/hashicorp/nomad/blob/109c5ef650206fc62334d202002cda92ceb67399/drivers/shared/executor/executor_windows.go#L9-L17\n")
	os.Exit(1)
}
