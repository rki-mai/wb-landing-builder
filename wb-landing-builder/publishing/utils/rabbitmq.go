package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQConfig — параметры подключения к RabbitMQ.
type RabbitMQConfig struct {
	URL   string
	Queue string
}

type rabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   string
}

// NewRabbitMQ подключается к RabbitMQ и объявляет durable-очередь.
func NewRabbitMQ(cfg RabbitMQConfig) (Queue, error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to rabbitmq: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to open rabbitmq channel: %w", err)
	}

	_, err = channel.QueueDeclare(
		cfg.Queue,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return nil, fmt.Errorf("failed to declare rabbitmq queue: %w", err)
	}

	log.Printf("Connected to RabbitMQ (queue=%s)", cfg.Queue)
	return &rabbitMQ{
		conn:    conn,
		channel: channel,
		queue:   cfg.Queue,
	}, nil
}

func (r *rabbitMQ) Publish(ctx context.Context, task PublishTask) error {
	body, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to encode publish task: %w", err)
	}

	return r.channel.PublishWithContext(ctx, "", r.queue, false, false, amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		ContentType:  "application/json",
		Body:         body,
	})
}

func (r *rabbitMQ) Consume(ctx context.Context, handler func(ctx context.Context, task PublishTask) error) error {
	deliveries, err := r.channel.Consume(r.queue, "publish-worker", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to consume rabbitmq queue: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case delivery, ok := <-deliveries:
			if !ok {
				return nil
			}

			var task PublishTask
			if err := json.Unmarshal(delivery.Body, &task); err != nil {
				log.Printf("Invalid publish task payload: %v", err)
				_ = delivery.Nack(false, false)
				continue
			}

			if err := handler(ctx, task); err != nil {
				log.Printf("Failed to process publish task %s: %v", task.PublicationID, err)
				_ = delivery.Nack(false, true)
				continue
			}

			if err := delivery.Ack(false); err != nil {
				return fmt.Errorf("failed to ack publish task: %w", err)
			}
		}
	}
}

func (r *rabbitMQ) Close() error {
	if r.channel != nil {
		if err := r.channel.Close(); err != nil {
			return err
		}
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}
