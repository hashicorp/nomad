// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package fingerprint

func (f *BridgeFingerprint) Fingerprint(*FingerprintRequest, *FingerprintResponse) error { return nil }
