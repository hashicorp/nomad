package stream

const (
	AllKeys = "*"
)

type Topic string

type Event struct {
	Topic   Topic
	Key     string
	Index   uint64
	Payload interface{}
}
