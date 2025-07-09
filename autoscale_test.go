package tspool

import (
	"testing"
	"time"
)

// Test intelligent auto-scaling functionality
func TestTaskPool_AutoScale(t *testing.T) {
	// Create custom configuration with shorter time intervals for testing
	config := &AutoScaleConfig{
		ScaleUpThreshold: 1.5,
		ScaleUpCooldown:  1 * time.Second,
		ScaleUpStep:      1,

		ScaleDownThreshold: 0.3,
		ScaleDownCooldown:  2 * time.Second,
		ScaleDownStep:      1,
		IdleTimeout:        3 * time.Second,

		MonitorInterval: 500 * time.Millisecond,
		HistorySize:     5,
		SmoothingFactor: 0.3,

		EnableAutoScale: true,
	}

	pool := NewTaskPool(
		WithMinWorkers(1),
		WithMaxWorkers(5),
		WithQueueSize(20),
		WithAutoScaleConfig(config),
	)
	pool.Start()
	defer pool.Shutdown()

	// Wait for monitoring to start
	time.Sleep(600 * time.Millisecond)

	// Test auto scale-up
	t.Log("Testing auto scale-up...")

	// Quickly submit tasks to trigger scale-up
	for i := 0; i < 8; i++ {
		err := pool.Submit(func() {
			time.Sleep(2 * time.Second) // Long-running task to maintain queue backlog
		})
		if err != nil {
			t.Logf("Task %d submission failed: %v", i, err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for auto scale-up to trigger
	time.Sleep(2 * time.Second)

	workerCount, _, _ := pool.GetStatus()
	if workerCount <= 1 {
		t.Errorf("Should trigger auto scale-up, current worker count: %d", workerCount)
	} else {
		t.Logf("Auto scale-up successful, worker count: %d", workerCount)
	}

	// Wait for tasks to complete
	time.Sleep(3 * time.Second)

	// Test auto scale-down
	t.Log("Testing auto scale-down...")

	// Wait for idle time to exceed threshold
	time.Sleep(4 * time.Second)

	finalWorkerCount, _, _ := pool.GetStatus()
	if finalWorkerCount >= workerCount {
		t.Logf("Scale-down may not have triggered yet, current worker count: %d", finalWorkerCount)
	} else {
		t.Logf("Auto scale-down successful, worker count reduced from %d to %d", workerCount, finalWorkerCount)
	}
}

// Test auto-scaling configuration
func TestTaskPool_AutoScaleConfig(t *testing.T) {
	pool := NewTaskPool(
		WithMinWorkers(1),
		WithMaxWorkers(10),
		WithQueueSize(50),
	)

	// Test getting default configuration
	config := pool.GetAutoScaleConfig()
	if config == nil {
		t.Error("Should have default auto-scaling configuration")
	}

	if !config.EnableAutoScale {
		t.Error("Auto-scaling should be enabled by default")
	}

	// Test updating configuration
	newConfig := &AutoScaleConfig{
		ScaleUpThreshold: 2.0,
		ScaleUpCooldown:  5 * time.Second,
		ScaleUpStep:      2,

		ScaleDownThreshold: 0.5,
		ScaleDownCooldown:  10 * time.Second,
		ScaleDownStep:      1,
		IdleTimeout:        15 * time.Second,

		MonitorInterval: 3 * time.Second,
		HistorySize:     8,
		SmoothingFactor: 0.4,

		EnableAutoScale: true,
	}

	pool.UpdateAutoScaleConfig(newConfig)

	updatedConfig := pool.GetAutoScaleConfig()
	if updatedConfig.ScaleUpThreshold != 2.0 {
		t.Errorf("Configuration update failed, expected threshold 2.0, got %.1f", updatedConfig.ScaleUpThreshold)
	}

	// Test disabling and enabling auto-scaling
	pool.DisableAutoScale()
	if pool.GetAutoScaleConfig().EnableAutoScale {
		t.Error("Failed to disable auto-scaling")
	}

	pool.EnableAutoScale()
	if !pool.GetAutoScaleConfig().EnableAutoScale {
		t.Error("Failed to enable auto-scaling")
	}
}

// Test minimum and maximum worker settings
func TestTaskPool_MinMaxWorkers(t *testing.T) {
	pool := NewTaskPool(
		WithMinWorkers(3),
		WithMaxWorkers(10),
		WithQueueSize(50),
	)
	pool.Start()
	defer pool.Shutdown()

	// Test setting minimum worker count
	err := pool.SetMinWorkers(5)
	if err != nil {
		t.Errorf("Failed to set minimum worker count: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	workerCount, _, _ := pool.GetStatus()
	if workerCount < 5 {
		t.Errorf("Worker count should increase to minimum value 5, actual: %d", workerCount)
	}

	// Test setting maximum worker count
	err = pool.SetMaxWorkers(7)
	if err != nil {
		t.Errorf("Failed to set maximum worker count: %v", err)
	}

	// Test error conditions
	err = pool.SetMinWorkers(0)
	if err == nil {
		t.Error("Setting minimum worker count to 0 should fail")
	}

	err = pool.SetMaxWorkers(3)
	if err == nil {
		t.Error("Setting maximum worker count less than minimum should fail")
	}
}

// Test detailed status retrieval
func TestTaskPool_DetailedStatus(t *testing.T) {
	pool := NewTaskPool(
		WithMinWorkers(1),
		WithMaxWorkers(5),
		WithQueueSize(20),
	)
	pool.Start()
	defer pool.Shutdown()

	status := pool.GetDetailedStatus()

	// Check required fields
	requiredFields := []string{
		"workerCount", "minWorkers", "maxWorkers",
		"queueLength", "queueCapacity", "queueDensity",
		"autoScaleEnabled", "scalingInProgress",
	}

	for _, field := range requiredFields {
		if _, exists := status[field]; !exists {
			t.Errorf("Missing field in detailed status: %s", field)
		}
	}

	// Check some basic values
	if status["minWorkers"].(int) != 1 {
		t.Errorf("Minimum worker count should be 1, actual: %v", status["minWorkers"])
	}

	if status["maxWorkers"].(int) != 5 {
		t.Errorf("Maximum worker count should be 5, actual: %v", status["maxWorkers"])
	}
}
