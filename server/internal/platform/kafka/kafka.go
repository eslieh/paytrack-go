// Package kafka adapts segmentio/kafka-go to the outbox Publisher and a
// retrying, dead-lettering consumer loop.
package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	kgo "github.com/segmentio/kafka-go"

	"github.com/eslieh/paytrack-go/server/internal/platform/outbox"
)

type Producer struct {
	w *kgo.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{w: &kgo.Writer{
		Addr:         kgo.TCP(brokers...),
		Balancer:     &kgo.Hash{}, // same key -> same partition -> per-key ordering
		RequiredAcks: kgo.RequireAll,
		BatchTimeout: 10 * time.Millisecond,
		WriteTimeout: 15 * time.Second,
	}}
}

func (p *Producer) Publish(ctx context.Context, recs []outbox.Record) error {
	msgs := make([]kgo.Message, len(recs))
	for i, r := range recs {
		msgs[i] = kgo.Message{Topic: r.Topic, Key: r.Key, Value: r.Value}
	}
	return p.w.WriteMessages(ctx, msgs...)
}

func (p *Producer) Close() error { return p.w.Close() }

// EnsureTopics creates topics (and their .dlq twins) if missing.
func EnsureTopics(ctx context.Context, brokers []string, partitions, replication int, topics ...string) error {
	d := &kgo.Dialer{Timeout: 10 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		return fmt.Errorf("kafka: dial: %w", err)
	}
	defer conn.Close()
	ctrl, err := conn.Controller()
	if err != nil {
		return fmt.Errorf("kafka: controller: %w", err)
	}
	cc, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ctrl.Host, strconv.Itoa(ctrl.Port)))
	if err != nil {
		return fmt.Errorf("kafka: dial controller: %w", err)
	}
	defer cc.Close()

	var cfgs []kgo.TopicConfig
	for _, t := range topics {
		for _, name := range []string{t, DLQ(t)} {
			cfgs = append(cfgs, kgo.TopicConfig{Topic: name, NumPartitions: partitions, ReplicationFactor: replication})
		}
	}
	if err := cc.CreateTopics(cfgs...); err != nil && !errors.Is(err, kgo.TopicAlreadyExists) {
		return fmt.Errorf("kafka: create topics: %w", err)
	}
	return nil
}

func DLQ(topic string) string { return topic + ".dlq" }

// Handler processes one message. Returning an error wrapped with Permanent
// skips retries and dead-letters immediately.
type Handler func(ctx context.Context, key, value []byte) error

type permanent struct{ error }

func (p permanent) Unwrap() error { return p.error }

func Permanent(err error) error { return permanent{err} }

type Consumer struct {
	Brokers  []string
	GroupID  string
	Topic    string
	Handler  Handler
	DLQ      *Producer
	Attempts int
}

// Run consumes until ctx is cancelled. Offsets are committed only after the
// handler succeeds or the message is dead-lettered, giving at-least-once
// processing; handlers must be idempotent.
func (c *Consumer) Run(ctx context.Context) error {
	r := kgo.NewReader(kgo.ReaderConfig{
		Brokers:        c.Brokers,
		GroupID:        c.GroupID,
		Topic:          c.Topic,
		MinBytes:       1,
		MaxBytes:       10 << 20,
		MaxWait:        time.Second,
		CommitInterval: 0, // synchronous commits
		StartOffset:    kgo.FirstOffset,
	})
	defer r.Close()
	log := slog.With("topic", c.Topic, "group", c.GroupID)
	log.Info("consumer started")

	attempts := c.Attempts
	if attempts == 0 {
		attempts = 5
	}
	for {
		m, err := r.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Error("fetch failed", "err", err)
			time.Sleep(time.Second)
			continue
		}
		var herr error
		for i := range attempts {
			if herr = c.Handler(ctx, m.Key, m.Value); herr == nil || errors.As(herr, new(permanent)) {
				break
			}
			if ctx.Err() != nil {
				return nil // uncommitted; redelivered to whoever owns the partition next
			}
			log.Warn("handler failed, retrying", "attempt", i+1, "err", herr, "offset", m.Offset)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Duration(1<<i) * time.Second):
			}
		}
		if herr != nil {
			log.Error("dead-lettering message", "err", herr, "partition", m.Partition, "offset", m.Offset)
			if err := c.DLQ.w.WriteMessages(ctx, kgo.Message{
				Topic: DLQ(c.Topic), Key: m.Key, Value: m.Value,
				Headers: []kgo.Header{{Key: "error", Value: []byte(herr.Error())}},
			}); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				return fmt.Errorf("kafka: dead-letter: %w", err)
			}
		}
		if err := r.CommitMessages(ctx, m); err != nil && ctx.Err() == nil {
			log.Error("commit failed", "err", err)
		}
	}
}

// RunFunc adapts Run for errgroup.Go.
func (c *Consumer) RunFunc(ctx context.Context) func() error {
	return func() error { return c.Run(ctx) }
}
