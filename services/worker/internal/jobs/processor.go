package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// OrderEvent is the message we receive from RabbitMQ
// Same structure as what Orders service publishes
type OrderEvent struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	OrderID   int64     `json:"order_id"`
	UserID    int64     `json:"user_id"`
	ItemName  string    `json:"item_name"`
	Quantity  int       `json:"quantity"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}

// JobProcessor consumes order events and processes background jobs
type JobProcessor struct {
	db         *sql.DB
	conn       *amqp.Connection
	channel    *amqp.Channel
	rabbitURL  string
}

// NewJobProcessor creates a new JobProcessor
func NewJobProcessor(db *sql.DB, rabbitURL string) *JobProcessor {
	return &JobProcessor{
		db:        db,
		rabbitURL: rabbitURL,
	}
}

// Connect establishes RabbitMQ connection
func (p *JobProcessor) Connect() error {
	conn, err := amqp.Dial(p.rabbitURL)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	// Only process one message at a time
	if err := channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	p.conn = conn
	p.channel = channel

	log.Printf(`{"service":"worker","message":"Connected to RabbitMQ"}`)
	return nil
}

// Start begins consuming from the worker queue
func (p *JobProcessor) Start(ctx context.Context) error {
	if err := p.Connect(); err != nil {
		return err
	}

	// Declare the queue — same as Orders service declared
	queue, err := p.channel.QueueDeclare(
		"worker.orders",
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-dead-letter-exchange": "orders.dlx",
			"x-message-ttl":          int32(86400000),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Start consuming messages
	msgs, err := p.channel.Consume(
		queue.Name,
		"worker-consumer", // consumer name
		false,             // auto-ack false — we manually ack
		false,             // exclusive
		false,             // no-local
		false,             // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	log.Printf(`{"service":"worker","message":"Started consuming from worker.orders queue"}`)

	// Process messages in a loop
	for {
		select {
		case <-ctx.Done():
			log.Printf(`{"service":"worker","message":"Job processor stopped"}`)
			return nil

		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("RabbitMQ channel closed")
			}
			p.processMessage(msg)
		}
	}
}

// processMessage handles a single message from RabbitMQ
func (p *JobProcessor) processMessage(msg amqp.Delivery) {
	start := time.Now()

	// Parse event
	var event OrderEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		log.Printf(`{"service":"worker","level":"error","message":"Failed to parse message","error":"%v"}`, err)
		// Reject message — send to DLQ
		msg.Nack(false, false)
		return
	}

	log.Printf(`{`+
		`"service":"worker",`+
		`"level":"info",`+
		`"message":"Processing job",`+
		`"event_id":"%s",`+
		`"event_type":"%s",`+
		`"order_id":%d`+
		`}`,
		event.EventID,
		event.EventType,
		event.OrderID,
	)

	// Process based on event type
	var err error
	switch event.EventType {
	case "order.placed":
		err = p.generateOrderReport(event)
	default:
		log.Printf(`{"service":"worker","message":"Unknown event type: %s"}`, event.EventType)
	}

	if err != nil {
		log.Printf(`{`+
			`"service":"worker",`+
			`"level":"error",`+
			`"message":"Job failed",`+
			`"event_id":"%s",`+
			`"error":"%v"`+
			`}`,
			event.EventID, err,
		)
		// Requeue the message for retry
		msg.Nack(false, true)
		return
	}

	// Acknowledge message — remove from queue
	msg.Ack(false)

	log.Printf(`{`+
		`"service":"worker",`+
		`"level":"info",`+
		`"message":"Job completed",`+
		`"event_id":"%s",`+
		`"duration_ms":%d`+
		`}`,
		event.EventID,
		time.Since(start).Milliseconds(),
	)
}

// generateOrderReport simulates generating a report for an order
// In production this would create a PDF report and store in S3
func (p *JobProcessor) generateOrderReport(event OrderEvent) error {
	log.Printf(`{`+
		`"service":"worker",`+
		`"level":"info",`+
		`"message":"Generating order report",`+
		`"order_id":%d,`+
		`"item_name":"%s",`+
		`"quantity":%d,`+
		`"price":%.2f`+
		`}`,
		event.OrderID,
		event.ItemName,
		event.Quantity,
		event.Price,
	)

	// Simulate report generation taking some time
	time.Sleep(2 * time.Second)

	// In production: generate PDF, upload to S3, update DB
	// For now: just log success
	log.Printf(`{`+
		`"service":"worker",`+
		`"level":"info",`+
		`"message":"Order report generated successfully",`+
		`"order_id":%d`+
		`}`,
		event.OrderID,
	)

	return nil
}

// Close cleans up connections
func (p *JobProcessor) Close() {
	if p.channel != nil {
		p.channel.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}

// NewDBConnection creates a PostgreSQL connection
func NewDBConnection(host, port, user, password, dbname string) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
		host, port, user, password, dbname,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)

	return db, nil
}