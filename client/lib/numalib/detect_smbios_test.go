//go:build linux

package numalib

import (
	"testing"

	"github.com/shoenig/test/must"
)

func TestSmbios_blah(t *testing.T) {
	top := new(Topology)
	sysfs := new(Sysfs)
	smbios := new(Smbios)

	sysfs.ScanSystem(top)
	smbios.ScanSystem(top)

	for _, core := range top.Cores {
		must.Positive(t, core.GuessSpeed)
	}
}
