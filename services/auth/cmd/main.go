package main

// ShopSRE Auth Service v1.0.2
import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amansinha24/shopsre-platform/auth/config"
	"github.com/amansinha24/shopsre-platform/auth/internal/handler"
	"github.com/amansinha24/shopsre-platform/auth/internal/repository"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

func main() {
	// ── Step 1: Load config ────────────────────────────────────────
	cfg := config.Load()
	log.Printf("Starting Auth Service on port %s (env: %s)", cfg.Port, cfg.Env)

	// ── Step 2: Setup AWS X-Ray ────────────────────────────────────
	// X-Ray captures every request as a trace
	// You will see these traces in AWS X-Ray console
	err := xray.Configure(xray.Config{
		DaemonAddr:     "127.0.0.1:2000", // X-Ray daemon address
		ServiceVersion: "1.0.0",
	})
	if err != nil {
		log.Printf("WARNING: X-Ray configuration failed: %v", err)
		log.Printf("Traces will not be sent to X-Ray")
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

	// ── Step 4: Create database tables ────────────────────────────
	// This runs CREATE TABLE IF NOT EXISTS
	// Safe to run every time the service starts
	userRepo := repository.NewUserRepository(db)
	if err := userRepo.CreateTable(); err != nil {
		log.Fatalf("FATAL: Cannot create database tables: %v", err)
	}
	log.Printf("Database tables ready")

	// ── Step 5: Connect to Redis ───────────────────────────────────
	log.Printf("Connecting to Redis at %s:%s...", cfg.RedisHost, cfg.RedisPort)
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.RedisHost, cfg.RedisPort),
		Password: cfg.RedisPassword,
		DB:       0,
	})

	// Test Redis connection
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		// Redis failure is not fatal — service can run without it
		log.Printf("WARNING: Cannot connect to Redis: %v", err)
		log.Printf("Sessions will not be cached — service continues")
	} else {
		log.Printf("Redis connected successfully")
	}
	defer redisClient.Close()

	// ── Step 6: Create handler ─────────────────────────────────────
	authHandler := handler.NewAuthHandler(
		userRepo,
		redisClient,
		cfg.JWTSecret,
	)

	// ── Step 7: Setup HTTP router ──────────────────────────────────
	// Set Gin to release mode in production
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	// Middleware
	router.Use(gin.Recovery()) // recovers from panics — never crashes
	router.Use(loggingMiddleware()) // structured JSON logging
	router.Use(xrayMiddleware())    // X-Ray tracing on every request

	// ── Step 8: Register routes ────────────────────────────────────

	// Health check — Kubernetes calls this every 10 seconds
	// If this returns anything other than 200, pod is restarted
	router.GET("/healthz", authHandler.Healthz)

	// Metrics — Prometheus scrapes this every 15 seconds
	// This is how your Grafana dashboards get data
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Auth API routes
	api := router.Group("/api/auth")
	{
		// POST /api/auth/register → create new account
		api.POST("/register", authHandler.Register)

		// POST /api/auth/login → login and get JWT token
		api.POST("/login", authHandler.Login)

		// GET /api/auth/validate → validate JWT token
		// Called by Orders service to verify user is logged in
		api.GET("/validate", authHandler.ValidateToken)
	}

	// ── Step 9: Start HTTP server ──────────────────────────────────
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: router,
		// Timeouts prevent slow clients from holding connections
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine so it doesn't block
	go func() {
		log.Printf("Auth Service listening on :%s", cfg.Port)
		log.Printf("Routes:")
		log.Printf("  GET  /healthz          → health check")
		log.Printf("  GET  /metrics          → Prometheus metrics")
		log.Printf("  POST /api/auth/register → register new user")
		log.Printf("  POST /api/auth/login    → login")
		log.Printf("  GET  /api/auth/validate → validate JWT token")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("FATAL: Server failed to start: %v", err)
		}
	}()

	// ── Step 10: Graceful shutdown ─────────────────────────────────
	// Wait for Ctrl+C or kill signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("Shutting down Auth Service...")

	// Give existing requests 30 seconds to complete
	// before forcefully shutting down
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("FATAL: Server forced to shutdown: %v", err)
	}

	log.Printf("Auth Service stopped cleanly")
}

// loggingMiddleware logs every request as structured JSON
// These logs go to CloudWatch Logs on EKS
func loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Log after request completes
		duration := time.Since(start)
		status := c.Writer.Status()

		// Structured JSON log — CloudWatch can filter and search these
		log.Printf(`{`+
			`"timestamp":"%s",`+
			`"service":"auth",`+
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
// This creates the top level trace you see in X-Ray console
func xrayMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip tracing for health checks and metrics
		// We don't want thousands of /healthz traces cluttering X-Ray
		if c.Request.URL.Path == "/healthz" ||
			c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		// Begin X-Ray segment for this request
		_, seg := xray.BeginSegment(
			context.Background(),
			"auth-service",
		)
		defer seg.Close(nil)

		// Add HTTP info to the trace
		seg.AddAnnotation("method", c.Request.Method)
		seg.AddAnnotation("path", c.Request.URL.Path)

		// Store segment in context so handlers can add subsegments
		c.Request = c.Request.WithContext(
			context.WithValue(c.Request.Context(), "xray_seg", seg),
		)

		c.Next()

		// Record response status in trace
		seg.AddAnnotation("status", fmt.Sprintf("%d", c.Writer.Status()))
	}
}