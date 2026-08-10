package simulator

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics for the memory simulator
var (
	simulatorMemoryBytes = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "worker_simulator_memory_bytes",
			Help: "Memory allocated by the simulator in bytes",
		},
	)

	simulatorRunning = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "worker_simulator_running",
			Help: "1 if simulator is running, 0 if not",
		},
	)

	simulatorIterations = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "worker_simulator_iterations_total",
			Help: "Total number of memory allocation iterations",
		},
	)

	processMemoryBytes = promauto.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "worker_process_memory_bytes",
			Help: "Current process memory usage in bytes — watched by Prometheus alert",
		},
		func() float64 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			return float64(m.Alloc)
		},
	)
)

// MemorySimulator simulates a memory leak
// This is what Button 4 triggers
type MemorySimulator struct {
	mu            sync.Mutex
	isRunning     bool
	allocatedData [][]byte // holds allocated memory — never released
	cancelFunc    context.CancelFunc
	mbPerSecond   int
}

// NewMemorySimulator creates a new simulator
func NewMemorySimulator(mbPerSecond int) *MemorySimulator {
	return &MemorySimulator{
		mbPerSecond: mbPerSecond,
	}
}

// Start begins the memory leak simulation
// Returns error if simulation is already running
func (s *MemorySimulator) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return fmt.Errorf("simulation already running")
	}

	// Create cancellable context
	simCtx, cancel := context.WithCancel(ctx)
	s.cancelFunc = cancel
	s.isRunning = true
	s.allocatedData = make([][]byte, 0)

	simulatorRunning.Set(1)

	log.Printf(`{`+
		`"service":"worker",`+
		`"level":"warn",`+
		`"message":"MEMORY LEAK SIMULATION STARTED",`+
		`"mb_per_second":%d,`+
		`"warning":"This will trigger OOM kill"`+
		`}`,
		s.mbPerSecond,
	)

	// Run simulation in background goroutine
	go s.run(simCtx)

	return nil
}

// run is the actual memory leak loop
// It allocates mbPerSecond MB every second and never releases it
func (s *MemorySimulator) run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Simulation was stopped
			log.Printf(`{"service":"worker","message":"Memory simulation stopped"}`)
			simulatorRunning.Set(0)
			return

		case <-ticker.C:
			// Allocate mbPerSecond MB of memory
			// This memory is NEVER released — that is the leak
			chunk := make([]byte, s.mbPerSecond*1024*1024)

			// Write to the memory so Go does not optimize it away
			for i := range chunk {
				chunk[i] = byte(i % 256)
			}

			// Store reference so garbage collector cannot free it
			s.mu.Lock()
			s.allocatedData = append(s.allocatedData, chunk)
			totalAllocated := len(s.allocatedData) * s.mbPerSecond
			s.mu.Unlock()

			simulatorIterations.Inc()

			// Update Prometheus gauge with current allocated bytes
			simulatorMemoryBytes.Set(float64(totalAllocated * 1024 * 1024))

			// Read actual process memory from Go runtime
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)

			// This log line is what you find in CloudWatch
			// right before the OOM kill
			log.Printf(`{`+
				`"service":"worker",`+
				`"level":"warn",`+
				`"message":"Memory growing",`+
				`"allocated_mb":%d,`+
				`"process_alloc_mb":%d,`+
				`"sys_mb":%d,`+
				`"warning":"OOM kill approaching"`+
				`}`,
				totalAllocated,
				memStats.Alloc/1024/1024,
				memStats.Sys/1024/1024,
			)
		}
	}
}

// Stop stops the simulation and releases memory
func (s *MemorySimulator) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return fmt.Errorf("simulation is not running")
	}

	// Cancel the context — stops the run loop
	s.cancelFunc()

	// Release all allocated memory
	s.allocatedData = nil
	s.isRunning = false

	// Force garbage collection to release memory immediately
	runtime.GC()

	simulatorRunning.Set(0)
	simulatorMemoryBytes.Set(0)

	log.Printf(`{`+
		`"service":"worker",`+
		`"level":"info",`+
		`"message":"Memory simulation stopped and memory released"`+
		`}`)

	return nil
}

// Status returns current simulation status
func (s *MemorySimulator) Status() SimulatorStatus {
	s.mu.Lock()
	defer s.mu.Unlock()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	allocatedMB := len(s.allocatedData) * s.mbPerSecond

	return SimulatorStatus{
		IsRunning:      s.isRunning,
		AllocatedMB:    allocatedMB,
		ProcessAllocMB: int(memStats.Alloc / 1024 / 1024),
		SysMB:          int(memStats.Sys / 1024 / 1024),
	}
}

// SimulatorStatus holds current simulator state
type SimulatorStatus struct {
	IsRunning      bool `json:"is_running"`
	AllocatedMB    int  `json:"allocated_mb"`
	ProcessAllocMB int  `json:"process_alloc_mb"`
	SysMB          int  `json:"sys_mb"`
}