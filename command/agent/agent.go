package agent

type Agent struct {
}

func (a *Agent) Leave() error {
	return nil
}

func (a *Agent) Shutdown() error {
	return nil
}
