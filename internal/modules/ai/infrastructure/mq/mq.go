package mq

import "context"

type Message struct {
	Topic   string
	Key     []byte
	Value   []byte
	Headers map[string]string
}

type PublishResult struct {
	Partition int32
	Offset    int64
}

type Publisher interface {
	Publish(ctx context.Context, msg Message) (PublishResult, error)
	Close() error
}

type Handler interface {
	Handle(ctx context.Context, msg Message) error
}

type Consumer interface {
	Run(ctx context.Context, handler Handler) error
	Close() error
}
