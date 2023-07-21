package numalib

import (
	"fmt"
	"testing"
)

func TestScanSysfs(t *testing.T) {
	top := ScanSysfs()

	fmt.Println("top", top)

	fmt.Println("cores", top.NumCores(), "pcores", top.NumPCores(), "ecores", top.NumECores())
	fmt.Println("total_compute", top.TotalCompute())
}
