package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/amansinha24/shopsre-platform/orders/internal/domain"
	"github.com/amansinha24/shopsre-platform/orders/internal/publisher"
	"github.com/amansinha24/shopsre-platform/orders/internal/repository"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics for Orders service
var (
	ordersCreatedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orders_created_total",
			Help: "Total number of orders created",
		},
		[]string{"status"},
	)

	ordersFetchedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orders_fetched_total",
			Help: "Total number of order list requests",
		},
		[]string{"source"}, // "cache" or "database"
	)

	orderRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "orders_request_duration_seconds",
			Help:    "Duration of orders requests in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.3, 0.5, 1.0, 2.0},
		},
		[]string{"endpoint", "status"},
	)

	cacheHitTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "orders_cache_hits_total",
			Help: "Total number of Redis cache hits",
		},
	)

	cacheMissTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "orders_cache_misses_total",
			Help: "Total number of Redis cache misses",
		},
	)

	rabbitmqPublishTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "orders_rabbitmq_publish_total",
			Help: "Total number of RabbitMQ publish attempts",
		},
		[]string{"status"},
	)
)

// OrderHandler holds everything the handler needs
type OrderHandler struct {
	orderRepo  *repository.OrderRepository
	publisher  *publisher.RabbitMQPublisher
	authSvcURL string
}

// NewOrderHandler creates a new OrderHandler
func NewOrderHandler(
	orderRepo *repository.OrderRepository,
	publisher *publisher.RabbitMQPublisher,
	authSvcURL string,
) *OrderHandler {
	return &OrderHandler{
		orderRepo:  orderRepo,
		publisher:  publisher,
		authSvcURL: authSvcURL,
	}
}

// CreateOrder handles POST /api/orders
// Button 2 on the frontend calls this
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	start := time.Now()

	// Start X-Ray subsegment
	ctx, seg := xray.BeginSubsegment(c.Request.Context(), "orders-create")
	if seg != nil {
    	defer seg.Close(nil)
	}

	// Step 1: Validate JWT token by calling Auth Service
	userID, err := h.validateToken(ctx, c.GetHeader("Authorization"))
	if err != nil {
		duration := time.Since(start).Seconds()
		orderRequestDuration.WithLabelValues("create", "error").Observe(duration)
		ordersCreatedTotal.WithLabelValues("failure").Inc()

		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Error:   "unauthorized",
			Message: "Invalid or missing token",
		})
		return
	}

	seg.AddAnnotation("user_id", fmt.Sprintf("%d", userID))

	// Step 2: Parse request body
	var req domain.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		duration := time.Since(start).Seconds()
		orderRequestDuration.WithLabelValues("create", "error").Observe(duration)
		ordersCreatedTotal.WithLabelValues("failure").Inc()

		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
		return
	}

	seg.AddAnnotation("item_name", req.ItemName)

	// Step 3: Create order object
	order := &domain.Order{
		UserID:   userID,
		ItemName: req.ItemName,
		Quantity: req.Quantity,
		Price:    req.Price,
		Status:   "pending",
	}

	// Step 4: Save to PostgreSQL
	_, dbSeg := xray.BeginSubsegment(ctx, "postgresql-write")
	if dbSeg != nil {
    	defer dbSeg.Close(nil)
	}
	if err := h.orderRepo.Create(order); err != nil {
		dbSeg.Close(err)
		duration := time.Since(start).Seconds()
		orderRequestDuration.WithLabelValues("create", "error").Observe(duration)
		ordersCreatedTotal.WithLabelValues("failure").Inc()

		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Error:   "server_error",
			Message: "Failed to create order",
		})
		return
	}
	dbSeg.Close(nil)

	// Step 5: Publish event to RabbitMQ
	// This is async — Orders service does not wait for
	// Notifications or Worker to finish processing
	_, mqSeg := xray.BeginSubsegment(ctx, "rabbitmq-publish")
	if mqSeg != nil {
    	defer mqSeg.Close(nil)
	}
	if err := h.publisher.PublishOrderPlaced(ctx, order); err != nil {
		mqSeg.Close(err)
		// RabbitMQ failure is not critical
		// Order is already saved — just log the warning
		log.Printf(`{"service":"orders","level":"warn","message":"failed to publish event","order_id":%d,"error":"%v"}`,
			order.ID, err)
		rabbitmqPublishTotal.WithLabelValues("failure").Inc()
	} else {
		mqSeg.Close(nil)
		rabbitmqPublishTotal.WithLabelValues("success").Inc()
	}

	// Step 6: Record metrics and respond
	duration := time.Since(start).Seconds()
	orderRequestDuration.WithLabelValues("create", "success").Observe(duration)
	ordersCreatedTotal.WithLabelValues("success").Inc()

	log.Printf(`{"service":"orders","level":"info","message":"order created","order_id":%d,"user_id":%d,"item":"%s","duration_ms":%d}`,
		order.ID, order.UserID, order.ItemName, time.Since(start).Milliseconds())

	c.JSON(http.StatusCreated, domain.CreateOrderResponse{
		OrderID:  order.ID,
		UserID:   order.UserID,
		ItemName: order.ItemName,
		Quantity: order.Quantity,
		Price:    order.Price,
		Status:   order.Status,
		Message:  "Order placed successfully",
	})
}

// GetOrders handles GET /api/orders
// Button 3 on the frontend calls this
func (h *OrderHandler) GetOrders(c *gin.Context) {
	start := time.Now()

	// Start X-Ray subsegment
	ctx, seg := xray.BeginSubsegment(c.Request.Context(), "orders-get")
	if seg != nil {
    	defer seg.Close(nil)
	}

	// Validate JWT token
	userID, err := h.validateToken(ctx, c.GetHeader("Authorization"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Error:   "unauthorized",
			Message: "Invalid or missing token",
		})
		return
	}

	seg.AddAnnotation("user_id", fmt.Sprintf("%d", userID))

	// Fetch orders — checks Redis cache first
	orders, source, err := h.orderRepo.GetByUserID(ctx, userID)
	if err != nil {
		duration := time.Since(start).Seconds()
		orderRequestDuration.WithLabelValues("get", "error").Observe(duration)

		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Error:   "server_error",
			Message: "Failed to fetch orders",
		})
		return
	}

	// Track cache hit vs miss in Prometheus
	if source == "cache" {
		cacheHitTotal.Inc()
		ordersFetchedTotal.WithLabelValues("cache").Inc()
	} else {
		cacheMissTotal.Inc()
		ordersFetchedTotal.WithLabelValues("database").Inc()
	}

	seg.AddAnnotation("source", source)
	seg.AddAnnotation("count", fmt.Sprintf("%d", len(orders)))

	duration := time.Since(start).Seconds()
	orderRequestDuration.WithLabelValues("get", "success").Observe(duration)

	log.Printf(`{"service":"orders","level":"info","message":"orders fetched","user_id":%d,"count":%d,"source":"%s","duration_ms":%d}`,
		userID, len(orders), source, time.Since(start).Milliseconds())

	c.JSON(http.StatusOK, domain.GetOrdersResponse{
		Orders: orders,
		Count:  len(orders),
		Source: source,
	})
}

// Healthz handles GET /healthz
func (h *OrderHandler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "orders",
	})
}

// validateToken calls the Auth Service to verify the JWT token
// This creates a service-to-service call visible in X-Ray
func (h *OrderHandler) validateToken(ctx context.Context, authHeader string) (int64, error) {
	if authHeader == "" {
		return 0, fmt.Errorf("missing authorization header")
	}

	// Start X-Ray subsegment for this service-to-service call
	_, seg := xray.BeginSubsegment(ctx, "auth-service-validate")
	if seg != nil {
    	defer seg.Close(nil)
	}

	// Call Auth Service /api/auth/validate
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("%s/api/auth/validate", h.authSvcURL),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create request: %w", err)
	}

	// Forward the Authorization header
	req.Header.Set("Authorization", authHeader)

	// Make the HTTP call
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		seg.Close(err)
		return 0, fmt.Errorf("auth service call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("token validation failed: status %d", resp.StatusCode)
	}

	// Parse response to get user ID
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to read auth response: %w", err)
	}

	var authResp struct {
		Valid  bool    `json:"valid"`
		UserID float64 `json:"user_id"`
	}
	if err := json.Unmarshal(body, &authResp); err != nil {
		return 0, fmt.Errorf("failed to parse auth response: %w", err)
	}

	return int64(authResp.UserID), nil
}

// extractUserID is a helper to parse user ID from string
func extractUserID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}