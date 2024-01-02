// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package fingerprint

func (_ *CPUFingerprint) deriveReservableCores(string) []uint16 {
	return nil
}
