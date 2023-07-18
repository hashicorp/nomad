package proclib

type ProcessWrangler interface {
	Kill() error
	Cleanup() error

	SetAttributes(map[string]string)
}
