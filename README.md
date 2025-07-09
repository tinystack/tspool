# TSPool - Go Worker Pool

[![Go Report Card](https://goreportcard.com/badge/github.com/tinystack/tspool)](https://goreportcard.com/report/github.com/tinystack/tspool)
![Go Version](https://img.shields.io/badge/go%20version-%3E=1.21-61CFDD.svg?style=flat-square)
[![PkgGoDev](https://pkg.go.dev/badge/mod/github.com/tinystack/tspool)](https://pkg.go.dev/mod/github.com/tinystack/tspool)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

TSPool is an efficient Go worker pool (task pool) implementation with the following core features:

- ✅ **Safe Worker Pool Shutdown**: Ensures graceful exit after all tasks are completed
- ✅ **Intelligent Auto-Scaling**: Intelligently adjusts worker count based on queue load with anti-jitter design
- ✅ **Dynamic Worker Count Adjustment**: Supports runtime addition and removal of workers
- ✅ **Worker Reuse**: Avoids frequent creation and destruction of goroutines to improve performance
- ✅ **Task Timeout**: Supports setting execution timeout for tasks
- ✅ **Task Priority**: Supports priority scheduling (extended feature)
- ✅ **Status Monitoring**: Provides worker pool status query and monitoring functionality
- ✅ **Concurrent Safety**: All operations are thread-safe
- ✅ **Smooth Load Awareness**: Uses exponential moving average to avoid short-term fluctuation impact

## Installation

```bash
go get github.com/tinystack/tspool
```

## Quick Start

### Basic Usage

```go
package main

import (
    "fmt"
    "time"
    
    "github.com/tinystack/tspool"
)

func main() {
    // Create worker pool: max 10 workers, queue size 50
    pool := tspool.NewTaskPool(
        tspool.WithMinWorkers(1),
        tspool.WithMaxWorkers(10),
        tspool.WithQueueSize(50),
    )
    
    // Start worker pool (automatically enables intelligent auto-scaling)
    pool.Start()
    
    // Submit tasks
    for i := 0; i < 20; i++ {
        taskID := i
        err := pool.Submit(func() {
            fmt.Printf("Executing task %d\n", taskID)
            time.Sleep(100 * time.Millisecond)
        })
        if err != nil {
            fmt.Printf("Task %d submission failed: %v\n", taskID, err)
        }
    }
    
    // Wait for tasks to complete
    time.Sleep(3 * time.Second)
    
    // Safely shutdown worker pool
    pool.Shutdown()
}
```

### Intelligent Auto-Scaling

TSPool's core feature is intelligent auto-scaling, which automatically adjusts worker count based on task load:

```go
// Create worker pool with custom auto-scaling configuration
config := &tspool.AutoScaleConfig{
    ScaleUpThreshold:   1.5,              // Scale up when queue length reaches 1.5x worker count
    ScaleUpCooldown:    5 * time.Second,  // Scale-up cooldown period
    ScaleUpStep:        2,                // Add 2 workers per scale-up
    
    ScaleDownThreshold: 0.3,              // Scale down when queue length is below 0.3x worker count
    ScaleDownCooldown:  15 * time.Second, // Scale-down cooldown period
    ScaleDownStep:      1,                // Remove 1 worker per scale-down
    IdleTimeout:        30 * time.Second, // Start scale-down after 30 seconds idle
    
    MonitorInterval:    3 * time.Second,  // Monitoring interval
    HistorySize:        10,               // Number of queue history records
    SmoothingFactor:    0.3,              // Smoothing factor (0-1)
    
    EnableAutoScale:    true,             // Enable auto-scaling
}

// Create worker pool: max 15 workers, min 2 workers, queue size 100
pool := tspool.NewTaskPool(
    tspool.WithMinWorkers(2),
    tspool.WithMaxWorkers(15),
    tspool.WithQueueSize(100),
    tspool.WithAutoScaleConfig(config),
)
pool.Start()

// Get detailed status
status := pool.GetDetailedStatus()
fmt.Printf("Workers: %d, Queue density: %.2f, Smoothed queue length: %.1f\n",
    status["workerCount"], status["queueDensity"], status["smoothedQueueLen"])
```

## API Reference

### Creating Worker Pool

```go
// NewTaskPool creates a new worker pool with functional options
func NewTaskPool(opts ...Option) *TaskPool
```

### Creating Worker Pool with Options Pattern

TSPool supports functional options pattern for more flexible configuration:

```go
// NewTaskPool creates a worker pool with functional options
func NewTaskPool(opts ...Option) *TaskPool
```

#### Available Options

```go
// Basic configuration
WithMinWorkers(min int)                          // Set minimum workers
WithMaxWorkers(max int)                          // Set maximum workers
WithQueueSize(size int)                          // Set queue size
WithLogger(logger tslog.Logger)                  // Set custom logger
WithAutoScaleConfig(config *AutoScaleConfig)     // Set auto-scaling config
WithAutoScaleEnabled(enabled bool)               // Enable/disable auto-scaling

// Auto-scaling thresholds
WithScaleUpThreshold(threshold float64)          // Set scale-up threshold
WithScaleDownThreshold(threshold float64)        // Set scale-down threshold
WithScaleUpStep(step int)                        // Set scale-up step
WithScaleDownStep(step int)                      // Set scale-down step

// Auto-scaling timing
WithScaleUpCooldown(cooldown time.Duration)      // Set scale-up cooldown
WithScaleDownCooldown(cooldown time.Duration)    // Set scale-down cooldown
WithIdleTimeout(timeout time.Duration)           // Set idle timeout
WithMonitorInterval(interval time.Duration)      // Set monitoring interval
```

#### Usage Examples

```go
// Basic configuration with options
pool := tspool.NewTaskPool(
    tspool.WithMinWorkers(2),
    tspool.WithMaxWorkers(10),
    tspool.WithQueueSize(100),
    tspool.WithAutoScaleEnabled(true),
)

// Advanced configuration with custom auto-scaling
advancedPool := tspool.NewTaskPool(
    tspool.WithMinWorkers(3),
    tspool.WithMaxWorkers(20),
    tspool.WithQueueSize(200),
    tspool.WithScaleUpThreshold(1.5),
    tspool.WithScaleDownThreshold(0.3),
    tspool.WithScaleUpStep(2),
    tspool.WithScaleDownStep(1),
    tspool.WithScaleUpCooldown(3*time.Second),
    tspool.WithScaleDownCooldown(10*time.Second),
    tspool.WithIdleTimeout(5*time.Second),
    tspool.WithMonitorInterval(1*time.Second),
)

// Using custom logger
customLogger := tslog.NewLogger()
pool := tspool.NewTaskPool(
    tspool.WithMinWorkers(1),
    tspool.WithMaxWorkers(5),
    tspool.WithLogger(customLogger),
)
```

### Starting Worker Pool

```go
// Start starts the worker pool
func (tp *TaskPool) Start()
```

### Task Submission

```go
// Submit submits a task to the worker pool
func (tp *TaskPool) Submit(fn func()) error

// SubmitWithTimeout submits a task with timeout
func (tp *TaskPool) SubmitWithTimeout(fn func(), timeout time.Duration) error

// SubmitWithPriority submits a task with priority
func (tp *TaskPool) SubmitWithPriority(fn func(), timeout time.Duration, priority int) error
```

### Dynamic Worker Management

```go
// AddWorkers dynamically increases the number of workers
func (tp *TaskPool) AddWorkers(n int) error

// RemoveWorkers dynamically reduces the number of workers
func (tp *TaskPool) RemoveWorkers(n int) error
```

### Status Query

```go
// GetStatus returns basic worker pool status
func (tp *TaskPool) GetStatus() (workerCount, queueLen, maxWorkers int)

// GetDetailedStatus returns detailed status information
func (tp *TaskPool) GetDetailedStatus() map[string]interface{}

// IsShutdown checks if the worker pool is shutting down
func (tp *TaskPool) IsShutdown() bool

// IsClosed checks if the worker pool is closed
func (tp *TaskPool) IsClosed() bool
```

### Auto-Scaling Management

```go
// UpdateAutoScaleConfig updates auto-scaling configuration
func (tp *TaskPool) UpdateAutoScaleConfig(config *AutoScaleConfig)

// GetAutoScaleConfig returns the current auto-scaling configuration
func (tp *TaskPool) GetAutoScaleConfig() *AutoScaleConfig

// EnableAutoScale enables auto-scaling
func (tp *TaskPool) EnableAutoScale()

// DisableAutoScale disables auto-scaling
func (tp *TaskPool) DisableAutoScale()

// SetMinWorkers sets the minimum number of workers
func (tp *TaskPool) SetMinWorkers(min int) error

// SetMaxWorkers sets the maximum number of workers
func (tp *TaskPool) SetMaxWorkers(max int) error
```

### Monitoring

```go
// Monitor monitors worker pool status
func (tp *TaskPool) Monitor(interval time.Duration)
```

### Safe Shutdown

```go
// Shutdown safely shuts down the worker pool
func (tp *TaskPool) Shutdown()
```

## Advanced Features

### Intelligent Auto-Scaling

TSPool's intelligent auto-scaling algorithm has the following characteristics:

#### Anti-Jitter Mechanism
- **Cooldown Periods**: Scale-up and scale-down have independent cooldown periods to prevent frequent adjustments
- **Queue History Smoothing**: Uses queue history records and exponential moving average to avoid short-term fluctuation impact
- **Idle Detection**: Scale-down is only considered after the queue has been idle for a certain period

#### Intelligent Thresholds
- **Scale-up Threshold**: Scale-up is triggered when queue density (queue length/worker count) exceeds threshold
- **Scale-down Threshold**: Scale-down is triggered when queue density is below threshold and idle time requirements are met
- **Dynamic Adjustment**: Supports runtime adjustment of thresholds and other parameters

### Task Timeout

```go
// Submit a task with 2-second timeout
err := pool.SubmitWithTimeout(func() {
    // Long-running task
    time.Sleep(3 * time.Second)
    fmt.Println("Task completed")
}, 2*time.Second)
```

### Status Monitoring

```go
// Start status monitoring, print status every 2 seconds
pool.Monitor(2 * time.Second)

// Get detailed status
status := pool.GetDetailedStatus()
fmt.Printf("Workers: %d, Queue: %d, Density: %.2f, Auto-scaling enabled: %v\n", 
    status["workerCount"], status["queueLength"], status["queueDensity"], 
    status["autoScaleEnabled"])
```

### Manual Worker Management

```go
// Manually add 3 workers
err := pool.AddWorkers(3)
if err != nil {
    fmt.Printf("Failed to add workers: %v\n", err)
}

// Manually remove 2 workers
err = pool.RemoveWorkers(2)
if err != nil {
    fmt.Printf("Failed to remove workers: %v\n", err)
}
```

### Configuration Management

```go
// Get current configuration
config := pool.GetAutoScaleConfig()

// Update configuration
newConfig := &tspool.AutoScaleConfig{
    ScaleUpThreshold:   2.0,
    ScaleUpCooldown:    8 * time.Second,
    ScaleUpStep:        1,
    // ... other settings
}
pool.UpdateAutoScaleConfig(newConfig)

// Temporarily disable auto-scaling
pool.DisableAutoScale()

// Re-enable auto-scaling
pool.EnableAutoScale()
```

## Configuration Options

### AutoScaleConfig

```go
type AutoScaleConfig struct {
    // Scale-up configuration
    ScaleUpThreshold float64       // Scale-up threshold (queue density)
    ScaleUpCooldown  time.Duration // Scale-up cooldown period
    ScaleUpStep      int           // Workers to add per scale-up

    // Scale-down configuration
    ScaleDownThreshold float64       // Scale-down threshold (queue density)
    ScaleDownCooldown  time.Duration // Scale-down cooldown period
    ScaleDownStep      int           // Workers to remove per scale-down
    IdleTimeout        time.Duration // Idle time before considering scale-down

    // Monitoring configuration
    MonitorInterval time.Duration // Monitoring interval
    HistorySize     int           // Queue history size for smoothing
    SmoothingFactor float64       // Smoothing factor (0-1)

    // Control flags
    EnableAutoScale bool // Whether to enable auto-scaling
}
```

### Default Configuration

```go
// Default auto-scaling configuration
DefaultConfig := &AutoScaleConfig{
    ScaleUpThreshold:   2.0,              // Scale up at 2x queue density
    ScaleUpCooldown:    10 * time.Second, // 10 second scale-up cooldown
    ScaleUpStep:        1,                // Add 1 worker per scale-up
    
    ScaleDownThreshold: 0.5,              // Scale down at 0.5x queue density
    ScaleDownCooldown:  30 * time.Second, // 30 second scale-down cooldown
    ScaleDownStep:      1,                // Remove 1 worker per scale-down
    IdleTimeout:        60 * time.Second, // 60 second idle timeout
    
    MonitorInterval:    5 * time.Second,  // Monitor every 5 seconds
    HistorySize:        12,               // Keep 12 history records
    SmoothingFactor:    0.3,              // 0.3 smoothing factor
    
    EnableAutoScale:    true,             // Auto-scaling enabled
}
```

## Examples

See the [example directory](./example/) for complete usage examples demonstrating:

- Basic task submission
- Intelligent auto-scaling behavior
- Task timeout handling
- Burst load processing
- Status monitoring
- Configuration management
- Safe shutdown

Run the example:

```bash
cd example
go run example.go
```

## Anti-Jitter Design

TSPool implements a sophisticated anti-jitter mechanism to prevent performance degradation caused by frequent worker adjustments:

### Cooldown Mechanism
- **Independent Cooldowns**: Scale-up and scale-down have separate cooldown periods
- **Configurable Intervals**: Cooldown periods can be adjusted based on application characteristics
- **Graceful Handling**: Prevents rapid scaling oscillations

### Load Smoothing
- **Exponential Moving Average**: Uses EMA to smooth queue length measurements
- **Historical Data**: Maintains queue history for trend analysis
- **Noise Reduction**: Filters out short-term load spikes

### Intelligent Decision Making
- **Threshold-Based**: Uses configurable density thresholds for scaling decisions
- **Idle Detection**: Considers queue idle time before scale-down
- **Context Awareness**: Takes into account current scaling state

## Performance Characteristics

- **High Throughput**: Optimized goroutine reuse reduces overhead
- **Low Latency**: Efficient task queue and scheduling
- **Memory Efficient**: Controlled goroutine count prevents resource exhaustion
- **Adaptive Scaling**: Responds to load changes automatically
- **Stable Operation**: Anti-jitter mechanisms prevent performance degradation

## Testing

```bash
# Run all tests
go test -v

# Run tests with coverage
go test -cover

# Run auto-scaling specific tests
go test -v -run TestTaskPool_AutoScale

# Run performance benchmarks
go test -bench=.
```

## Best Practices

1. **Choose Appropriate Pool Size**: Set `maxWorkers` based on your system's CPU cores and I/O characteristics
2. **Configure Queue Size**: Set queue size to handle peak loads without excessive memory usage
3. **Tune Auto-Scaling**: Adjust thresholds and cooldown periods based on your application's load patterns
4. **Monitor Performance**: Use detailed status monitoring to understand pool behavior
5. **Graceful Shutdown**: Always call `Shutdown()` to ensure tasks complete properly
6. **Error Handling**: Always check errors when submitting tasks
7. **Avoid Blocking Tasks**: Keep tasks short and non-blocking when possible

## Troubleshooting

### High Memory Usage
- Reduce `queueSize` if task queue consumes too much memory
- Lower `maxWorkers` if too many goroutines are created
- Check for task leaks or long-running tasks

### Poor Auto-Scaling Performance
- Adjust `ScaleUpThreshold` and `ScaleDownThreshold` based on load patterns
- Increase cooldown periods if scaling is too aggressive
- Check `MonitorInterval` and `HistorySize` settings

### Task Submission Failures
- Increase `queueSize` if tasks are rejected due to full queue
- Check if worker pool is properly started
- Verify worker pool is not shut down

## Changelog

### v2.0.0 (Latest)
- 🚀 **Major Update**: Intelligent auto-scaling system
- ✨ Added anti-jitter mechanism to prevent frequent worker adjustments
- ✨ Added smooth load awareness using exponential moving average algorithm
- ✨ Added cooldown periods and idle detection mechanism
- ✨ Added detailed status monitoring interface
- ✨ Added dynamic minimum/maximum worker count setting
- ✨ Added hot-reload auto-scaling configuration
- 🔧 Optimized worker pool startup logic
- 📝 Improved documentation and examples
- ✅ Added comprehensive test coverage with 11 test cases

### v1.0.0
- Initial release
- Basic worker pool functionality
- Manual dynamic scaling
- Task timeout support
- Status monitoring
- Comprehensive test coverage

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the project
2. Create your feature branch (`git checkout -b feature/AmazingFeature`)
3. Commit your changes (`git commit -m 'Add some AmazingFeature'`)
4. Push to the branch (`git push origin feature/AmazingFeature`)
5. Open a Pull Request

## Support

If you encounter any issues or have questions, please [open an issue](https://github.com/tinystack/tspool/issues) on GitHub.

## Related Projects

- [ants](https://github.com/panjf2000/ants) - A high-performance goroutine pool
- [tunny](https://github.com/Jeffail/tunny) - A goroutine pool for Go
- [pool](https://github.com/go-playground/pool) - A limited consumer goroutine pool

## Star History

If you find this project helpful, please consider giving it a ⭐ on GitHub! 