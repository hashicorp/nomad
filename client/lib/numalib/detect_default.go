// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux && !darwin

package numalib

// PlatformScanners returns the set of SystemScanner for systems without a
// specific implementation.
func PlatformScanners() []SystemScanner {
	return []SystemScanner{
		new(Generic),
	}
}

// Generic implements SystemScanner as a fallback for operating systems without
// a specific implementation.
type Generic struct{}

func (g *Generic) ScanSystem(top *Topology) {
	scanGeneric(top)
}
