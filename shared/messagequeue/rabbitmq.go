package messagequeue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// MessageType defines the type of message
type MessageType string

const (
	PostCreated    MessageType = "post_created"
	FollowCreated  MessageType = "follow_created"
	CommentCreated MessageType = "comment_created"
)

// Message represents a message in the queue
type Message struct {
	Type      MessageType     `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
	Version   string          `json:"version"`
}

// RabbitMQ provides methods for interacting with RabbitMQ
type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// NewRabbitMQ creates a new RabbitMQ client
func NewRabbitMQ() (*RabbitMQ, error) {
	// Get connection details from environment variables
	host := os.Getenv("RABBITMQ_HOST")
	port := os.Getenv("RABBITMQ_PORT")
	user := os.Getenv("RABBITMQ_USER")
	pass := os.Getenv("RABBITMQ_PASS")

	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5672"
	}
	if user == "" {
		user = "admin"
	}
	if pass == "" {
		pass = "admin"
	}

	// Connect to RabbitMQ
	connStr := fmt.Sprintf("amqp://%s:%s@%s:%s/", user, pass, host, port)
	conn, err := amqp.Dial(connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Create a channel
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	// Create the client
	client := &RabbitMQ{
		conn:    conn,
		channel: channel,
	}

	// Declare exchanges and queues
	if err := client.setupInfrastructure(); err != nil {
		client.Close()
		return nil, err
	}

	return client, nil
}

// setupInfrastructure declares exchanges and queues
func (r *RabbitMQ) setupInfrastructure() error {
	// Declare the main exchange
	err := r.channel.ExchangeDeclare(
		"timeline_events", // name
		"topic",           // type
		true,              // durable
		false,             // auto-deleted
		false,             // internal
		false,             // no-wait
		nil,               // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare dead letter exchange
	err = r.channel.ExchangeDeclare(
		"timeline_events_dlx", // name
		"topic",               // type
		true,                  // durable
		false,                 // auto-deleted
		false,                 // internal
		false,                 // no-wait
		nil,                   // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare dead letter exchange: %w", err)
	}

	// Declare queues with their bindings
	queues := []struct {
		name       string
		routingKey string
		args       amqp.Table
	}{
		{
			name:       "post_events",
			routingKey: "event.post.*",
			args: amqp.Table{
				"x-dead-letter-exchange":    "timeline_events_dlx",
				"x-dead-letter-routing-key": "dead.post",
				"x-message-ttl":             int32(1000 * 60 * 60 * 24), // 24 hours
			},
		},
		{
			name:       "follow_events",
			routingKey: "event.follow.*",
			args: amqp.Table{
				"x-dead-letter-exchange":    "timeline_events_dlx",
				"x-dead-letter-routing-key": "dead.follow",
				"x-message-ttl":             int32(1000 * 60 * 60 * 24), // 24 hours
			},
		},
		{
			name:       "notification_events",
			routingKey: "event.notification.*",
			args: amqp.Table{
				"x-dead-letter-exchange":    "timeline_events_dlx",
				"x-dead-letter-routing-key": "dead.notification",
				"x-message-ttl":             int32(1000 * 60 * 60 * 24), // 24 hours
			},
		},
		{
			name:       "dead_letter_queue",
			routingKey: "dead.*",
			args:       nil,
		},
	}

	for _, q := range queues {
		_, err = r.channel.QueueDeclare(
			q.name, // name
			true,   // durable
			false,  // delete when unused
			false,  // exclusive
			false,  // no-wait
			q.args, // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", q.name, err)
		}

		// Bind queue to the appropriate exchange
		exchange := "timeline_events"
		if q.name == "dead_letter_queue" {
			exchange = "timeline_events_dlx"
		}

		err = r.channel.QueueBind(
			q.name,       // queue name
			q.routingKey, // routing key
			exchange,     // exchange
			false,        // no-wait
			nil,          // arguments
		)
		if err != nil {
			return fmt.Errorf("failed to bind queue %s: %w", q.name, err)
		}
	}

	return nil
}

// PublishMessage publishes a message to the exchange
func (r *RabbitMQ) PublishMessage(routingKey string, message Message) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Set message timestamp if not already set
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}

	// Set version if not already set
	if message.Version == "" {
		message.Version = "1.0"
	}

	// Marshal the message to JSON
	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Publish the message
	err = r.channel.PublishWithContext(
		ctx,
		"timeline_events", // exchange
		routingKey,        // routing key
		false,             // mandatory
		false,             // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
			Headers: amqp.Table{
				"x-retry-count": int32(0),
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// ConsumeMessages consumes messages from a queue
func (r *RabbitMQ) ConsumeMessages(queueName string, handler func(Message) error) error {
	// Set QoS for fair dispatch
	err := r.channel.Qos(
		10,    // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Start consuming
	msgs, err := r.channel.Consume(
		queueName, // queue
		"",        // consumer
		false,     // auto-ack
		false,     // exclusive
		false,     // no-local
		false,     // no-wait
		nil,       // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	go func() {
		for d := range msgs {
			var message Message
			if err := json.Unmarshal(d.Body, &message); err != nil {
				log.Printf("Error unmarshaling message: %v", err)
				d.Nack(false, false) // Don't requeue malformed messages
				continue
			}

			// Process the message
			if err := handler(message); err != nil {
				log.Printf("Error processing message: %v", err)

				// Get retry count
				retryCount := int32(0)
				if count, ok := d.Headers["x-retry-count"].(int32); ok {
					retryCount = count
				}

				// Implement exponential backoff for retries
				if retryCount < 5 {
					// Increment retry count
					retryCount++

					// Calculate delay with exponential backoff
					delay := time.Duration(1<<retryCount) * time.Second

					// Republish with delay
					time.Sleep(delay)

					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()

					err = r.channel.PublishWithContext(
						ctx,
						"timeline_events", // exchange
						d.RoutingKey,      // routing key
						false,             // mandatory
						false,             // immediate
						amqp.Publishing{
							ContentType:  "application/json",
							DeliveryMode: amqp.Persistent,
							Timestamp:    time.Now(),
							Body:         d.Body,
							Headers: amqp.Table{
								"x-retry-count": retryCount,
							},
						},
					)
					if err != nil {
						log.Printf("Error republishing message: %v", err)
					}

					d.Ack(false) // Acknowledge the original message
				} else {
					// Max retries reached, send to dead letter queue
					d.Nack(false, false)
				}
				continue
			}

			// Acknowledge successful processing
			d.Ack(false)
		}
	}()

	return nil
}

// Close closes the connection to RabbitMQ
func (r *RabbitMQ) Close() error {
	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}
