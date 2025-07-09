package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/tinystack/tslog"
	"github.com/tinystack/tspool"
)

func main() {
	fmt.Println("=== Go Worker Pool (TSPool) Demo ===")

	// --- Demo 0: Options Pattern ---
	fmt.Println("\n--- Demo 0: Functional Options Pattern ---")

	// Create a pool using functional options
	optionsPool := tspool.NewTaskPool(
		tspool.WithMinWorkers(2),
		tspool.WithMaxWorkers(6),
		tspool.WithQueueSize(30),
		tspool.WithAutoScaleEnabled(true),
	)

	// Create a custom logger for the pool
	logger := tslog.DefaultLogger()

	// Create another pool with custom logger
	optionsPool2 := tspool.NewTaskPool(
		tspool.WithMinWorkers(1),
		tspool.WithMaxWorkers(4),
		tspool.WithLogger(logger),
	)

	// Create a pool with advanced auto-scaling configuration
	advancedPool := tspool.NewTaskPool(
		tspool.WithMinWorkers(2),
		tspool.WithMaxWorkers(8),
		tspool.WithQueueSize(50),
		tspool.WithScaleUpThreshold(1.5),
		tspool.WithScaleDownThreshold(0.3),
		tspool.WithScaleUpStep(2),
		tspool.WithScaleDownStep(1),
		tspool.WithScaleUpCooldown(2*time.Second),
		tspool.WithScaleDownCooldown(5*time.Second),
		tspool.WithIdleTimeout(3*time.Second),
		tspool.WithMonitorInterval(1*time.Second),
		tspool.WithAutoScaleEnabled(true),
	)

	fmt.Printf("Advanced pool config - ScaleUpThreshold: %.1f, ScaleDownThreshold: %.1f, ScaleUpStep: %d\n",
		advancedPool.GetAutoScaleConfig().ScaleUpThreshold,
		advancedPool.GetAutoScaleConfig().ScaleDownThreshold,
		advancedPool.GetAutoScaleConfig().ScaleUpStep)

	optionsPool.Start()
	optionsPool2.Start()
	advancedPool.Start()

	// Test basic functionality
	for i := 0; i < 3; i++ {
		i := i
		err := optionsPool.Submit(func() {
			fmt.Printf("Options pool task %d executing\n", i)
		})
		if err != nil {
			fmt.Printf("Failed to submit task: %v\n", err)
		}
	}

	// Wait a bit for tasks to complete
	time.Sleep(200 * time.Millisecond)

	// Get status
	workers, queueLen, maxWorkers := optionsPool.GetStatus()
	fmt.Printf("Options pool status: Workers=%d, Queue=%d, MaxWorkers=%d\n", workers, queueLen, maxWorkers)

	// Clean up
	optionsPool.Shutdown()
	optionsPool2.Shutdown()
	advancedPool.Shutdown()

	fmt.Println("Options pattern demo completed\n")

	// Create custom auto-scaling configuration
	config := &tspool.AutoScaleConfig{
		ScaleUpThreshold: 1.5,             // Scale up when queue length reaches 1.5x worker count
		ScaleUpCooldown:  3 * time.Second, // Scale up cooldown 3 seconds
		ScaleUpStep:      2,               // Add 2 workers per scale-up

		ScaleDownThreshold: 0.3,              // Scale down when queue length is below 0.3x worker count
		ScaleDownCooldown:  8 * time.Second,  // Scale down cooldown 8 seconds
		ScaleDownStep:      1,                // Remove 1 worker per scale-down
		IdleTimeout:        10 * time.Second, // Start scale-down after 10 seconds idle

		MonitorInterval: 2 * time.Second, // Monitor every 2 seconds
		HistorySize:     10,              // Keep 10 history records
		SmoothingFactor: 0.3,             // Smoothing factor

		EnableAutoScale: true, // Enable auto-scaling
	}

	// Create worker pool: max 10 workers, min 2 workers, queue size 50
	pool := tspool.NewTaskPool(
		tspool.WithMinWorkers(2),
		tspool.WithMaxWorkers(10),
		tspool.WithQueueSize(50),
		tspool.WithAutoScaleConfig(config),
		tspool.WithLogger(tslog.DefaultLogger()),
	)

	// Start worker pool
	pool.Start()

	// Start detailed status monitoring
	go detailedStatusMonitor(pool)

	// Demo 1: Basic task submission
	fmt.Println("\n--- Demo 1: Basic Task Submission ---")
	basicTaskDemo(pool)

	// Demo 2: Intelligent auto-scaling test
	fmt.Println("\n--- Demo 2: Intelligent Auto-Scaling Test ---")
	autoScaleDemo(pool)

	// Demo 3: Task timeout functionality
	fmt.Println("\n--- Demo 3: Task Timeout Functionality ---")
	timeoutTaskDemo(pool)

	// Demo 4: Burst load handling
	fmt.Println("\n--- Demo 4: Burst Load Handling ---")
	burstLoadDemo(pool)

	// Demo 5: Load reduction and scale-down
	fmt.Println("\n--- Demo 5: Load Reduction and Scale-Down ---")
	idleScaleDownDemo(pool)

	// Wait for tasks to complete
	time.Sleep(5 * time.Second)

	// Demo 6: Intelligent auto-scaling
	fmt.Println("\n--- Demo 6: Intelligent Auto-Scaling ---")
	autoScaleDemo(pool)

	// Wait for tasks to complete
	time.Sleep(5 * time.Second)

	// Demo 7: Safe shutdown of worker pool
	fmt.Println("\n--- Demo 7: Safe Worker Pool Shutdown ---")
	pool.Shutdown()

	// Try to submit tasks after shutdown
	fmt.Println("\n--- Demo 7: Task Submission After Shutdown ---")
	err := pool.Submit(func() {
		fmt.Println("This task will not be executed")
	})
	if err != nil {
		fmt.Printf("Expected error: %v\n", err)
	}

	fmt.Println("\n=== Demo Complete ===")
}

// 基本任务提交演示
func basicTaskDemo(pool *tspool.TaskPool) {
	var wg sync.WaitGroup

	// 提交10个基本任务
	for i := 0; i < 10; i++ {
		taskID := i
		wg.Add(1)

		err := pool.Submit(func() {
			defer wg.Done()
			// Using fmt.Printf here is appropriate for demo output
			fmt.Printf("Executing task %d (Goroutine ID: %d)\n", taskID, getGoroutineID())
			time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
		})

		if err != nil {
			fmt.Printf("Task %d submission failed: %v\n", taskID, err)
			wg.Done()
		}
	}

	wg.Wait()
	fmt.Println("All basic tasks completed")
}

// 手动动态调整协程数量演示（保留用于对比）
func manualWorkerDemo(pool *tspool.TaskPool) {
	workerCount, queueLen, maxWorkers := pool.GetStatus()
	fmt.Printf("Current status: workers=%d, queue length=%d, max workers=%d\n", workerCount, queueLen, maxWorkers)

	// 手动增加协程数量
	fmt.Println("Manually adding 2 workers...")
	err := pool.AddWorkers(2)
	if err != nil {
		fmt.Printf("Failed to add workers: %v\n", err)
	}

	time.Sleep(500 * time.Millisecond)

	// 提交一些任务来测试新增的协程
	for i := 0; i < 5; i++ {
		taskID := i
		pool.Submit(func() {
			fmt.Printf("Manual adjustment test task %d\n", taskID)
			time.Sleep(200 * time.Millisecond)
		})
	}

	time.Sleep(1 * time.Second)

	// 手动减少协程数量
	fmt.Println("Manually removing 1 worker...")
	err = pool.RemoveWorkers(1)
	if err != nil {
		fmt.Printf("Failed to remove workers: %v\n", err)
	}

	time.Sleep(500 * time.Millisecond)

	workerCount, queueLen, maxWorkers = pool.GetStatus()
	fmt.Printf("Status after adjustment: workers=%d, queue length=%d, max workers=%d\n", workerCount, queueLen, maxWorkers)
}

// 任务超时演示
func timeoutTaskDemo(pool *tspool.TaskPool) {
	fmt.Println("Submitting normal task (no timeout)...")
	err := pool.Submit(func() {
		fmt.Println("Normal task started execution")
		time.Sleep(500 * time.Millisecond)
		fmt.Println("Normal task execution completed")
	})
	if err != nil {
		fmt.Printf("Normal task submission failed: %v\n", err)
	}

	fmt.Println("Submitting timed-out task (1 second timeout, actual execution 2 seconds)...")
	err = pool.SubmitWithTimeout(func() {
		fmt.Println("Timed-out task started execution")
		time.Sleep(2 * time.Second)
		fmt.Println("Timed-out task execution completed (this message will not be displayed)")
	}, 1*time.Second)
	if err != nil {
		fmt.Printf("Timed-out task submission failed: %v\n", err)
	}

	time.Sleep(2500 * time.Millisecond)
}

// 旧版高并发任务处理演示（已由新的burstLoadDemo替代）
func legacyHighConcurrencyDemo(pool *tspool.TaskPool) {
	const taskCount = 30
	var wg sync.WaitGroup

	fmt.Printf("Submitting %d legacy concurrent tasks...\n", taskCount)

	start := time.Now()

	for i := 0; i < taskCount; i++ {
		taskID := i
		wg.Add(1)

		err := pool.Submit(func() {
			defer wg.Done()
			// 模拟不同复杂度的任务
			sleepTime := time.Duration(rand.Intn(500)+100) * time.Millisecond
			time.Sleep(sleepTime)

			if taskID%10 == 0 {
				fmt.Printf("Legacy concurrent task %d completed\n", taskID)
			}
		})

		if err != nil {
			fmt.Printf("Task %d submission failed: %v\n", taskID, err)
			wg.Done()
		}
	}

	wg.Wait()
	elapsed := time.Since(start)
	fmt.Printf("All %d legacy tasks completed, time: %v\n", taskCount, elapsed)
}

// 旧版队列满处理演示（已由新的负载测试替代）
func legacyQueueFullDemo(pool *tspool.TaskPool) {
	fmt.Println("Submitting tasks quickly to test queue handling...")

	successCount := 0
	failCount := 0

	// 快速提交60个任务
	for i := 0; i < 60; i++ {
		taskID := i
		err := pool.Submit(func() {
			time.Sleep(50 * time.Millisecond)
			if taskID%20 == 0 {
				fmt.Printf("Queue test task %d completed\n", taskID)
			}
		})

		if err != nil {
			failCount++
			if failCount <= 3 {
				fmt.Printf("Task %d submission failed: %v\n", taskID, err)
			}
		} else {
			successCount++
		}
	}

	fmt.Printf("Queue test results: successfully submitted %d tasks, failed %d tasks\n", successCount, failCount)
}

// 获取协程ID的辅助函数（用于演示）
func getGoroutineID() int {
	// 这是一个简化的协程ID获取方法，实际项目中不推荐使用
	// 仅用于演示目的
	return rand.Intn(10000)
}

// 演示如何创建自定义任务类型
type CustomTask struct {
	ID       int
	Name     string
	Priority int
	Data     interface{}
}

func (ct *CustomTask) Execute() {
	fmt.Printf("Executing custom task: ID=%d, Name=%s, Priority=%d\n", ct.ID, ct.Name, ct.Priority)
	time.Sleep(100 * time.Millisecond)
}

// 批量任务提交辅助函数
func submitBatchTasks(pool *tspool.TaskPool, tasks []func(), batchSize int) error {
	for i := 0; i < len(tasks); i += batchSize {
		end := i + batchSize
		if end > len(tasks) {
			end = len(tasks)
		}

		// 批量提交任务
		for j := i; j < end; j++ {
			if err := pool.Submit(tasks[j]); err != nil {
				return fmt.Errorf("Batch task submission failed at index %d: %v", j, err)
			}
		}

		// 小延迟以避免队列过载
		time.Sleep(10 * time.Millisecond)
	}

	return nil
}

// 详细状态监控
func detailedStatusMonitor(pool *tspool.TaskPool) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if pool.IsClosed() {
				return
			}

			status := pool.GetDetailedStatus()
			fmt.Printf("📊 Pool Status - Workers: %d/%d/%d, Queue: %d/%d, Density: %.2f, Smoothed Queue: %.1f\n",
				status["workerCount"], status["minWorkers"], status["maxWorkers"],
				status["queueLength"], status["queueCapacity"],
				status["queueDensity"], status["smoothedQueueLen"])
		}
	}
}

// 智能自动扩缩容演示
func autoScaleDemo(pool *tspool.TaskPool) {
	fmt.Println("Submitting medium number of tasks to trigger auto-scaling...")

	var wg sync.WaitGroup

	// 快速提交15个任务
	for i := 0; i < 15; i++ {
		taskID := i
		wg.Add(1)

		err := pool.Submit(func() {
			defer wg.Done()
			fmt.Printf("Auto-scaling test task %d executing\n", taskID)
			time.Sleep(time.Duration(rand.Intn(2000)+1000) * time.Millisecond)
		})

		if err != nil {
			fmt.Printf("Task %d submission failed: %v\n", taskID, err)
			wg.Done()
		}

		// 前10个任务快速提交以增加队列密度
		if i < 10 {
			time.Sleep(100 * time.Millisecond)
		} else {
			time.Sleep(500 * time.Millisecond)
		}
	}

	fmt.Println("Waiting for auto-scaling to trigger...")
	time.Sleep(8 * time.Second)

	wg.Wait()
	fmt.Println("Auto-scaling demo tasks completed")
}

// 突发负载演示
func burstLoadDemo(pool *tspool.TaskPool) {
	fmt.Println("Simulating burst high load...")

	var wg sync.WaitGroup

	// 在短时间内提交大量任务
	for i := 0; i < 25; i++ {
		taskID := i
		wg.Add(1)

		err := pool.Submit(func() {
			defer wg.Done()
			fmt.Printf("Burst load task %d executing\n", taskID)
			time.Sleep(time.Duration(rand.Intn(1500)+800) * time.Millisecond)
		})

		if err != nil {
			fmt.Printf("Burst task %d submission failed: %v\n", taskID, err)
			wg.Done()
		}

		time.Sleep(50 * time.Millisecond) // 快速提交
	}

	fmt.Println("Waiting for burst load handling and auto-scaling...")
	time.Sleep(6 * time.Second)

	wg.Wait()
	fmt.Println("Burst load handling completed")
}

// 空闲缩容演示
func idleScaleDownDemo(pool *tspool.TaskPool) {
	fmt.Println("Stopping new task submissions, observing auto-scaling...")

	// 提交少量任务
	for i := 0; i < 3; i++ {
		taskID := i
		err := pool.Submit(func() {
			fmt.Printf("Idle scale-down test task %d executing\n", taskID)
			time.Sleep(500 * time.Millisecond)
		})

		if err != nil {
			fmt.Printf("Task %d submission failed: %v\n", taskID, err)
		}
	}

	fmt.Println("Waiting for idle time to reach threshold and trigger auto-scaling...")
	time.Sleep(15 * time.Second)

	fmt.Println("Idle scale-down demo completed")
}
