package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/amansinha24/shopsre-platform/orders/internal/domain"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

// OrderRepository handles all database and cache operations for orders
type OrderRepository struct {
	db          *sql.DB
	redisClient *redis.Client
}

// NewOrderRepository creates a new OrderRepository
func NewOrderRepository(db *sql.DB, redisClient *redis.Client) *OrderRepository {
	return &OrderRepository{
		db:          db,
		redisClient: redisClient,
	}
}

// CreateTable creates the orders table if it does not exist
func (r *OrderRepository) CreateTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS orders (
			id         SERIAL PRIMARY KEY,
			user_id    INTEGER NOT NULL,
			item_name  VARCHAR(255) NOT NULL,
			quantity   INTEGER NOT NULL,
			price      DECIMAL(10,2) NOT NULL,
			status     VARCHAR(50) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := r.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create orders table: %w", err)
	}
	return nil
}

// Create saves a new order to PostgreSQL
func (r *OrderRepository) Create(order *domain.Order) error {
	query := `
		INSERT INTO orders (user_id, item_name, quantity, price, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`

	err := r.db.QueryRow(
		query,
		order.UserID,
		order.ItemName,
		order.Quantity,
		order.Price,
		order.Status,
	).Scan(&order.ID, &order.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create order: %w", err)
	}

	// Invalidate cache for this user
	// Next time they fetch orders it will come fresh from DB
	cacheKey := fmt.Sprintf("orders:user:%d", order.UserID)
	r.redisClient.Del(context.Background(), cacheKey)

	return nil
}

// GetByUserID fetches all orders for a user
// Checks Redis cache first — only hits PostgreSQL on cache miss
func (r *OrderRepository) GetByUserID(ctx context.Context, userID int64) ([]domain.Order, string, error) {
	cacheKey := fmt.Sprintf("orders:user:%d", userID)

	// ── Try Redis cache first ──────────────────────────────────────
	cached, err := r.redisClient.Get(ctx, cacheKey).Result()
	if err == nil {
		// Cache hit — deserialize and return
		var orders []domain.Order
		if err := json.Unmarshal([]byte(cached), &orders); err == nil {
			return orders, "cache", nil
		}
	}

	// ── Cache miss — fetch from PostgreSQL ─────────────────────────
	query := `
		SELECT id, user_id, item_name, quantity, price, status, created_at
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, "database", fmt.Errorf("failed to fetch orders: %w", err)
	}
	defer rows.Close()

	var orders []domain.Order
	for rows.Next() {
		var order domain.Order
		err := rows.Scan(
			&order.ID,
			&order.UserID,
			&order.ItemName,
			&order.Quantity,
			&order.Price,
			&order.Status,
			&order.CreatedAt,
		)
		if err != nil {
			return nil, "database", fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, order)
	}

	// ── Store in Redis cache for next time ─────────────────────────
	// Cache expires after 5 minutes
	if len(orders) > 0 {
		data, err := json.Marshal(orders)
		if err == nil {
			r.redisClient.Set(ctx, cacheKey, data, 5*time.Minute)
		}
	}

	return orders, "database", nil
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