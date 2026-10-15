package scheduler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/smaje/orgdoc/internal/worker"
)

func BenchmarkNoopWorkerBatch(b *testing.B) {
	script, err := filepath.Abs(filepath.Join("..", "..", "benchmarks", "fixtures", "noop-worker.sh"))
	if err != nil {
		b.Fatal(err)
	}
	if err := os.Chmod(script, 0o755); err != nil {
		b.Fatal(err)
	}
	for _, workers := range []int{1, 4} {
		b.Run(fmt.Sprintf("workers_%d", workers), func(b *testing.B) {
			for iteration := 0; iteration < b.N; iteration++ {
				artifactRoot := filepath.Join(b.TempDir(), fmt.Sprintf("run-%d", iteration))
				runner := worker.NewRunner(worker.ResourceLimits{worker.ResourceClassCPU: workers}, 5*time.Second)
				jobs := make([]Job, 100)
				for index := range jobs {
					jobID := fmt.Sprintf("job-%03d", index)
					jobs[index] = Job{ID: jobID, ResourceClass: worker.ResourceClassCPU, Run: func(ctx context.Context) (any, error) {
						return runner.Run(ctx, worker.Capability{ID: "benchmark-noop", EntryPoint: []string{script}, ResourceClass: worker.ResourceClassCPU, TimeoutSeconds: 5, MaxOutputBytes: 1024}, worker.WorkerRequest{
							ProtocolVersion: worker.ProtocolVersion, JobID: jobID, ArtifactDir: filepath.Join(artifactRoot, jobID),
						})
					}}
				}
				limits := ResourceLimits{MaxWorkers: workers, ByClass: map[worker.ResourceClass]int{worker.ResourceClassCPU: workers}}
				b.StartTimer()
				result := RunBatch(context.Background(), jobs, limits)
				b.StopTimer()
				if result.Status != BatchStatusSucceeded {
					b.Fatalf("RunBatch() = %s", result.Status)
				}
			}
			b.ReportMetric(100*float64(b.N)/b.Elapsed().Seconds(), "jobs/s")
		})
	}
}

func BenchmarkMixedResourceClassNoopBatch(b *testing.B) {
	script, err := filepath.Abs(filepath.Join("..", "..", "benchmarks", "fixtures", "noop-worker.sh"))
	if err != nil {
		b.Fatal(err)
	}
	if err := os.Chmod(script, 0o755); err != nil {
		b.Fatal(err)
	}
	limits := ResourceLimits{
		MaxWorkers: 4,
		ByClass: map[worker.ResourceClass]int{
			worker.ResourceClassCPU:    4,
			worker.ResourceClassGPU:    1,
			worker.ResourceClassMemory: 1,
		},
	}
	for iteration := 0; iteration < b.N; iteration++ {
		artifactRoot := filepath.Join(b.TempDir(), fmt.Sprintf("mixed-%d", iteration))
		runner := worker.NewRunner(limits.ByClass, 5*time.Second)
		jobs := make([]Job, 100)
		for index := range jobs {
			class := worker.ResourceClassCPU
			if index >= 50 && index < 75 {
				class = worker.ResourceClassGPU
			} else if index >= 75 {
				class = worker.ResourceClassMemory
			}
			jobID := fmt.Sprintf("mixed-%03d", index)
			jobs[index] = Job{ID: jobID, ResourceClass: class, Run: func(ctx context.Context) (any, error) {
				return runner.Run(ctx, worker.Capability{ID: "benchmark-noop", EntryPoint: []string{script}, ResourceClass: class, TimeoutSeconds: 5, MaxOutputBytes: 1024}, worker.WorkerRequest{
					ProtocolVersion: worker.ProtocolVersion, JobID: jobID, ArtifactDir: filepath.Join(artifactRoot, jobID),
				})
			}}
		}
		b.StartTimer()
		result := RunBatch(context.Background(), jobs, limits)
		b.StopTimer()
		if result.Status != BatchStatusSucceeded {
			b.Fatalf("RunBatch() = %s", result.Status)
		}
	}
	b.ReportMetric(100*float64(b.N)/b.Elapsed().Seconds(), "jobs/s")
}
