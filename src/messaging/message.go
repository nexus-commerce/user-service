package messaging

type Message struct {
	ID    string
	Topic string
	Key   string
	Value []byte
}
