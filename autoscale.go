package tspool

import (
	"sync/atomic"
	"time"
)

// AutoScaleConfig defines the configuration for intelligent auto-scaling
type AutoScaleConfig struct {
	// Scale-up configuration
	ScaleUpThreshold float64       // Scale-up queue threshold (queue length / current worker count)
	ScaleUpCooldown  time.Duration // Cooldown period after scaling up
	ScaleUpStep      int           // Number of workers to add in each scale-up operation

	// Scale-down configuration
	ScaleDownThreshold float64       // Scale-down queue threshold (queue length / current worker count)
	ScaleDownCooldown  time.Duration // Cooldown period after scaling down
	ScaleDownStep      int           // Number of workers to remove in each scale-down operation
	IdleTimeout        time.Duration // Idle time before considering scale-down

	// Monitoring configuration
	MonitorInterval time.Duration // Monitoring interval
	HistorySize     int           // Size of queue history for smoothing
	SmoothingFactor float64       // Smoothing factor (0-1)

	// Control flags
	EnableAutoScale bool // Whether to enable auto-scaling
}

// DefaultAutoScaleConfig returns the default auto-scaling configuration
func DefaultAutoScaleConfig() *AutoScaleConfig {
	return &AutoScaleConfig{
		ScaleUpThreshold: 2.0,              // Scale up when queue length reaches 2x current worker count
		ScaleUpCooldown:  10 * time.Second, // 10 seconds cooldown for scale-up
		ScaleUpStep:      1,                // Add 1 worker per scale-up

		ScaleDownThreshold: 0.5,              // Scale down when queue length is below 0.5x current worker count
		ScaleDownCooldown:  30 * time.Second, // 30 seconds cooldown for scale-down
		ScaleDownStep:      1,                // Remove 1 worker per scale-down
		IdleTimeout:        60 * time.Second, // Start scale-down after 60 seconds idle

		MonitorInterval: 5 * time.Second, // Monitor every 5 seconds
		HistorySize:     12,              // Keep 12 history records (1 minute)
		SmoothingFactor: 0.3,             // Smoothing factor

		EnableAutoScale: true, // Enable auto-scaling by default
	}
}

// autoScaleMonitor continuously monitors and performs auto-scaling
func (tp *TaskPool) autoScaleMonitor() {
	ticker := time.NewTicker(tp.scaleConfig.MonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tp.ctx.Done():
			return
		case <-ticker.C:
			if tp.scaleConfig.EnableAutoScale {
				tp.performAutoScale()
			}
		}
	}
}

// performAutoScale performs the actual auto-scaling logic
func (tp *TaskPool) performAutoScale() {
	// Check if scaling is already in progress
	if !atomic.CompareAndSwapInt32(&tp.scalingInProgress, 0, 1) {
		return
	}
	defer atomic.StoreInt32(&tp.scalingInProgress, 0)

	tp.mu.Lock()
	defer tp.mu.Unlock()

	if atomic.LoadInt32(&tp.closed) == 1 {
		return
	}

	currentWorkers := int(atomic.LoadInt32(&tp.workerCount))
	queueLen := len(tp.taskQueue)

	// Update queue history
	tp.updateQueueHistory(queueLen)

	// Update idle time
	tp.updateIdleTime(queueLen)

	// Calculate queue density
	queueDensity := float64(queueLen) / float64(currentWorkers)

	now := time.Now()

	// Check if should scale up
	if tp.shouldScaleUp(queueDensity, currentWorkers, now) {
		tp.scaleUp()
	} else if tp.shouldScaleDown(queueDensity, currentWorkers, now, queueLen) {
		tp.scaleDown()
	}
}

// updateQueueHistory updates the queue length history for smoothing
func (tp *TaskPool) updateQueueHistory(queueLen int) {
	tp.queueHistory = append(tp.queueHistory, queueLen)
	if len(tp.queueHistory) > tp.scaleConfig.HistorySize {
		tp.queueHistory = tp.queueHistory[1:]
	}
}

// getSmoothedQueueLength calculates the smoothed queue length using exponential moving average
func (tp *TaskPool) getSmoothedQueueLength() float64 {
	if len(tp.queueHistory) == 0 {
		return 0
	}

	if len(tp.queueHistory) == 1 {
		return float64(tp.queueHistory[0])
	}

	// Calculate exponential moving average
	smoothed := float64(tp.queueHistory[0])
	for i := 1; i < len(tp.queueHistory); i++ {
		smoothed = tp.scaleConfig.SmoothingFactor*float64(tp.queueHistory[i]) +
			(1-tp.scaleConfig.SmoothingFactor)*smoothed
	}

	return smoothed
}

// shouldScaleUp determines if the pool should scale up
func (tp *TaskPool) shouldScaleUp(queueDensity float64, currentWorkers int, now time.Time) bool {
	// Check if we've reached the maximum workers
	if currentWorkers >= tp.maxWorkers {
		return false
	}

	// Check if queue density exceeds threshold
	if queueDensity < tp.scaleConfig.ScaleUpThreshold {
		return false
	}

	// Check cooldown period
	if now.Sub(tp.lastScaleTime) < tp.scaleConfig.ScaleUpCooldown {
		return false
	}

	return true
}

// shouldScaleDown determines if the pool should scale down
func (tp *TaskPool) shouldScaleDown(queueDensity float64, currentWorkers int, now time.Time, queueLen int) bool {
	// Check if we've reached the minimum workers
	if currentWorkers <= tp.minWorkers {
		return false
	}

	// Check if queue density is below threshold
	if queueDensity > tp.scaleConfig.ScaleDownThreshold {
		return false
	}

	// Check cooldown period
	if now.Sub(tp.lastScaleTime) < tp.scaleConfig.ScaleDownCooldown {
		return false
	}

	// Check if queue has been idle for enough time
	if queueLen > 0 {
		tp.idleStartTime = now
		return false
	}

	// Check idle timeout
	if now.Sub(tp.idleStartTime) < tp.scaleConfig.IdleTimeout {
		return false
	}

	return true
}

// scaleUp increases the number of workers
func (tp *TaskPool) scaleUp() {
	workersToAdd := tp.scaleConfig.ScaleUpStep
	actualWorkers := int(atomic.LoadInt32(&tp.workerCount))

	// Don't exceed maximum workers
	if actualWorkers+workersToAdd > tp.maxWorkers {
		workersToAdd = tp.maxWorkers - actualWorkers
	}

	if workersToAdd > 0 {
		for i := 0; i < workersToAdd; i++ {
			tp.startWorker()
		}
		tp.lastScaleTime = time.Now()

		newWorkerCount := int(atomic.LoadInt32(&tp.workerCount))
		tp.logger.Debug("Auto scale-up: added %d workers, current worker count: %d", workersToAdd, newWorkerCount)
	}
}

// scaleDown decreases the number of workers
func (tp *TaskPool) scaleDown() {
	workersToRemove := tp.scaleConfig.ScaleDownStep
	actualWorkers := int(atomic.LoadInt32(&tp.workerCount))

	// Don't go below minimum workers
	if actualWorkers-workersToRemove < tp.minWorkers {
		workersToRemove = actualWorkers - tp.minWorkers
	}

	if workersToRemove > 0 {
	removeLoop:
		for i := 0; i < workersToRemove; i++ {
			select {
			case tp.workerControl <- -1:
				// Successfully sent signal to remove worker
			default:
				// Channel is full, stop trying
				break removeLoop
			}
		}
		tp.lastScaleTime = time.Now()

		newWorkerCount := int(atomic.LoadInt32(&tp.workerCount))
		tp.logger.Debug("Auto scale-down: removed %d workers, current worker count: %d", workersToRemove, newWorkerCount)
	}
}

// updateIdleTime updates the idle start time
func (tp *TaskPool) updateIdleTime(queueLen int) {
	if queueLen > 0 {
		tp.idleStartTime = time.Now()
	}
}

// UpdateAutoScaleConfig updates the auto-scaling configuration
func (tp *TaskPool) UpdateAutoScaleConfig(config *AutoScaleConfig) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	if config != nil {
		tp.scaleConfig = config
		tp.logger.Info("Auto-scaling configuration updated")
	}
}

// GetAutoScaleConfig returns the current auto-scaling configuration
func (tp *TaskPool) GetAutoScaleConfig() *AutoScaleConfig {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	// Return a copy to prevent external modification
	configCopy := *tp.scaleConfig
	return &configCopy
}

// EnableAutoScale enables auto-scaling
func (tp *TaskPool) EnableAutoScale() {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	if !tp.scaleConfig.EnableAutoScale {
		tp.scaleConfig.EnableAutoScale = true
		tp.logger.Info("Auto-scaling enabled")
		// Start auto-scaling monitor if pool is not closed
		if atomic.LoadInt32(&tp.closed) == 0 {
			go tp.autoScaleMonitor()
		}
	}
}

// DisableAutoScale disables auto-scaling
func (tp *TaskPool) DisableAutoScale() {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	if tp.scaleConfig.EnableAutoScale {
		tp.scaleConfig.EnableAutoScale = false
		tp.logger.Info("Auto-scaling disabled")
	}
}
