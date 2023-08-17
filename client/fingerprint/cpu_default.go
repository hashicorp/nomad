// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package fingerprint

func (_ *CPUFingerprint) deriveReservableCores(string) []uint16 {
	return nil
}
