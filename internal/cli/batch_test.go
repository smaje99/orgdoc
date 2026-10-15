package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/smaje/orgdoc/internal/scheduler"
)

func TestBatchRunKeepsOrderContinuesFailuresAndProtectsRetries(t *testing.T) {
	organization := writeDocumentOrganization(t)
	writeBatchWorker(t, organization, `#!/bin/sh
request=$(cat)
job_id=$(printf '%s\n' "$request" | sed -n 's/.*"job_id":"\([^"]*\)".*/\1/p')
case "$job_id" in
  slow) sleep 0.05 ;;
  sleepy) sleep 2 ;;
  fail) printf '{"protocol_version":1,"job_id":"%s","status":"failed","outputs":[],"diagnostics":[{"code":"fixture_failure","message":"intentional"}]}\n' "$job_id"; exit 0 ;;
esac
printf 'invoice for %s\n' "$job_id" > invoice.txt
printf '{"protocol_version":1,"job_id":"%s","status":"succeeded","outputs":[{"path":"invoice.txt","media_type":"text/plain"}],"diagnostics":[]}\n' "$job_id"
`)
	outputRoot := filepath.Join(t.TempDir(), "runs")
	batchPath := writeBatchFile(t, organization, outputRoot, []map[string]any{
		batchJob("fast"), batchJob("slow"), batchJob("fail"), batchJob("last"),
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--format", "json", "batch", "run", batchPath, "--max-workers", "3"}, strings.NewReader(""), &stdout, &stderr)
	var result scheduler.BatchResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid batch JSON: %v: %s; stderr=%s", err, stdout.String(), stderr.String())
	}
	if code != ExitError || result.Status != scheduler.BatchStatusPartialFailure || len(result.Results) != 4 {
		t.Fatalf("batch result = (%d, %#v, %q)", code, result, stderr.String())
	}
	wantIDs := []string{"fast", "slow", "fail", "last"}
	wantStatuses := []scheduler.JobStatus{scheduler.JobStatusSucceeded, scheduler.JobStatusSucceeded, scheduler.JobStatusFailed, scheduler.JobStatusSucceeded}
	for index := range result.Results {
		if result.Results[index].JobID != wantIDs[index] || result.Results[index].Status != wantStatuses[index] {
			t.Fatalf("result[%d] = %#v; want %s/%s", index, result.Results[index], wantIDs[index], wantStatuses[index])
		}
	}
	for _, jobID := range []string{"fast", "slow", "last"} {
		artifact := filepath.Join(outputRoot, jobID, "invoice.txt")
		contents, err := os.ReadFile(artifact)
		if err != nil || !strings.Contains(string(contents), jobID) {
			t.Fatalf("job %q artifact was not isolated: contents=%q error=%v", jobID, contents, err)
		}
	}
	priorPath := filepath.Join(outputRoot, "fast", "invoice.txt")
	prior, err := os.ReadFile(priorPath)
	if err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"--format", "json", "batch", "run", batchPath}, strings.NewReader(""), &stdout, &stderr)
	if code != ExitError {
		t.Fatalf("retry with existing job artifact exit code = %d, want %d", code, ExitError)
	}
	after, err := os.ReadFile(priorPath)
	if err != nil || string(after) != string(prior) {
		t.Fatalf("retry changed a completed artifact: before=%q after=%q error=%v", prior, after, err)
	}
}

func TestBatchRunCancellationStopsQueuedJobs(t *testing.T) {
	organization := writeDocumentOrganization(t)
	writeBatchWorker(t, organization, `#!/bin/sh
request=$(cat)
job_id=$(printf '%s\n' "$request" | sed -n 's/.*"job_id":"\([^"]*\)".*/\1/p')
sleep 2
printf '{"protocol_version":1,"job_id":"%s","status":"succeeded","outputs":[],"diagnostics":[]}\n' "$job_id"
`)
	batchPath := writeBatchFile(t, organization, filepath.Join(t.TempDir(), "runs"), []map[string]any{batchJob("sleepy"), batchJob("queued")})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	var stdout, stderr bytes.Buffer
	code := RunContext(ctx, []string{"--format", "json", "batch", "run", batchPath, "--max-workers", "1"}, strings.NewReader(""), &stdout, &stderr)
	var result scheduler.BatchResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid cancellation JSON: %v: %s; stderr=%s", err, stdout.String(), stderr.String())
	}
	if code != ExitError || result.Status != scheduler.BatchStatusCanceled || len(result.StartedJobIDs) != 1 || len(result.Results) != 2 {
		t.Fatalf("canceled batch = (%d, %#v, %q)", code, result, stderr.String())
	}
}

func TestBatchRunRequiresPositiveMaxWorkers(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"batch", "run", "jobs.json", "--max-workers", "0", "--format", "json"}, strings.NewReader(""), &stdout, &stderr)
	if code != ExitUsage || !strings.Contains(stderr.String(), "positive integer") {
		t.Fatalf("Run() = (%d, %q, %q), want a positive worker count usage error", code, stdout.String(), stderr.String())
	}
}

func TestLoadBatchDefinitionRejectsTrailingJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "batch.json")
	contents := `{"schema_version":1,"organization":"org","output_root":"runs","jobs":[]} {"extra":true}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadBatchDefinition(path); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("loadBatchDefinition() error = %v, want trailing JSON error", err)
	}
}

func TestBatchFailureKeepsRequestedJobID(t *testing.T) {
	organization := writeDocumentOrganization(t)
	batchPath := writeBatchFile(t, organization, filepath.Join(t.TempDir(), "runs"), []map[string]any{
		{"job_id": "bad-input-job", "operation": "consulting", "document_type": "invoice", "input": "not-an-object"},
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--format", "json", "batch", "run", batchPath}, strings.NewReader(""), &stdout, &stderr)
	var result scheduler.BatchResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid batch JSON: %v: %s; stderr=%s", err, stdout.String(), stderr.String())
	}
	if code != ExitError || len(result.Results) != 1 || result.Results[0].JobID != "bad-input-job" {
		t.Fatalf("batch result = (%d, %#v, %q), want failure for bad-input-job", code, result, stderr.String())
	}
}

func writeBatchWorker(t *testing.T, organization, contents string) {
	t.Helper()
	path := filepath.Join(organization, "capabilities", "invoice-pdf", "renderer.sh")
	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
}

func writeBatchFile(t *testing.T, organization, outputRoot string, jobs []map[string]any) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "batch.json")
	encoded, err := json.Marshal(map[string]any{
		"schema_version": 1,
		"organization":   organization,
		"output_root":    outputRoot,
		"jobs":           jobs,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func batchJob(id string) map[string]any {
	return map[string]any{
		"job_id":        id,
		"operation":     "consulting",
		"document_type": "invoice",
		"input":         map[string]any{"invoice_number": "F-1", "grand_total": 42},
	}
}
