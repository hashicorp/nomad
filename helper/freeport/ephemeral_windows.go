//+build windows

package freeport

// For now we hard-code the Windows ephemeral port range, which is documented by
// Microsoft to be in this range for Vista / Server 2008 and newer.
//
// https://support.microsoft.com/en-us/help/832017/service-overview-and-network-port-requirements-for-windows
func getEphemeralPortRange() (int, int, error) {
	return 49152, 65535, nil
}
