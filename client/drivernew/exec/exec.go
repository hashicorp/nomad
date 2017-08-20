package exec

const (
	Name = "exec"
)

type Exec struct{}

// New returns a new ExecDriver instance
func New() (interface{}, error) {
	return &Exec{}, nil
}

func (e *Exec) Name() (string, error) {
	return Name, nil
}

func (e *Exec) Exit() error {
	return nil
}
