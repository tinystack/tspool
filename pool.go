package tspool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tinystack/tslog"
)

// Task represents a task to be executed
type Task struct {
	fn       func()
	timeout  time.Duration
	priority int
}

// TaskPool represents a worker pool structure
type TaskPool struct {
	// Basic configuration
	maxWorkers  int   // Maximum number of workers
	minWorkers  int   // Minimum number of workers
	workerCount int32 // Current number of workers (atomic operation)

	// Task queue and channels
	taskQueue chan Task
	queueSize int

	// Context and cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Synchronization control
	wg       sync.WaitGroup // Wait for all tasks to complete
	workerWg sync.WaitGroup // Wait for all workers to exit
	mu       sync.RWMutex   // Read-write lock to protect internal state

	// Status control
	shutdown int32 // Shutdown flag (atomic operation)
	closed   int32 // Whether closed (atomic operation)

	// Dynamic adjustment control
	workerControl chan int // Signal channel for dynamic worker adjustment

	// Auto-scaling configuration
	scaleConfig *AutoScaleConfig

	// Auto-scaling state
	lastScaleTime     time.Time // Last scaling time
	queueHistory      []int     // Queue length history
	idleStartTime     time.Time // Idle start time
	scalingInProgress int32     // Scaling in progress flag

	logger tslog.Logger
}

// Option defines a functional option for TaskPool configuration
type Option func(*TaskPool)

// WithMinWorkers sets the minimum number of workers
func WithMinWorkers(min int) Option {
	return func(tp *TaskPool) {
		if min > 0 {
			tp.minWorkers = min
		}
	}
}

// WithMaxWorkers sets the maximum number of workers
func WithMaxWorkers(max int) Option {
	return func(tp *TaskPool) {
		if max > 0 {
			tp.maxWorkers = max
		}
	}
}

// WithQueueSize sets the task queue size
func WithQueueSize(size int) Option {
	return func(tp *TaskPool) {
		if size > 0 {
			tp.queueSize = size
		}
	}
}

// WithAutoScaleConfig sets the auto-scaling configuration
func WithAutoScaleConfig(config *AutoScaleConfig) Option {
	return func(tp *TaskPool) {
		if config != nil {
			tp.scaleConfig = config
		}
	}
}

// WithLogger sets a custom logger
func WithLogger(logger tslog.Logger) Option {
	return func(tp *TaskPool) {
		if logger != nil {
			tp.logger = logger
		}
	}
}

// WithAutoScaleEnabled enables or disables auto-scaling
func WithAutoScaleEnabled(enabled bool) Option {
	return func(tp *TaskPool) {
		if tp.scaleConfig == nil {
			tp.scaleConfig = DefaultAutoScaleConfig()
		}
		tp.scaleConfig.EnableAutoScale = enabled
	}
}

// WithScaleUpThreshold sets the scale-up threshold
func WithScaleUpThreshold(threshold float64) Option {
	return func(tp *TaskPool) {
		if tp.scaleConfig == nil {
			tp.scaleConfig = DefaultAutoScaleConfig()
		}
		if threshold > 0 {
			tp.scaleConfig.ScaleUpThreshold = threshold
		}
	}
}

// WithScaleDownThreshold sets the scale-down threshold
func WithScaleDownThreshold(threshold float64) Option {
	return func(tp *TaskPool) {
		if tp.scaleConfig == nil {
			tp.scaleConfig = DefaultAutoScaleConfig()
		}
		if threshold > 0 {
			tp.scaleConfig.ScaleDownThreshold = threshold
		}
	}
}

// WithScaleUpStep sets the number of workers to add during scale-up
func WithScaleUpStep(step int) Option {
	return func(tp *TaskPool) {
		if tp.scaleConfig == nil {
			tp.scaleConfig = DefaultAutoScaleConfig()
		}
		if step > 0 {
			tp.scaleConfig.ScaleUpStep = step
		}
	}
}

// WithScaleDownStep sets the number of workers to remove during scale-down
func WithScaleDownStep(step int) Option {
	return func(tp *TaskPool) {
		if tp.scaleConfig == nil {
			tp.scaleConfig = DefaultAutoScaleConfig()
		}
		if step > 0 {
			tp.scaleConfig.ScaleDownStep = step
		}
	}
}

// WithScaleUpCooldown sets the cooldown period after scale-up
func WithScaleUpCooldown(cooldown time.Duration) Option {
	return func(tp *TaskPool) {
		if tp.scaleConfig == nil {
			tp.scaleConfig = DefaultAutoScaleConfig()
		}
		if cooldown > 0 {
			tp.scaleConfig.ScaleUpCooldown = cooldown
		}
	}
}

// WithScaleDownCooldown sets the cooldown period after scale-down
func WithScaleDownCooldown(cooldown time.Duration) Option {
	return func(tp *TaskPool) {
		if tp.scaleConfig == nil {
			tp.scaleConfig = DefaultAutoScaleConfig()
		}
		if cooldown > 0 {
			tp.scaleConfig.ScaleDownCooldown = cooldown
		}
	}
}

// WithIdleTimeout sets the idle timeout before considering scale-down
func WithIdleTimeout(timeout time.Duration) Option {
	return func(tp *TaskPool) {
		if tp.scaleConfig == nil {
			tp.scaleConfig = DefaultAutoScaleConfig()
		}
		if timeout > 0 {
			tp.scaleConfig.IdleTimeout = timeout
		}
	}
}

// WithMonitorInterval sets the monitoring interval
func WithMonitorInterval(interval time.Duration) Option {
	return func(tp *TaskPool) {
		if tp.scaleConfig == nil {
			tp.scaleConfig = DefaultAutoScaleConfig()
		}
		if interval > 0 {
			tp.scaleConfig.MonitorInterval = interval
		}
	}
}

// NewTaskPool creates a new TaskPool instance with functional options
func NewTaskPool(opts ...Option) *TaskPool {
	// Default configuration
	tp := &TaskPool{
		minWorkers:  1,
		maxWorkers:  10,
		queueSize:   100,
		workerCount: 0,
		scaleConfig: DefaultAutoScaleConfig(),
		logger:      tslog.DefaultLogger(),
	}

	// Apply options
	for _, opt := range opts {
		opt(tp)
	}

	// Validate configuration
	if tp.minWorkers > tp.maxWorkers {
		tp.minWorkers = tp.maxWorkers
	}
	if tp.minWorkers <= 0 {
		tp.minWorkers = 1
	}
	if tp.maxWorkers <= 0 {
		tp.maxWorkers = 10
	}
	if tp.queueSize <= 0 {
		tp.queueSize = 100
	}

	// Initialize context and channels
	ctx, cancel := context.WithCancel(context.Background())
	tp.ctx = ctx
	tp.cancel = cancel
	tp.taskQueue = make(chan Task, tp.queueSize)
	tp.workerControl = make(chan int, tp.maxWorkers)

	// Initialize auto-scaling state
	tp.lastScaleTime = time.Now()
	tp.queueHistory = make([]int, tp.scaleConfig.HistorySize)
	tp.idleStartTime = time.Now()
	tp.scalingInProgress = 0

	return tp
}

// Start starts the worker pool
func (tp *TaskPool) Start() {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if atomic.LoadInt32(&tp.closed) == 1 {
		return
	}

	// Start initial workers
	initialWorkers := tp.minWorkers
	if initialWorkers == 0 {
		initialWorkers = 1
	}

	for i := 0; i < initialWorkers; i++ {
		tp.startWorker()
	}

	// Start auto-scaling monitor
	if tp.scaleConfig.EnableAutoScale {
		go tp.autoScaleMonitor()
	}

	tp.logger.Info("Worker pool started, initial workers: %d, minimum workers: %d, maximum workers: %d",
		initialWorkers, tp.minWorkers, tp.maxWorkers)
}

// startWorker starts a new worker goroutine
func (tp *TaskPool) startWorker() {
	atomic.AddInt32(&tp.workerCount, 1)
	tp.workerWg.Add(1)

	go func() {
		defer tp.workerWg.Done()
		defer atomic.AddInt32(&tp.workerCount, -1)

		for {
			select {
			case <-tp.ctx.Done():
				// Received shutdown signal, gracefully exit
				return
			case signal := <-tp.workerControl:
				if signal < 0 {
					// Received signal to reduce workers
					return
				}
			case task := <-tp.taskQueue:
				// Execute task
				tp.executeTask(task)
			}
		}
	}()
}

// executeTask executes a single task
func (tp *TaskPool) executeTask(task Task) {
	defer tp.wg.Done()

	if task.timeout > 0 {
		// Execute task with timeout
		done := make(chan struct{})
		go func() {
			defer close(done)
			task.fn()
		}()

		select {
		case <-done:
			// Task completed normally
		case <-time.After(task.timeout):
			// Task timed out
			tp.logger.Warn("Task execution timeout: %v", task.timeout)
		}
	} else {
		// Execute task normally
		task.fn()
	}
}

// Submit submits a task to the worker pool
func (tp *TaskPool) Submit(fn func()) error {
	return tp.SubmitWithTimeout(fn, 0)
}

// SubmitWithTimeout submits a task with timeout
func (tp *TaskPool) SubmitWithTimeout(fn func(), timeout time.Duration) error {
	return tp.SubmitWithPriority(fn, timeout, 0)
}

// SubmitWithPriority submits a task with priority
func (tp *TaskPool) SubmitWithPriority(fn func(), timeout time.Duration, priority int) error {
	if atomic.LoadInt32(&tp.shutdown) == 1 {
		return errors.New("worker pool is shutting down, cannot submit new tasks")
	}

	if atomic.LoadInt32(&tp.closed) == 1 {
		return errors.New("worker pool is closed, cannot submit new tasks")
	}

	task := Task{
		fn:       fn,
		timeout:  timeout,
		priority: priority,
	}

	tp.wg.Add(1)

	select {
	case tp.taskQueue <- task:
		// Task successfully added to queue
		tp.autoScale()
		return nil
	case <-tp.ctx.Done():
		tp.wg.Done()
		return errors.New("worker pool is closed")
	default:
		tp.wg.Done()
		return errors.New("task queue is full")
	}
}

// autoScale automatically adjusts worker count based on task queue length (deprecated, use autoScaleMonitor)
func (tp *TaskPool) autoScale() {
	// Skip old logic if new auto-scaling is enabled
	if tp.scaleConfig.EnableAutoScale {
		return
	}

	queueLen := len(tp.taskQueue)
	currentWorkers := int(atomic.LoadInt32(&tp.workerCount))

	// Add workers if queue length exceeds 2x current workers and not at max
	if queueLen > currentWorkers*2 && currentWorkers < tp.maxWorkers {
		tp.AddWorkers(1)
	}
}

// AddWorkers dynamically increases the number of workers
func (tp *TaskPool) AddWorkers(n int) error {
	if n <= 0 {
		return errors.New("worker count must be greater than 0")
	}

	if atomic.LoadInt32(&tp.shutdown) == 1 {
		return errors.New("worker pool is shutting down, cannot add workers")
	}

	tp.mu.Lock()
	defer tp.mu.Unlock()

	currentWorkers := int(atomic.LoadInt32(&tp.workerCount))
	if currentWorkers+n > tp.maxWorkers {
		n = tp.maxWorkers - currentWorkers
	}

	if n <= 0 {
		return errors.New("maximum worker count reached")
	}

	for i := 0; i < n; i++ {
		tp.startWorker()
	}

	tp.logger.Info("Added %d workers, current worker count: %d", n, atomic.LoadInt32(&tp.workerCount))
	return nil
}

// RemoveWorkers dynamically reduces the number of workers
func (tp *TaskPool) RemoveWorkers(n int) error {
	if n <= 0 {
		return errors.New("worker count must be greater than 0")
	}

	currentWorkers := int(atomic.LoadInt32(&tp.workerCount))
	if currentWorkers <= 1 {
		return errors.New("at least one worker must be kept")
	}

	if n >= currentWorkers {
		n = currentWorkers - 1
	}

	// Send signals to reduce workers
	for i := 0; i < n; i++ {
		select {
		case tp.workerControl <- -1:
			// Signal sent successfully
		default:
			// Signal channel is full, stop sending
			break
		}
	}

	tp.logger.Info("Requested to remove %d workers, current worker count: %d", n, atomic.LoadInt32(&tp.workerCount))
	return nil
}

// Shutdown safely shuts down the worker pool
func (tp *TaskPool) Shutdown() {
	atomic.StoreInt32(&tp.shutdown, 1)

	tp.logger.Info("Starting to shut down worker pool, waiting for all tasks to complete...")

	// Wait for all submitted tasks to complete
	tp.wg.Wait()

	// Cancel context to notify all workers to exit
	tp.cancel()

	// Wait for all worker goroutines to exit
	tp.workerWg.Wait()

	// Close channels
	close(tp.taskQueue)
	close(tp.workerControl)

	atomic.StoreInt32(&tp.closed, 1)
	tp.logger.Info("Worker pool has been safely shut down")
}

// GetStatus returns the current worker pool status
func (tp *TaskPool) GetStatus() (int, int, int) {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	return int(atomic.LoadInt32(&tp.workerCount)), len(tp.taskQueue), tp.maxWorkers
}

// GetDetailedStatus returns detailed worker pool status
func (tp *TaskPool) GetDetailedStatus() map[string]interface{} {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	smoothedQueueLen := tp.getSmoothedQueueLength()
	currentWorkers := int(atomic.LoadInt32(&tp.workerCount))

	var queueDensity float64
	if currentWorkers > 0 {
		queueDensity = float64(len(tp.taskQueue)) / float64(currentWorkers)
	}

	return map[string]interface{}{
		"workerCount":       currentWorkers,
		"minWorkers":        tp.minWorkers,
		"maxWorkers":        tp.maxWorkers,
		"queueLength":       len(tp.taskQueue),
		"queueCapacity":     tp.queueSize,
		"queueDensity":      queueDensity,
		"smoothedQueueLen":  smoothedQueueLen,
		"autoScaleEnabled":  tp.scaleConfig.EnableAutoScale,
		"lastScaleTime":     tp.lastScaleTime,
		"idleStartTime":     tp.idleStartTime,
		"scalingInProgress": atomic.LoadInt32(&tp.scalingInProgress) == 1,
		"queueHistory":      tp.queueHistory,
	}
}

// IsShutdown checks if the worker pool is shutting down
func (tp *TaskPool) IsShutdown() bool {
	return atomic.LoadInt32(&tp.shutdown) == 1
}

// IsClosed checks if the worker pool is closed
func (tp *TaskPool) IsClosed() bool {
	return atomic.LoadInt32(&tp.closed) == 1
}

// Monitor monitors the worker pool status (optional feature)
func (tp *TaskPool) Monitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ticker.C:
				workerCount, queueLen, maxWorkers := tp.GetStatus()
				if !tp.IsClosed() {
					tp.logger.Info("Worker pool status - Current workers: %d, Queue length: %d, Max workers: %d",
						workerCount, queueLen, maxWorkers)
				}
			case <-tp.ctx.Done():
				return
			}
		}
	}()
}

// SetMinWorkers sets the minimum number of workers
func (tp *TaskPool) SetMinWorkers(min int) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if min <= 0 {
		return errors.New("minimum worker count must be greater than 0")
	}

	if min > tp.maxWorkers {
		return errors.New("minimum worker count cannot be greater than maximum worker count")
	}

	tp.minWorkers = min

	// Add workers if current count is less than minimum
	currentWorkers := int(atomic.LoadInt32(&tp.workerCount))
	if currentWorkers < min {
		needed := min - currentWorkers
		for i := 0; i < needed; i++ {
			tp.startWorker()
		}
		tp.logger.Info("Added workers to minimum count: %d", min)
	}

	return nil
}

// SetMaxWorkers sets the maximum number of workers
func (tp *TaskPool) SetMaxWorkers(max int) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	if max <= 0 {
		return errors.New("maximum worker count must be greater than 0")
	}

	if max < tp.minWorkers {
		return errors.New("maximum worker count cannot be less than minimum worker count")
	}

	tp.maxWorkers = max

	// Remove workers if current count exceeds maximum
	currentWorkers := int(atomic.LoadInt32(&tp.workerCount))
	if currentWorkers > max {
		excess := currentWorkers - max
		tp.RemoveWorkers(excess)
		tp.logger.Info("Reduced workers to maximum count: %d", max)
	}

	return nil
}
