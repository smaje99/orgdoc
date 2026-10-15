package worker

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunExecutesWorkerAndValidatesOutputArtifact(t *testing.T) {
	capability := fixtureCapability(t, "success.sh", 1024, time.Second)
	runner := NewRunner(ResourceLimits{ResourceClassCPU: 1}, time.Second)
	request := WorkerRequest{ProtocolVersion: 1, JobID: "job-1", OperationID: "consulting", BrandID: "henko", DocumentType: "invoice", Input: map[string]any{"amount": 12}, ArtifactDir: filepath.Join(t.TempDir(), "job-1")}

	result, err := runner.Run(context.Background(), capability, request)
	if err != nil {
		t.Fatalf("Run() error = %v; result = %#v", err, result)
	}
	if result.Status != WorkerStatusSucceeded || len(result.Outputs) != 1 || result.Outputs[0].Path != "result.txt" {
		t.Fatalf("Run() result = %#v", result)
	}
	if _, err := os.Stat(filepath.Join(request.ArtifactDir, "result.txt")); err != nil {
		t.Fatalf("worker output is absent: %v", err)
	}
}

func TestRunRejectsMalformedWorkerJSON(t *testing.T) {
	result, err := runFixture(t, "malformed-json.sh", 1024, time.Second)
	if err == nil || result.Status != WorkerStatusFailed || !hasWorkerDiagnostic(result, "invalid_json") {
		t.Fatalf("Run() = (%#v, %v), want invalid JSON failure", result, err)
	}
}

func TestRunTimesOutAndCancelsWorker(t *testing.T) {
	result, err := runFixture(t, "timeout.sh", 1024, 50*time.Millisecond)
	if err == nil || result.Status != WorkerStatusFailed || !hasWorkerDiagnostic(result, "timeout") {
		t.Fatalf("Run() = (%#v, %v), want timeout failure", result, err)
	}
}

func TestRunReportsNonzeroWorkerExit(t *testing.T) {
	result, err := runFixture(t, "nonzero-exit.sh", 1024, time.Second)
	if err == nil || result.Status != WorkerStatusFailed || !hasWorkerDiagnostic(result, "worker_exit") {
		t.Fatalf("Run() = (%#v, %v), want nonzero-exit failure", result, err)
	}
	if !strings.Contains(result.Diagnostics[0].Message, "fixture failure") {
		t.Fatalf("worker stderr was not captured: %#v", result.Diagnostics)
	}
}

func TestRunRejectsOversizedWorkerOutput(t *testing.T) {
	result, err := runFixture(t, "oversized-output.sh", 64, time.Second)
	if err == nil || result.Status != WorkerStatusFailed || !hasWorkerDiagnostic(result, "output_limit") {
		t.Fatalf("Run() = (%#v, %v), want output-limit failure", result, err)
	}
}

func TestRunRejectsArtifactPathTraversal(t *testing.T) {
	result, err := runFixture(t, "path-traversal.sh", 1024, time.Second)
	if err == nil || result.Status != WorkerStatusFailed || !hasWorkerDiagnostic(result, "invalid_artifact_path") {
		t.Fatalf("Run() = (%#v, %v), want artifact traversal failure", result, err)
	}
}

func TestRunDoesNotTouchExistingArtifactDirectory(t *testing.T) {
	capability := fixtureCapability(t, "success.sh", 1024, time.Second)
	runner := NewRunner(ResourceLimits{ResourceClassCPU: 1}, time.Second)
	artifactDir := filepath.Join(t.TempDir(), "prior")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prior := filepath.Join(artifactDir, "keep.txt")
	if err := os.WriteFile(prior, []byte("preserve"), 0o644); err != nil {
		t.Fatal(err)
	}
	request := WorkerRequest{ProtocolVersion: 1, JobID: "job-1", ArtifactDir: artifactDir}

	result, err := runner.Run(context.Background(), capability, request)
	content, readErr := os.ReadFile(prior)
	if err == nil || result.Status != WorkerStatusFailed || readErr != nil || string(content) != "preserve" {
		t.Fatalf("Run() = (%#v, %v), prior content = %q, read error = %v", result, err, content, readErr)
	}
}

func TestRunInvokesExecutableDirectlyWithoutShellEvaluation(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	capability := Capability{ID: "direct", EntryPoint: []string{"/bin/echo", "; touch " + marker}, ResourceClass: ResourceClassCPU, MaxOutputBytes: 1024, TimeoutSeconds: 1}
	request := WorkerRequest{ProtocolVersion: 1, JobID: "job-1", ArtifactDir: filepath.Join(t.TempDir(), "job")}
	result, _ := NewRunner(ResourceLimits{ResourceClassCPU: 1}, time.Second).Run(context.Background(), capability, request)
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("shell expression was evaluated; marker stat error = %v", err)
	}
	if result.Status != WorkerStatusFailed {
		t.Fatalf("echo did not fail the worker protocol: %#v", result)
	}
}

func TestResourceSemaphoresAreKeyedByClassAndBounded(t *testing.T) {
	runner := NewRunner(ResourceLimits{ResourceClassCPU: 1, ResourceClassGPU: 1}, time.Second)
	firstRelease, err := runner.acquire(context.Background(), ResourceClassCPU)
	if err != nil {
		t.Fatal(err)
	}
	defer firstRelease()
	if release, err := runner.acquire(context.Background(), ResourceClassGPU); err != nil {
		t.Fatalf("GPU resource class was blocked by CPU permit: %v", err)
	} else {
		release()
	}

	blocked := make(chan struct{})
	go func() {
		defer close(blocked)
		release, err := runner.acquire(context.Background(), ResourceClassCPU)
		if err == nil {
			release()
		}
	}()
	select {
	case <-blocked:
		t.Fatal("second CPU job acquired a permit before the first released it")
	case <-time.After(25 * time.Millisecond):
	}
	firstRelease()
	select {
	case <-blocked:
	case <-time.After(time.Second):
		t.Fatal("waiting CPU job did not resume after a permit was released")
	}
}

func fixtureCapability(t *testing.T, workerName string, outputLimit int64, timeout time.Duration) Capability {
	t.Helper()
	path := filepath.Join("..", "..", "tests", "fixtures", "workers", workerName)
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return Capability{ID: "fixture", EntryPoint: []string{absolute}, ResourceClass: ResourceClassCPU, MaxOutputBytes: outputLimit, TimeoutSeconds: int(timeout.Seconds())}
}

func runFixture(t *testing.T, workerName string, outputLimit int64, timeout time.Duration) (WorkerResult, error) {
	t.Helper()
	capability := fixtureCapability(t, workerName, outputLimit, timeout)
	request := WorkerRequest{ProtocolVersion: 1, JobID: "job-1", ArtifactDir: filepath.Join(t.TempDir(), "job-1")}
	return NewRunner(ResourceLimits{ResourceClassCPU: 1}, timeout).Run(context.Background(), capability, request)
}

func hasWorkerDiagnostic(result WorkerResult, code string) bool {
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
