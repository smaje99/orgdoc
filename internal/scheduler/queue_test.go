package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/smaje99/orgdoc/internal/worker"
)

func TestRunBatchBoundsConcurrencyByResourceClass(t *testing.T) {
	var cpuActive, gpuActive, memoryActive atomic.Int32
	var cpuMax, gpuMax, memoryMax atomic.Int32
	jobs := make([]Job, 0, 18)
	for index := 0; index < 10; index++ {
		jobs = append(jobs, countedJob(fmt.Sprintf("cpu-%02d", index), worker.ResourceClassCPU, &cpuActive, &cpuMax))
	}
	for index := 0; index < 4; index++ {
		jobs = append(jobs, countedJob(fmt.Sprintf("gpu-%02d", index), worker.ResourceClassGPU, &gpuActive, &gpuMax))
		jobs = append(jobs, countedJob(fmt.Sprintf("memory-%02d", index), worker.ResourceClassMemory, &memoryActive, &memoryMax))
	}

	result := RunBatch(context.Background(), jobs, ResourceLimits{
		MaxWorkers: 6,
		ByClass:    map[worker.ResourceClass]int{worker.ResourceClassCPU: 4, worker.ResourceClassGPU: 1, worker.ResourceClassMemory: 1},
	})
	if result.Status != BatchStatusSucceeded || len(result.Results) != len(jobs) {
		t.Fatalf("RunBatch() = %#v", result)
	}
	if cpuMax.Load() > 4 || gpuMax.Load() > 1 || memoryMax.Load() > 1 {
		t.Fatalf("resource limits exceeded: CPU=%d GPU=%d memory=%d", cpuMax.Load(), gpuMax.Load(), memoryMax.Load())
	}
	if cpuMax.Load() < 2 || gpuMax.Load() != 1 || memoryMax.Load() != 1 {
		t.Fatalf("jobs did not run with expected parallelism: CPU=%d GPU=%d memory=%d", cpuMax.Load(), gpuMax.Load(), memoryMax.Load())
	}
}

func TestRunBatchReturnsResultsInInputOrder(t *testing.T) {
	delays := []time.Duration{40 * time.Millisecond, 5 * time.Millisecond, 20 * time.Millisecond}
	jobs := make([]Job, len(delays))
	for index, delay := range delays {
		index, delay := index, delay
		jobs[index] = Job{ID: fmt.Sprintf("job-%d", index), ResourceClass: worker.ResourceClassCPU, Run: func(context.Context) (any, error) {
			time.Sleep(delay)
			return index, nil
		}}
	}
	result := RunBatch(context.Background(), jobs, ResourceLimits{MaxWorkers: 3, ByClass: map[worker.ResourceClass]int{worker.ResourceClassCPU: 3}})
	for index, item := range result.Results {
		if item.JobID != jobs[index].ID || item.Value != index {
			t.Fatalf("results[%d] = %#v; results must preserve submission order", index, item)
		}
	}
}

func TestRunBatchContinuesAfterJobFailure(t *testing.T) {
	jobs := []Job{
		{ID: "first", ResourceClass: worker.ResourceClassCPU, Run: func(context.Context) (any, error) { return "ok", nil }},
		{ID: "bad", ResourceClass: worker.ResourceClassCPU, Run: func(context.Context) (any, error) { return nil, errors.New("fixture failure") }},
		{ID: "last", ResourceClass: worker.ResourceClassCPU, Run: func(context.Context) (any, error) { return "still ran", nil }},
	}
	result := RunBatch(context.Background(), jobs, ResourceLimits{MaxWorkers: 2, ByClass: map[worker.ResourceClass]int{worker.ResourceClassCPU: 2}})
	if result.Status != BatchStatusPartialFailure {
		t.Fatalf("batch status = %q, want partial_failure", result.Status)
	}
	if result.Results[0].Status != JobStatusSucceeded || result.Results[1].Status != JobStatusFailed || result.Results[2].Status != JobStatusSucceeded {
		t.Fatalf("unexpected per-job results: %#v", result.Results)
	}
}

func TestRunBatchCancellationStopsQueuedAndActiveWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 10)
	jobs := make([]Job, 10)
	for index := range jobs {
		jobs[index] = Job{ID: fmt.Sprintf("job-%d", index), ResourceClass: worker.ResourceClassCPU, Run: func(ctx context.Context) (any, error) {
			started <- struct{}{}
			<-ctx.Done()
			return nil, ctx.Err()
		}}
	}
	done := make(chan BatchResult, 1)
	go func() {
		done <- RunBatch(ctx, jobs, ResourceLimits{MaxWorkers: 2, ByClass: map[worker.ResourceClass]int{worker.ResourceClassCPU: 2}})
	}()
	<-started
	<-started
	cancel()
	select {
	case result := <-done:
		if result.Status != BatchStatusCanceled || len(result.Results) != len(jobs) {
			t.Fatalf("RunBatch() after cancellation = %#v", result)
		}
		if len(result.StartedJobIDs) != 2 {
			t.Fatalf("started jobs after cancellation = %v; queued jobs must not start", result.StartedJobIDs)
		}
	case <-time.After(time.Second):
		t.Fatal("RunBatch did not return after cancellation")
	}
}

func TestRunBatchRejectsDuplicateIDsWithoutStartingJobs(t *testing.T) {
	var started atomic.Int32
	job := func() Job {
		return Job{ID: "same", ResourceClass: worker.ResourceClassCPU, Run: func(context.Context) (any, error) { started.Add(1); return nil, nil }}
	}
	result := RunBatch(context.Background(), []Job{job(), job()}, ResourceLimits{MaxWorkers: 2, ByClass: map[worker.ResourceClass]int{worker.ResourceClassCPU: 2}})
	if result.Status != BatchStatusFailed || started.Load() != 0 || result.Results[0].Status != JobStatusFailed || result.Results[1].Status != JobStatusFailed {
		t.Fatalf("duplicate job IDs should fail before execution: %#v; started=%d", result, started.Load())
	}
}

func TestRunBatchKeepsArtifactsIsolated(t *testing.T) {
	temp := t.TempDir()
	var mu sync.Mutex
	seen := map[string]bool{}
	jobs := make([]Job, 4)
	for index := range jobs {
		id := fmt.Sprintf("artifact-%d", index)
		jobs[index] = Job{ID: id, ResourceClass: worker.ResourceClassCPU, Run: func(context.Context) (any, error) {
			mu.Lock()
			defer mu.Unlock()
			if seen[id] {
				return nil, fmt.Errorf("artifact path reused: %s", id)
			}
			seen[id] = true
			return temp + "/" + id, nil
		}}
	}
	result := RunBatch(context.Background(), jobs, ResourceLimits{MaxWorkers: 4, ByClass: map[worker.ResourceClass]int{worker.ResourceClassCPU: 4}})
	if result.Status != BatchStatusSucceeded || len(seen) != len(jobs) {
		t.Fatalf("artifact isolation failed: %#v; seen=%v", result, seen)
	}
	for index, item := range result.Results {
		if item.Value != fmt.Sprintf("%s/artifact-%d", temp, index) {
			t.Fatalf("result %d reused another job's artifact path: %#v", index, item)
		}
	}
}

func countedJob(id string, class worker.ResourceClass, active, maximum *atomic.Int32) Job {
	return Job{ID: id, ResourceClass: class, Run: func(context.Context) (any, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for observed := maximum.Load(); current > observed && !maximum.CompareAndSwap(observed, current); observed = maximum.Load() {
		}
		time.Sleep(15 * time.Millisecond)
		return id, nil
	}}
}
