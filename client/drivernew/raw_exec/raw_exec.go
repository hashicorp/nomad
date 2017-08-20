package rawexec

const (
	Name = "raw_exec"
)

type RawExec struct{}

// New returns a new RawExec driver instance
func New() (interface{}, error) {
	return &RawExec{}, nil
}

func (r *RawExec) Name() (string, error) {
	return Name, nil
}

func (r *RawExec) Exit() error {
	return nil
}
