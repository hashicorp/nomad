// +build !linux

package fingerprint

func (f *BridgeFingerprint) Fingerprint(*FingerprintRequest, *FingerprintResponse) error { return nil }
