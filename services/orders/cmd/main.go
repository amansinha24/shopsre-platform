package main
// ShopSRE Orders Service v1.0.0
import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amansinha24/shopsre-platform/orders/config"
	"github.com/amansinha24/shopsre-platform/orders/internal/handler"
	"github.com/amansinha24/shopsre-platform/orders/internal/publisher"
	"github.com/amansinha24/shopsre-platform/orders/internal/repository"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

func main() {
	// ── Step 1: Load config ────────────────────────────────────────
	cfg := config.Load()
	log.Printf("Starting Orders Service on port %s (env: %s)", cfg.Port, cfg.Env)

	// ── Step 2: Setup AWS X-Ray ────────────────────────────────────
	err := xray.Configure(xray.Config{
		DaemonAddr:     "127.0.0.1:2000",
		ServiceVersion: "1.0.0",
	})
	if err != nil {
		log.Printf("WARNING: X-Ray configuration failed: %v", err)
	}

	// ── Step 3: Connect to PostgreSQL ──────────────────────────────
	log.Printf("Connecting to PostgreSQL at %s:%s...", cfg.PostgresHost, cfg.PostgresPort)
	db, err := repository.NewDBConnection(
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresDB,
	)
	if err != nil {
		log.Fatalf("FATAL: Cannot connect to PostgreSQL: %v", err)
	}
	defer db.Close()
	log.Printf("PostgreSQL connected successfully")

	// ── Step 4: Connect to Redis ───────────────────────────────────
	log.Printf("Connecting to Redis at %s:%s...", cfg.RedisHost, cfg.RedisPort)
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       0,
	})

	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("WARNING: Cannot connect to Redis: %v", err)
		log.Printf("Cache will be disabled — service continues")
	} else {
		log.Printf("Redis connected successfully")
	}
	defer redisClient.Close()

	// ── Step 5: Create database tables ────────────────────────────
	orderRepo := repository.NewOrderRepository(db, redisClient)
	if err := orderRepo.CreateTable(); err != nil {
		log.Fatalf("FATAL: Cannot create database tables: %v", err)
	}
	log.Printf("Database tables ready")

	// ── Step 6: Connect to RabbitMQ ───────────────────────────────
	log.Printf("Connecting to RabbitMQ...")
	rabbitPublisher, err := publisher.NewRabbitMQPublisher(cfg.RabbitMQURL)
	if err != nil {
		// RabbitMQ failure is not fatal at startup
		// Service can still create orders — just won't publish events
		log.Printf("WARNING: Cannot connect to RabbitMQ: %v", err)
		log.Printf("Events will not be published — service continues")
		rabbitPublisher = nil
	} else {
		defer rabbitPublisher.Close()
		log.Printf("RabbitMQ connected successfully")
	}

	// ── Step 7: Create handler ─────────────────────────────────────
	orderHandler := handler.NewOrderHandler(
		orderRepo,
		rabbitPublisher,
		cfg.AuthServiceURL,
	)

	// ── Step 8: Setup HTTP router ──────────────────────────────────
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggingMiddleware())
	router.Use(xrayMiddleware())

	// ── Step 9: Register routes ────────────────────────────────────
	router.GET("/healthz", orderHandler.Healthz)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := router.Group("/api/orders")
	{
		// POST /api/orders → Button 2 — Place Order
		api.POST("", orderHandler.CreateOrder)

		// GET /api/orders → Button 3 — Get My Orders
		api.GET("", orderHandler.GetOrders)
	}

	// ── Step 10: Start HTTP server ─────────────────────────────────
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Orders Service listening on :%s", cfg.Port)
		log.Printf("Routes:")
		log.Printf("  GET  /healthz      → health check")
		log.Printf("  GET  /metrics      → Prometheus metrics")
		log.Printf("  POST /api/orders   → create order (Button 2)")
		log.Printf("  GET  /api/orders   → get orders  (Button 3)")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("FATAL: Server failed to start: %v", err)
		}
	}()

	// ── Step 11: Graceful shutdown ─────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("Shutting down Orders Service...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("FATAL: Server forced to shutdown: %v", err)
	}

	log.Printf("Orders Service stopped cleanly")
}

// loggingMiddleware logs every request as structured JSON for CloudWatch
func loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		status := c.Writer.Status()

		log.Printf(`{`+
			`"timestamp":"%s",`+
			`"service":"orders",`+
			`"method":"%s",`+
			`"path":"%s",`+
			`"status":%d,`+
			`"duration_ms":%d,`+
			`"client_ip":"%s"`+
			`}`,
			time.Now().UTC().Format(time.RFC3339),
			c.Request.Method,
			c.Request.URL.Path,
			status,
			duration.Milliseconds(),
			c.ClientIP(),
		)
	}
}

// xrayMiddleware wraps every request in an X-Ray trace segment
func xrayMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/healthz" ||
			c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		_, seg := xray.BeginSegment(
			context.Background(),
			"orders-service",
		)
		defer seg.Close(nil)

		seg.AddAnnotation("method", c.Request.Method)
		seg.AddAnnotation("path", c.Request.URL.Path)

		c.Request = c.Request.WithContext(
			context.WithValue(c.Request.Context(), "xray_seg", seg),
		)

		c.Next()

		seg.AddAnnotation("status", fmt.Sprintf("%d", c.Writer.Status()))
	}
}