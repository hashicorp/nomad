//go:build !linux
// +build !linux

package cgutil

import (
	"github.com/hashicorp/go-hclog"
)

func NewCpusetManager(_ string, _ hclog.Logger) CpusetManager { return noopCpusetManager{} }
