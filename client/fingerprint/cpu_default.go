//+build !linux

package fingerprint

func (f *CPUFingerprint) deriveReservableCores(req *FingerprintRequest, totalCores int) ([]uint16, error) {
	return defaultReservableCores(totalCores), nil
}
