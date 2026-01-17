package kafka

import (
	"context"
	"errors"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/infrastructure/mq"

	"github.com/IBM/sarama"
)

type PublisherConfig struct {
	Brokers  []string
	ClientID string
}

type saramaPublisher struct {
	p sarama.SyncProducer
}

func NewPublisher(brokers []string) (mq.Publisher, error) {
	return NewSaramaPublisher(PublisherConfig{Brokers: brokers})
}

func NewSaramaPublisher(cfg PublisherConfig) (mq.Publisher, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka brokers is empty")
	}

	sc := sarama.NewConfig()
	sc.Version = sarama.V2_8_0_0
	sc.Producer.Return.Successes = true
	sc.Producer.RequiredAcks = sarama.WaitForAll
	sc.Producer.Retry.Max = 10
	sc.Producer.Retry.Backoff = 100 * time.Millisecond
	sc.Producer.Idempotent = true
	sc.Net.MaxOpenRequests = 1
	sc.Producer.Partitioner = sarama.NewHashPartitioner
	sc.ClientID = strings.TrimSpace(cfg.ClientID)

	p, err := sarama.NewSyncProducer(cfg.Brokers, sc)
	if err != nil {
		return nil, err
	}
	return &saramaPublisher{p: p}, nil
}

func (s *saramaPublisher) Publish(ctx context.Context, msg mq.Message) (mq.PublishResult, error) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return mq.PublishResult{}, ctx.Err()
		default:
		}
	}
	if strings.TrimSpace(msg.Topic) == "" {
		return mq.PublishResult{}, errors.New("kafka topic is empty")
	}

	m := &sarama.ProducerMessage{
		Topic: msg.Topic,
		Key:   sarama.ByteEncoder(msg.Key),
		Value: sarama.ByteEncoder(msg.Value),
	}

	if len(msg.Headers) > 0 {
		m.Headers = make([]sarama.RecordHeader, 0, len(msg.Headers))
		for k, v := range msg.Headers {
			kk := strings.TrimSpace(k)
			if kk == "" {
				continue
			}
			m.Headers = append(m.Headers, sarama.RecordHeader{
				Key:   []byte(kk),
				Value: []byte(v),
			})
		}
	}

	partition, offset, err := s.p.SendMessage(m)
	if err != nil {
		return mq.PublishResult{}, err
	}
	return mq.PublishResult{Partition: partition, Offset: offset}, nil
}

func (s *saramaPublisher) Close() error {
	if s == nil || s.p == nil {
		return nil
	}
	return s.p.Close()
}
