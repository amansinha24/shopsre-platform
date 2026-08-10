package domain

import "time"

// Order represents an order in our system
type Order struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	ItemName  string    `json:"item_name"`
	Quantity  int       `json:"quantity"`
	Price     float64   `json:"price"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateOrderRequest is what the frontend sends when placing an order
type CreateOrderRequest struct {
	ItemName string  `json:"item_name" binding:"required"`
	Quantity int     `json:"quantity"  binding:"required,min=1"`
	Price    float64 `json:"price"     binding:"required,min=0"`
}

// CreateOrderResponse is what we send back after order is created
type CreateOrderResponse struct {
	OrderID  int64   `json:"order_id"`
	UserID   int64   `json:"user_id"`
	ItemName string  `json:"item_name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"`
	Status   string  `json:"status"`
	Message  string  `json:"message"`
}

// GetOrdersResponse is what we send back when listing orders
type GetOrdersResponse struct {
	Orders []Order `json:"orders"`
	Count  int     `json:"count"`
	Source string  `json:"source"` // "cache" or "database"
}

// OrderEvent is the message we publish to RabbitMQ
// Both Notifications and Worker services consume this
type OrderEvent struct {
	EventID   string    `json:"event_id"`  // unique ID for idempotency
	EventType string    `json:"event_type"` // "order.placed"
	OrderID   int64     `json:"order_id"`
	UserID    int64     `json:"user_id"`
	ItemName  string    `json:"item_name"`
	Quantity  int       `json:"quantity"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}

// ErrorResponse is what we send back when something goes wrong
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}