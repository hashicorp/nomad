package stream

type Sink interface {
	Start() error
	Stop()
	Subscribe() error
}

type Manager interface {
}
