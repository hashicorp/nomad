package csi

import "errors"

var (
	ErrCSIClientRPCIgnorable  = errors.New("CSI client error (ignorable)")
	ErrCSIClientRPCRetryable  = errors.New("CSI client error (retryable)")
	ErrCSIVolumeMaxClaims     = errors.New("volume max claims reached")
	ErrCSIVolumeUnschedulable = errors.New("volume is currently unschedulable")
	ErrCSIPluginInUse         = errors.New("plugin in use")
)
