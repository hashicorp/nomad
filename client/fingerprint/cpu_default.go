//go:build !linux
// +build !linux

package fingerprint

func (f *CPUFingerprint) deriveReservableCores(string) []uint16 {
	return nil
}
