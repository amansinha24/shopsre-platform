package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/amansinha24/shopsre-platform/worker/internal/simulator"
	"github.com/aws/aws-xray-sdk-go/xray"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics for Worker service
var (
	simulateRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_simulate_requests_total",
			Help: "Total number of simulate requests",
		},
		[]string{"action", "status"},
	)

	jobsProcessedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "worker_jobs_processed_total",
			Help: "Total number of background jobs processed",
		},
		[]string{"status"},
	)

	workerRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "worker_request_duration_seconds",
			Help:    "Duration of worker HTTP requests",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 2.0, 5.0},
		},
		[]string{"endpoint", "status"},
	)
)

// WorkerHandler holds everything the handler needs
type WorkerHandler struct {
	simulator *simulator.MemorySimulator
}

// NewWorkerHandler creates a new WorkerHandler
func NewWorkerHandler(sim *simulator.MemorySimulator) *WorkerHandler {
	return &WorkerHandler{
		simulator: sim,
	}
}

// StartSimulation handles POST /api/worker/simulate/start
// Button 4 on the frontend calls this
// This starts the intentional memory leak
func (h *WorkerHandler) StartSimulation(c *gin.Context) {
	start := time.Now()

	// Start X-Ray subsegment
	_, seg := xray.BeginSubsegment(c.Request.Context(), "worker-simulate-start")
	defer seg.Close(nil)

	log.Printf(`{`+
		`"service":"worker",`+
		`"level":"warn",`+
		`"message":"Button 4 clicked - starting memory simulation",`+
		`"client_ip":"%s",`+
		`"warning":"This will cause OOM kill"`+
		`}`,
		c.ClientIP(),
	)

	// Start the memory leak
	if err := h.simulator.Start(context.Background()); err != nil {
		duration := time.Since(start).Seconds()
		workerRequestDuration.WithLabelValues("simulate-start", "error").Observe(duration)
		simulateRequestsTotal.WithLabelValues("start", "failure").Inc()

		c.JSON(http.StatusConflict, gin.H{
			"error":   "simulation_already_running",
			"message": err.Error(),
		})
		return
	}

	duration := time.Since(start).Seconds()
	workerRequestDuration.WithLabelValues("simulate-start", "success").Observe(duration)
	simulateRequestsTotal.WithLabelValues("start", "success").Inc()

	c.JSON(http.StatusOK, gin.H{
		"status":  "started",
		"message": "Memory leak simulation started — watch Grafana for memory spike",
		"warning": "Pod will be OOM killed in approximately 5 seconds",
		"grafana": "Check worker_simulator_memory_bytes metric",
		"xray":    "Check X-Ray for failed traces",
	})
}

// StopSimulation handles POST /api/worker/simulate/stop
// Stops the memory leak and releases memory
func (h *WorkerHandler) StopSimulation(c *gin.Context) {
	start := time.Now()

	_, seg := xray.BeginSubsegment(c.Request.Context(), "worker-simulate-stop")
	defer seg.Close(nil)

	if err := h.simulator.Stop(); err != nil {
		duration := time.Since(start).Seconds()
		workerRequestDuration.WithLabelValues("simulate-stop", "error").Observe(duration)
		simulateRequestsTotal.WithLabelValues("stop", "failure").Inc()

		c.JSON(http.StatusConflict, gin.H{
			"error":   "simulation_not_running",
			"message": err.Error(),
		})
		return
	}

	duration := time.Since(start).Seconds()
	workerRequestDuration.WithLabelValues("simulate-stop", "success").Observe(duration)
	simulateRequestsTotal.WithLabelValues("stop", "success").Inc()

	c.JSON(http.StatusOK, gin.H{
		"status":  "stopped",
		"message": "Memory simulation stopped and memory released",
	})
}

// SimulationStatus handles GET /api/worker/simulate/status
// Frontend polls this to show current memory usage
func (h *WorkerHandler) SimulationStatus(c *gin.Context) {
	_, seg := xray.BeginSubsegment(c.Request.Context(), "worker-simulate-status")
	defer seg.Close(nil)

	status := h.simulator.Status()

	c.JSON(http.StatusOK, gin.H{
		"is_running":       status.IsRunning,
		"allocated_mb":     status.AllocatedMB,
		"process_alloc_mb": status.ProcessAllocMB,
		"sys_mb":           status.SysMB,
		"message":          getStatusMessage(status),
	})
}

// Healthz handles GET /healthz
func (h *WorkerHandler) Healthz(c *gin.Context) {
	status := h.simulator.Status()

	c.JSON(http.StatusOK, gin.H{
		"status":     "healthy",
		"service":    "worker",
		"simulation": fmt.Sprintf("running=%v allocated_mb=%d", status.IsRunning, status.AllocatedMB),
	})
}

// getStatusMessage returns a human readable status message
func getStatusMessage(status simulator.SimulatorStatus) string {
	if !status.IsRunning {
		return "Simulation is not running"
	}
	return fmt.Sprintf(
		"Simulation running — allocated %dMB of %dMB process memory",
		status.AllocatedMB,
		status.ProcessAllocMB,
	)
}