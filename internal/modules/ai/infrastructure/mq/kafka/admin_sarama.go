package kafka

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/IBM/sarama"
)

type TopicAdminConfig struct {
	Brokers  []string
	ClientID string
}

func EnsureTopic(cfg TopicAdminConfig, topic string, partitions int32, replicationFactor int16) error {
	if len(cfg.Brokers) == 0 {
		return errors.New("kafka brokers is empty")
	}
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return errors.New("kafka topic is empty")
	}
	if partitions <= 0 {
		partitions = 1
	}
	if replicationFactor <= 0 {
		replicationFactor = 1
	}

	sc := sarama.NewConfig()
	sc.Version = sarama.V2_8_0_0
	sc.ClientID = strings.TrimSpace(cfg.ClientID)

	admin, err := sarama.NewClusterAdmin(cfg.Brokers, sc)
	if err != nil {
		return err
	}
	defer admin.Close()

	topics, err := admin.ListTopics()
	if err != nil {
		return err
	}
	if _, ok := topics[topic]; ok {
		return nil
	}

	td := &sarama.TopicDetail{
			NumPartitions:     partitions,
			ReplicationFactor: replicationFactor,
			ConfigEntries: map[string]*string{
				"retention.ms": strPtr(strconv.FormatInt((24*time.Hour).Milliseconds(), 10)),
			},
		}
	if err := admin.CreateTopic(topic, td, false); err != nil {
		if errors.Is(err, sarama.ErrTopicAlreadyExists) {
			return nil
		}
		return err
	}
	return nil
}

func strPtr(v string) *string {
	s := v
	return &s
}