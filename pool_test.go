package tspool

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tinystack/tslog"
)

// Test basic functionality of worker pool
func TestTaskPool_Basic(t *testing.T) {
	pool := NewTaskPool(
		WithMinWorkers(1),
		WithMaxWorkers(3),
		WithQueueSize(10),
	)
	pool.Start()
	defer pool.Shutdown()

	// Test task submission
	var counter int32
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		err := pool.Submit(func() {
			defer wg.Done()
			atomic.AddInt32(&counter, 1)
			time.Sleep(100 * time.Millisecond)
		})
		if err != nil {
			t.Fatalf("Task submission failed: %v", err)
		}
	}

	wg.Wait()

	if counter != 5 {
		t.Errorf("Expected 5 tasks to execute, actually executed %d", counter)
	}
}

// Test dynamic scaling of workers
func TestTaskPool_DynamicWorkers(t *testing.T) {
	pool := NewTaskPool(
		WithMinWorkers(1),
		WithMaxWorkers(5),
		WithQueueSize(20),
	)
	pool.Start()
	defer pool.Shutdown()

	// Test adding workers
	initialWorkers, _, _ := pool.GetStatus()
	err := pool.AddWorkers(3)
	if err != nil {
		t.Errorf("Failed to add workers: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	newWorkers, _, _ := pool.GetStatus()
	if newWorkers <= initialWorkers {
		t.Errorf("Worker count should increase, initial: %d, current: %d", initialWorkers, newWorkers)
	}

	// Test removing workers
	err = pool.RemoveWorkers(2)
	if err != nil {
		t.Errorf("Failed to remove workers: %v", err)
	}

	time.Sleep(100 * time.Millisecond)
	finalWorkers, _, _ := pool.GetStatus()
	if finalWorkers >= newWorkers {
		t.Errorf("Worker count should decrease, previous: %d, current: %d", newWorkers, finalWorkers)
	}
}

// Test task timeout functionality
func TestTaskPool_Timeout(t *testing.T) {
	pool := NewTaskPool(
		WithMinWorkers(1),
		WithMaxWorkers(2),
		WithQueueSize(5),
	)
	pool.Start()
	defer pool.Shutdown()

	// Test normal task
	var normalTaskDone bool
	err := pool.Submit(func() {
		time.Sleep(100 * time.Millisecond)
		normalTaskDone = true
	})
	if err != nil {
		t.Fatalf("Normal task submission failed: %v", err)
	}

	// Test timeout task
	var timeoutTaskDone bool
	err = pool.SubmitWithTimeout(func() {
		time.Sleep(2 * time.Second)
		timeoutTaskDone = true
	}, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Timeout task submission failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	if !normalTaskDone {
		t.Error("Normal task should have completed")
	}
	if timeoutTaskDone {
		t.Error("Timeout task should not have completed")
	}
}

// Test queue full scenario
func TestTaskPool_QueueFull(t *testing.T) {
	pool := NewTaskPool(
		WithMinWorkers(1),
		WithMaxWorkers(1),
		WithQueueSize(2),
	)
	pool.Start()
	defer pool.Shutdown()

	// Submit tasks to fill the queue
	successCount := 0
	failCount := 0

	for i := 0; i < 10; i++ {
		err := pool.Submit(func() {
			time.Sleep(100 * time.Millisecond)
		})
		if err != nil {
			failCount++
		} else {
			successCount++
		}
	}

	if failCount == 0 {
		t.Error("Some tasks should fail due to full queue")
	}

	if successCount == 0 {
		t.Error("Some tasks should be submitted successfully")
	}
}

// Test worker pool shutdown
func TestTaskPool_Shutdown(t *testing.T) {
	pool := NewTaskPool(
		WithMinWorkers(1),
		WithMaxWorkers(3),
		WithQueueSize(10),
	)
	pool.Start()

	var completedTasks int32
	var wg sync.WaitGroup

	// Submit some tasks
	for i := 0; i < 5; i++ {
		wg.Add(1)
		err := pool.Submit(func() {
			defer wg.Done()
			time.Sleep(100 * time.Millisecond)
			atomic.AddInt32(&completedTasks, 1)
		})
		if err != nil {
			t.Errorf("Task submission failed: %v", err)
			wg.Done()
		}
	}

	// Wait for all tasks to complete
	wg.Wait()

	// Shutdown the worker pool
	pool.Shutdown()

	// Verify all tasks completed
	if completedTasks != 5 {
		t.Errorf("Expected 5 tasks to complete, actually completed %d", completedTasks)
	}

	// Verify no new tasks can be submitted after shutdown
	err := pool.Submit(func() {})
	if err == nil {
		t.Error("Task submission should fail after shutdown")
	}

	// Verify status
	if !pool.IsShutdown() {
		t.Error("Worker pool should be in shutdown state")
	}
}

// Test concurrent safety
func TestTaskPool_ConcurrentSafety(t *testing.T) {
	pool := NewTaskPool(
		WithMinWorkers(1),
		WithMaxWorkers(10),
		WithQueueSize(200),
	)
	pool.Start()
	defer pool.Shutdown()

	var counter int32
	var wg sync.WaitGroup

	// Submit tasks concurrently
	numTasks := 20
	wg.Add(numTasks)

	for i := 0; i < numTasks; i++ {
		go func() {
			err := pool.Submit(func() {
				atomic.AddInt32(&counter, 1)
				time.Sleep(5 * time.Millisecond)
			})
			if err != nil {
				t.Logf("Task submission failed: %v", err)
			}
			wg.Done()
		}()
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond) // Wait for all tasks to complete

	if counter == 0 {
		t.Error("Should execute at least some tasks")
	}

	t.Logf("Concurrent safety test completed, executed %d tasks", counter)
}

// Test status retrieval
func TestTaskPool_GetStatus(t *testing.T) {
	pool := NewTaskPool(
		WithMinWorkers(1),
		WithMaxWorkers(3),
		WithQueueSize(10),
	)
	pool.Start()
	defer pool.Shutdown()

	workerCount, queueLen, maxWorkers := pool.GetStatus()

	if workerCount <= 0 {
		t.Error("Worker count should be greater than 0")
	}

	if queueLen < 0 {
		t.Error("Queue length should not be negative")
	}

	if maxWorkers != 3 {
		t.Errorf("Maximum worker count should be 3, actual: %d", maxWorkers)
	}
}

// Test functional options pattern
func TestTaskPool_WithOptions(t *testing.T) {
	// Test creating pool with options
	pool := NewTaskPool(
		WithMinWorkers(2),
		WithMaxWorkers(8),
		WithQueueSize(50),
		WithAutoScaleEnabled(true),
	)

	if pool.minWorkers != 2 {
		t.Errorf("Expected minWorkers to be 2, got %d", pool.minWorkers)
	}
	if pool.maxWorkers != 8 {
		t.Errorf("Expected maxWorkers to be 8, got %d", pool.maxWorkers)
	}
	if pool.queueSize != 50 {
		t.Errorf("Expected queueSize to be 50, got %d", pool.queueSize)
	}
	if !pool.scaleConfig.EnableAutoScale {
		t.Error("Expected auto-scaling to be enabled")
	}

	// Test with custom logger
	customLogger := tslog.DefaultLogger()
	pool2 := NewTaskPool(
		WithMinWorkers(1),
		WithMaxWorkers(5),
		WithLogger(customLogger),
	)

	if pool2.logger != customLogger {
		t.Error("Custom logger was not set correctly")
	}

	// Test with custom auto-scale config
	customConfig := &AutoScaleConfig{
		ScaleUpThreshold: 3.0,
		ScaleUpStep:      2,
		EnableAutoScale:  false,
	}
	pool3 := NewTaskPool(
		WithAutoScaleConfig(customConfig),
	)

	if pool3.scaleConfig.ScaleUpThreshold != 3.0 {
		t.Errorf("Expected ScaleUpThreshold to be 3.0, got %f", pool3.scaleConfig.ScaleUpThreshold)
	}
	if pool3.scaleConfig.ScaleUpStep != 2 {
		t.Errorf("Expected ScaleUpStep to be 2, got %d", pool3.scaleConfig.ScaleUpStep)
	}
	if pool3.scaleConfig.EnableAutoScale {
		t.Error("Expected auto-scaling to be disabled")
	}

	// Test validation (minWorkers > maxWorkers)
	pool4 := NewTaskPool(
		WithMinWorkers(10),
		WithMaxWorkers(5),
	)
	if pool4.minWorkers != 5 {
		t.Errorf("Expected minWorkers to be adjusted to 5, got %d", pool4.minWorkers)
	}
}
