package agent

import "io"

type Agent struct {
}

func NewAgent(config *Config, logOutput io.Writer) (*Agent, error) {
	a := &Agent{}
	return a, nil
}

func (a *Agent) Leave() error {
	return nil
}

func (a *Agent) Shutdown() error {
	return nil
}
