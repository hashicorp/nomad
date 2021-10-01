//go:build !linux
// +build !linux

package fingerprint

func (f *CGroupFingerprint) Fingerprint(*FingerprintRequest, *FingerprintResponse) error {
	return nil
}
