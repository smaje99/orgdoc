package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/smaje99/orgdoc/internal/worker"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusSucceeded JobStatus = "succeeded"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCanceled  JobStatus = "canceled"
)

type BatchStatus string

const (
	BatchStatusSucceeded      BatchStatus = "succeeded"
	BatchStatusPartialFailure BatchStatus = "partial_failure"
	BatchStatusFailed         BatchStatus = "failed"
	BatchStatusCanceled       BatchStatus = "canceled"
)

type Job struct {
	ID            string
	ResourceClass worker.ResourceClass
	Run           func(context.Context) (any, error)
}

type JobResult struct {
	JobID  string    `json:"job_id"`
	Status JobStatus `json:"status"`
	Value  any       `json:"value,omitempty"`
	Error  string    `json:"error,omitempty"`
}

type BatchResult struct {
	Status        BatchStatus `json:"status"`
	StartedJobIDs []string    `json:"started_job_ids"`
	Results       []JobResult `json:"results"`
	StartedAt     string      `json:"started_at"`
	CompletedAt   string      `json:"completed_at"`
}

type completion struct {
	index  int
	result JobResult
}

// RunBatch executes independent work under both a global limit and per-class limits.
// Results always retain the input order, regardless of completion order.
func RunBatch(ctx context.Context, jobs []Job, limits ResourceLimits) BatchResult {
	startedAt := time.Now().UTC()
	result := BatchResult{Status: BatchStatusSucceeded, StartedJobIDs: []string{}, Results: make([]JobResult, len(jobs)), StartedAt: startedAt.Format(time.RFC3339Nano)}
	for index, job := range jobs {
		result.Results[index] = JobResult{JobID: job.ID, Status: JobStatusPending}
	}
	if duplicate := duplicateID(jobs); duplicate != "" {
		for index := range result.Results {
			result.Results[index].Status = JobStatusFailed
			result.Results[index].Error = fmt.Sprintf("duplicate job_id %q in batch", duplicate)
		}
		result.Status = BatchStatusFailed
		result.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
		return result
	}
	if len(jobs) == 0 {
		result.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
		return result
	}

	limits = normalizeLimits(limits)
	activeByClass := make(map[worker.ResourceClass]int)
	active, finishedCount := 0, 0
	finished := make(chan completion, len(jobs))
	for index, job := range jobs {
		if job.ID == "" {
			result.Results[index].Status = JobStatusFailed
			result.Results[index].Error = "job_id is required"
			finishedCount++
			continue
		}
		if classLimit := limits.ByClass[job.ResourceClass]; classLimit <= 0 {
			result.Results[index].Status = JobStatusFailed
			result.Results[index].Error = fmt.Sprintf("no positive concurrency limit for resource class %q", job.ResourceClass)
			finishedCount++
			continue
		}
		if job.Run == nil {
			result.Results[index].Status = JobStatusFailed
			result.Results[index].Error = "job has no run function"
			finishedCount++
		}
	}

	for finishedCount < len(jobs) {
		if ctx.Err() != nil {
			for index := range result.Results {
				if result.Results[index].Status == JobStatusPending {
					result.Results[index].Status = JobStatusCanceled
					result.Results[index].Error = "batch canceled before the job started"
					finishedCount++
				}
			}
		}

		launched := false
		if ctx.Err() == nil {
			for active < limits.MaxWorkers {
				index := nextRunnable(jobs, result.Results, activeByClass, limits.ByClass)
				if index < 0 {
					break
				}
				job := jobs[index]
				result.Results[index].Status = JobStatusRunning
				active++
				activeByClass[job.ResourceClass]++
				result.StartedJobIDs = append(result.StartedJobIDs, job.ID)
				launched = true
				go func(index int, job Job) {
					value, err := runSafely(ctx, job.Run)
					item := JobResult{JobID: job.ID, Value: value, Status: JobStatusSucceeded}
					if err != nil {
						item.Status, item.Error = JobStatusFailed, err.Error()
					}
					if ctx.Err() != nil {
						item.Status = JobStatusCanceled
						if item.Error == "" {
							item.Error = ctx.Err().Error()
						}
					}
					finished <- completion{index: index, result: item}
				}(index, job)
			}
		}

		if active == 0 {
			if finishedCount == len(jobs) {
				break
			}
			for index := range result.Results {
				if result.Results[index].Status == JobStatusPending {
					result.Results[index].Status = JobStatusFailed
					result.Results[index].Error = "job could not be scheduled under the configured resource limits"
					finishedCount++
				}
			}
			continue
		}

		if launched {
			continue
		}
		select {
		case completed := <-finished:
			job := jobs[completed.index]
			active--
			activeByClass[job.ResourceClass]--
			result.Results[completed.index] = completed.result
			finishedCount++
		case <-ctx.Done():
			// The next loop marks queued jobs canceled and waits for active work to exit.
		}
	}
	result.Status = batchStatus(ctx, result.Results)
	result.CompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return result
}

func nextRunnable(jobs []Job, results []JobResult, active map[worker.ResourceClass]int, limits map[worker.ResourceClass]int) int {
	for index, job := range jobs {
		if results[index].Status == JobStatusPending && active[job.ResourceClass] < limits[job.ResourceClass] && job.Run != nil && job.ID != "" {
			return index
		}
	}
	return -1
}

func duplicateID(jobs []Job) string {
	seen := make(map[string]struct{}, len(jobs))
	for _, job := range jobs {
		if job.ID == "" {
			continue
		}
		if _, exists := seen[job.ID]; exists {
			return job.ID
		}
		seen[job.ID] = struct{}{}
	}
	return ""
}

func runSafely(ctx context.Context, run func(context.Context) (any, error)) (value any, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("job panicked: %v", recovered)
		}
	}()
	return run(ctx)
}

func batchStatus(ctx context.Context, results []JobResult) BatchStatus {
	if ctx.Err() != nil {
		return BatchStatusCanceled
	}
	succeeded, failed := 0, 0
	for _, result := range results {
		switch result.Status {
		case JobStatusFailed:
			failed++
		case JobStatusSucceeded:
			succeeded++
		}
	}
	if failed == 0 {
		return BatchStatusSucceeded
	}
	if succeeded > 0 {
		return BatchStatusPartialFailure
	}
	return BatchStatusFailed
}
