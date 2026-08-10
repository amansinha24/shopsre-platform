package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amansinha24/shopsre-platform/worker/config"
	"github.com/amansinha24/shopsre-platform/worker/internal/handler"
	"github.com/amansinha24/shopsre-platform/worker/internal/jobs"
	"github.com/amansinha24/shopsre-platform/worker/internal/simulator"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// ── Step 1: Load config ────────────────────────────────────────
	cfg := config.Load()
	log.Printf("Starting Worker Service on port %s (env: %s)", cfg.Port, cfg.Env)

	// ── Step 2: Setup AWS X-Ray ────────────────────────────────────
	err := xray.Configure(xray.Config{
		DaemonAddr:     "127.0.0.1:2000",
		ServiceVersion: "1.0.0",
	})
	if err != nil {
		log.Printf("WARNING: X-Ray configuration failed: %v", err)
	}

	// ── Step 3: Connect to PostgreSQL ──────────────────────────────
	log.Printf("Connecting to PostgreSQL...")
	db, err := jobs.NewDBConnection(
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresDB,
	)
	if err != nil {
		log.Printf("WARNING: Cannot connect to PostgreSQL: %v", err)
		log.Printf("Job processing will be limited — service continues")
		db = nil
	} else {
		defer db.Close()
		log.Printf("PostgreSQL connected successfully")
	}

	// ── Step 4: Setup Memory Simulator ────────────────────────────
	// This is what Button 4 triggers
	memSimulator := simulator.NewMemorySimulator(cfg.SimulatorMBPerSecond)
	log.Printf("Memory simulator ready — %dMB per second", cfg.SimulatorMBPerSecond)

	// ── Step 5: Setup Job Processor ───────────────────────────────
	// Connects to RabbitMQ and processes order events
	jobProcessor := jobs.NewJobProcessor(db, cfg.RabbitMQURL)

	// Start job processor in background
	// It runs independently of the HTTP server
	processorCtx, cancelProcessor := context.WithCancel(context.Background())
	go func() {
		log.Printf("Starting RabbitMQ job processor...")
		if err := jobProcessor.Start(processorCtx); err != nil {
			log.Printf("WARNING: Job processor stopped: %v", err)
		}
	}()

	// ── Step 6: Create handler ─────────────────────────────────────
	workerHandler := handler.NewWorkerHandler(memSimulator)

	// ── Step 7: Setup HTTP router ──────────────────────────────────
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggingMiddleware())
	router.Use(xrayMiddleware())

	// ── Step 8: Register routes ────────────────────────────────────
	router.GET("/healthz", workerHandler.Healthz)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	api := router.Group("/api/worker")
	{
		// Button 4 — Simulate Load routes
		api.POST("/simulate/start", workerHandler.StartSimulation)
		api.POST("/simulate/stop", workerHandler.StopSimulation)
		api.GET("/simulate/status", workerHandler.SimulationStatus)
	}

	// ── Step 9: Start HTTP server ──────────────────────────────────
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Worker Service listening on :%s", cfg.Port)
		log.Printf("Routes:")
		log.Printf("  GET  /healthz                    → health check")
		log.Printf("  GET  /metrics                    → Prometheus metrics")
		log.Printf("  POST /api/worker/simulate/start  → start OOM simulation (Button 4)")
		log.Printf("  POST /api/worker/simulate/stop   → stop simulation")
		log.Printf("  GET  /api/worker/simulate/status → current memory status")

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("FATAL: Server failed to start: %v", err)
		}
	}()

	// ── Step 10: Graceful shutdown ─────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("Shutting down Worker Service...")

	// Stop job processor
	cancelProcessor()
	jobProcessor.Close()

	// Stop memory simulator if running
	if status := memSimulator.Status(); status.IsRunning {
		memSimulator.Stop()
	}

	// Graceful HTTP shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("FATAL: Server forced to shutdown: %v", err)
	}

	log.Printf("Worker Service stopped cleanly")
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
			`"service":"worker",`+
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
			"worker-service",
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