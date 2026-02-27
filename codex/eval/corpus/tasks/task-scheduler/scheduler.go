// Package scheduler implements a periodic task scheduler.
package scheduler

import (
	"context"
	"errors"
	"time"
)

// ErrDuplicateJob is returned when adding a job with a name that already exists.
var ErrDuplicateJob = errors.New("job with this name already exists")

// Job defines a periodic task to be executed by the scheduler.
type Job struct {
	Name     string
	Interval time.Duration
	Fn       func(ctx context.Context) error
}

// jobEntry holds a registered job and its cancellation handle.
type jobEntry struct {
	job  Job
	done chan struct{} // closed to signal the job goroutine to exit
}

// Scheduler manages periodic job execution.
type Scheduler struct {
	// TODO: add fields
	//   - jobs map[string]*jobEntry — registered jobs
	//   - mu sync.Mutex — protects jobs map and running state
	//   - running bool — whether Start() has been called
	//   - wg sync.WaitGroup — tracks active job goroutines for graceful Stop
	//   - stopped bool — prevents double-stop
}

// NewScheduler creates a new periodic task scheduler.
func NewScheduler() *Scheduler {
	// TODO: initialize fields (jobs map, etc.)
	return &Scheduler{}
}

// Add registers a job with the scheduler.
// Returns ErrDuplicateJob if a job with the same Name is already registered.
// If the scheduler is already running, the job begins executing immediately.
func (s *Scheduler) Add(job Job) error {
	// TODO: implement
	//   1. Lock mutex
	//   2. Check for duplicate name in jobs map
	//   3. Create jobEntry with done channel
	//   4. Store in jobs map
	//   5. If running, start the job goroutine (call s.runJob)
	return ErrDuplicateJob
}

// Remove removes a registered job by name.
// Returns true if the job was found and removed, false otherwise.
// If the scheduler is running, the job's goroutine must be stopped.
func (s *Scheduler) Remove(name string) bool {
	// TODO: implement
	//   1. Lock mutex
	//   2. Look up job in map
	//   3. If found and running, close done channel to stop goroutine
	//   4. Delete from map
	//   5. Return whether it was found
	return false
}

// Start begins executing all registered jobs at their configured intervals.
// Each job runs in its own goroutine with a time.Ticker.
func (s *Scheduler) Start() {
	// TODO: implement
	//   1. Lock mutex
	//   2. Set running = true
	//   3. For each job in the map, launch its goroutine (call s.runJob)
}

// Stop signals all job goroutines to stop and waits for in-flight executions
// to finish. It respects the context deadline and is safe to call multiple times.
func (s *Scheduler) Stop(ctx context.Context) error {
	// TODO: implement
	//   1. Lock mutex, check if already stopped (idempotent)
	//   2. Close done channel for each job to signal goroutines to exit
	//   3. Set running = false, stopped = true
	//   4. Unlock mutex
	//   5. Wait for wg (all goroutines) with context deadline:
	//      - Launch goroutine that calls wg.Wait() and signals a channel
	//      - Select on that channel vs ctx.Done()
	//   6. Return nil or ctx.Err()
	return nil
}

// runJob launches a goroutine for the given job entry that ticks at the job's interval.
// It must:
//   - Use time.NewTicker for periodic scheduling
//   - Check the done channel to know when to exit
//   - Prevent overlapping executions (skip tick if previous execution still running)
//   - Track itself in the WaitGroup so Stop can wait for it
func (s *Scheduler) runJob(entry *jobEntry) {
	// TODO: implement
	//   s.wg.Add(1)
	//   go func() {
	//       defer s.wg.Done()
	//       ticker := time.NewTicker(entry.job.Interval)
	//       defer ticker.Stop()
	//       var running int32 // atomic flag for overlap prevention
	//       for {
	//           select {
	//           case <-entry.done:
	//               return
	//           case <-ticker.C:
	//               // Skip if previous execution still running
	//               // Execute entry.job.Fn with context
	//           }
	//       }
	//   }()
}
