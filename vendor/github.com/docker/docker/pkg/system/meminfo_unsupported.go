<<<<<<< df68129e5afd485c281a9b7a3cd36d3ed32ffd83
// +build !linux,!windows,!solaris

package system

// ReadMemInfo is not supported on platforms other than linux and windows.
func ReadMemInfo() (*MemInfo, error) {
	return nil, ErrNotSupportedPlatform
}
||||||| merged common ancestors
=======
// +build !linux,!windows

package system

// ReadMemInfo is not supported on platforms other than linux and windows.
func ReadMemInfo() (*MemInfo, error) {
	return nil, ErrNotSupportedPlatform
}
>>>>>>> Added missing vendored dependencies
