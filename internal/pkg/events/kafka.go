package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jeogram/messenger/internal/config"
	"github.com/rs/zerolog/log"
	"github.com/segmentio/kafka-go"
)

// MessageCreatedEvent is published whenever a message is stored. Internal
// microservices within the monolith (notification, search, analytics) consume it.
type MessageCreatedEvent struct {
	MessageID string    `json:"message_id"`
	ChatID    string    `json:"chat_id"`
	SenderID  string    `json:"sender_id"`
	Type      string    `json:"type"`
	Text      string    `json:"text,omitempty"`
	MediaURL  string    `json:"media_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Producer publishes domain events to Kafka.
type Producer struct {
	writers map[string]*kafka.Writer
}

// NewProducer creates writers for the configured topics.
func NewProducer(cfg config.KafkaConfig) *Producer {
	makeWriter := func(topic string) *kafka.Writer {
		return &kafka.Writer{
			Addr:         kafka.TCP(cfg.Brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			Async:        false,
		}
	}
	return &Producer{
		writers: map[string]*kafka.Writer{
			cfg.MessageTopic: makeWriter(cfg.MessageTopic),
			cfg.NotifyTopic:  makeWriter(cfg.NotifyTopic),
		},
	}
}

// Publish serializes and writes a message to the given topic.
func (p *Producer) Publish(ctx context.Context, topic string, key string, value interface{}) error {
	w, ok := p.writers[topic]
	if !ok {
		return errUnknownTopic
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return w.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: payload,
		Time:  time.Now(),
	})
}

// Close closes all writers.
func (p *Producer) Close() error {
	for _, w := range p.writers {
		if err := w.Close(); err != nil {
			log.Error().Err(err).Msg("error closing kafka writer")
		}
	}
	return nil
}

// Consumer reads messages from a topic within a consumer group.
type Consumer struct {
	reader *kafka.Reader
}

// NewConsumer creates a Kafka consumer for a topic.
func NewConsumer(cfg config.KafkaConfig, topic string) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        cfg.Brokers,
		GroupID:        cfg.ConsumerGroup,
		Topic:          topic,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: time.Second,
	})
	return &Consumer{reader: r}
}

// Read blocks until a message is available or the context is cancelled.
func (c *Consumer) Read(ctx context.Context) (kafka.Message, error) {
	return c.reader.ReadMessage(ctx)
}

// Close closes the consumer.
func (c *Consumer) Close() error { return c.reader.Close() }

// EnsureTopics creates the configured topics if they do not yet exist.
func EnsureTopics(cfg config.KafkaConfig) {
	all := map[string]struct{}{}
	for _, t := range []string{cfg.MessageTopic, cfg.NotifyTopic} {
		all[t] = struct{}{}
	}
	for topic := range all {
		conn, err := kafka.Dial("tcp", cfg.Brokers[0])
		if err != nil {
			log.Warn().Err(err).Msg("kafka dial failed (topics may already exist)")
			return
		}
		partitions := 1
		replication := 1
		err = conn.CreateTopics(kafka.TopicConfig{
			Topic:             topic,
			NumPartitions:     partitions,
			ReplicationFactor: replication,
		})
		if err != nil {
			log.Debug().Err(err).Str("topic", topic).Msg("topic create (may already exist)")
		}
		_ = conn.Close()
	}
}

type unknownTopicError struct{}

var errUnknownTopic = &unknownTopicError{}

func (e *unknownTopicError) Error() string { return "unknown kafka topic" }
