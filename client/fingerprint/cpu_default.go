//+build !linux

package fingerprint

func (f *CPUFingerprint) deriveReservableCores(req *FingerprintRequest) ([]uint16, error) {
	return nil, nil
}
