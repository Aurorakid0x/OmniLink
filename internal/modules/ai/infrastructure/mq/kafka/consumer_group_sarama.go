package kafka

import (
	"context"
	"errors"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/infrastructure/mq"

	"github.com/IBM/sarama"
)

type ConsumerConfig struct {
	Brokers  []string
	GroupID  string
	Topics   []string
	ClientID string
}

type saramaConsumer struct {
	cg     sarama.ConsumerGroup
	topics []string
}

func NewConsumer(cfg ConsumerConfig) (mq.Consumer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("kafka brokers is empty")
	}
	if strings.TrimSpace(cfg.GroupID) == "" {
		return nil, errors.New("kafka consumer group id is empty")
	}
	if len(cfg.Topics) == 0 {
		return nil, errors.New("kafka topics is empty")
	}

	sc := sarama.NewConfig()
	sc.Version = sarama.V2_8_0_0
	sc.Consumer.Offsets.Initial = sarama.OffsetNewest
	sc.Consumer.Group.Rebalance.Timeout = 30 * time.Second
	sc.Consumer.Group.Session.Timeout = 30 * time.Second
	sc.ClientID = strings.TrimSpace(cfg.ClientID)

	cg, err := sarama.NewConsumerGroup(cfg.Brokers, strings.TrimSpace(cfg.GroupID), sc)
	if err != nil {
		return nil, err
	}
	return &saramaConsumer{cg: cg, topics: cfg.Topics}, nil
}

func (c *saramaConsumer) Run(ctx context.Context, handler mq.Handler) error {
	if handler == nil {
		return errors.New("handler is nil")
	}
	h := &consumerGroupHandler{h: handler}

	for {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		err := c.cg.Consume(ctx, c.topics, h)
		if err != nil {
			return err
		}
	}
}

func (c *saramaConsumer) Close() error {
	if c == nil {
		return nil
	}
	return c.cg.Close()
}

type consumerGroupHandler struct {
	h mq.Handler
}

func (consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (h *consumerGroupHandler) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for m := range claim.Messages() {
		msg := mq.Message{
			Topic: m.Topic,
			Key:   m.Key,
			Value: m.Value,
		}

		if len(m.Headers) > 0 {
			msg.Headers = make(map[string]string, len(m.Headers))
			for _, hdr := range m.Headers {
				if hdr == nil || len(hdr.Key) == 0 {
					continue
				}
				msg.Headers[string(hdr.Key)] = string(hdr.Value)
			}
		}

		if err := h.h.Handle(sess.Context(), msg); err == nil {
			sess.MarkMessage(m, "")
		}
	}
	return nil
}
