package event

type Event struct {
	Topic   string
	Key     string
	Index   uint64
	Payload interface{}
}

type EventPublisher struct{}

func NewPublisher() *EventPublisher             { return &EventPublisher{} }
func (e EventPublisher) Publish(events []Event) {}
