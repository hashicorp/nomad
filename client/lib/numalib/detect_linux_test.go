package numalib

import (
	"fmt"
	"testing"
)

func TestScanSysfs(t *testing.T) {
	top := ScanSysfs()

	fmt.Println("top", top)
}
