// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !linux

package fingerprint

func (f *BridgeFingerprint) Fingerprint(*FingerprintRequest, *FingerprintResponse) error { return nil }
