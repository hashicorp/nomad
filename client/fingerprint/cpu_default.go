//go:build !linux

package fingerprint

func (_ *CPUFingerprint) deriveReservableCores(string) []uint16 {
	return nil
}
