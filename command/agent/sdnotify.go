package agent

// These constants are for readiness signalling via the systemd notify protocol.
// The functions we send these messages to are no-op on non-Linux systems. See
// also https://www.man7.org/linux/man-pages/man3/sd_notify.3.html
const (
	sdReady    = "READY=1"
	sdStopping = "STOPPING=1"
)
