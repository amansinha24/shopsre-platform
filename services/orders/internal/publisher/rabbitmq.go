package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/amansinha24/shopsre-platform/orders/internal/domain"
	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQPublisher publishes events to RabbitMQ
type RabbitMQPublisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// NewRabbitMQPublisher creates a new RabbitMQ connection and channel
func NewRabbitMQPublisher(url string) (*RabbitMQPublisher, error) {
	// Connect to RabbitMQ
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Open a channel
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	publisher := &RabbitMQPublisher{
		conn:    conn,
		channel: channel,
	}

	// Setup the exchange and queues
	if err := publisher.setup(); err != nil {
		publisher.Close()
		return nil, err
	}

	log.Printf("RabbitMQ connected successfully")
	return publisher, nil
}

// setup creates the exchange and queues
// This is idempotent — safe to run every time service starts
func (p *RabbitMQPublisher) setup() error {
	// Declare a topic exchange named "orders"
	// Topic exchange routes messages based on routing key patterns
	err := p.channel.ExchangeDeclare(
		"orders",  // exchange name
		"topic",   // exchange type
		true,      // durable — survives RabbitMQ restart
		false,     // auto-delete
		false,     // internal
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare notifications queue
	// Notifications service consumes from this queue
	_, err = p.channel.QueueDeclare(
		"notifications.orders", // queue name
		true,                   // durable — messages survive restart
		false,                  // auto-delete
		false,                  // exclusive
		false,                  // no-wait
		amqp.Table{
			// Dead letter exchange — failed messages go here
			"x-dead-letter-exchange": "orders.dlx",
			// Messages expire after 24 hours if not consumed
			"x-message-ttl": int32(86400000),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare notifications queue: %w", err)
	}

	// Declare worker queue
	// Worker service consumes from this queue
	_, err = p.channel.QueueDeclare(
		"worker.orders", // queue name
		true,            // durable
		false,           // auto-delete
		false,           // exclusive
		false,           // no-wait
		amqp.Table{
			"x-dead-letter-exchange": "orders.dlx",
			"x-message-ttl":          int32(86400000),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare worker queue: %w", err)
	}

	// Bind notifications queue to exchange
	// Routing key "order.placed" → goes to notifications queue
	err = p.channel.QueueBind(
		"notifications.orders", // queue name
		"order.placed",         // routing key
		"orders",               // exchange name
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind notifications queue: %w", err)
	}

	// Bind worker queue to exchange
	// Routing key "order.*" → all order events go to worker queue
	err = p.channel.QueueBind(
		"worker.orders", // queue name
		"order.*",       // routing key — matches order.placed, order.cancelled etc
		"orders",        // exchange name
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind worker queue: %w", err)
	}

	// Declare dead letter exchange
	// Messages that fail processing go here
	err = p.channel.ExchangeDeclare(
		"orders.dlx", // dead letter exchange name
		"fanout",     // fanout — sends to all bound queues
		true,         // durable
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare dead letter exchange: %w", err)
	}

	log.Printf("RabbitMQ exchange and queues setup complete")
	return nil
}

// PublishOrderPlaced publishes an order.placed event to RabbitMQ
// Both Notifications and Worker services will receive this
func (p *RabbitMQPublisher) PublishOrderPlaced(ctx context.Context, order *domain.Order) error {
	// Build the event
	event := domain.OrderEvent{
		EventID:   fmt.Sprintf("evt_%d_%d", order.ID, time.Now().UnixNano()),
		EventType: "order.placed",
		OrderID:   order.ID,
		UserID:    order.UserID,
		ItemName:  order.ItemName,
		Quantity:  order.Quantity,
		Price:     order.Price,
		Timestamp: time.Now().UTC(),
	}

	// Serialize event to JSON
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Publish to RabbitMQ
	err = p.channel.PublishWithContext(
		ctx,
		"orders",       // exchange name
		"order.placed", // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent, // message survives RabbitMQ restart
			Timestamp:    time.Now(),
			MessageId:    event.EventID,
			// Headers carry the X-Ray trace context
			// This links the async trace in Notifications/Worker
			// back to the original Orders trace
			Headers: amqp.Table{
				"event_type": event.EventType,
				"event_id":   event.EventID,
				"order_id":   fmt.Sprintf("%d", event.OrderID),
			},
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	log.Printf(`{"service":"orders","event":"order.placed","order_id":%d,"event_id":"%s"}`,
		order.ID, event.EventID)

	return nil
}

// Close cleans up RabbitMQ connection
func (p *RabbitMQPublisher) Close() {
	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}