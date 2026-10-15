package document

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/smaje/orgdoc/internal/orgconfig"
	"github.com/smaje/orgdoc/internal/worker"
)

func TestRenderDocumentPersistsRunManifestAndLeavesPriorRunUntouched(t *testing.T) {
	_, profile := rendererFixture(t)
	outputRoot := filepath.Join(t.TempDir(), "artifacts")
	runner := worker.NewRunner(worker.ResourceLimits{worker.ResourceClassCPU: 1}, time.Second)
	request := RenderRequest{Profile: profile, Input: map[string]any{"invoice_number": "F-1", "grand_total": 125.0}, InputReferences: []string{"input.json"}, OutputRoot: outputRoot, JobID: "run-001", Runner: runner}
	result, err := RenderDocument(context.Background(), request)
	if err != nil {
		t.Fatalf("RenderDocument() error = %v; result=%#v", err, result)
	}
	if result.Status != RunStatusSucceeded || len(result.Outputs) != 1 {
		t.Fatalf("result = %#v", result)
	}
	manifest, err := os.ReadFile(result.ManifestPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"\"brand\": \"2.1.0\"", "\"capability_version\": \"1.2.0\"", "\"input_references\": [", "\"status\": \"succeeded\""} {
		if !strings.Contains(string(manifest), expected) {
			t.Fatalf("manifest missing %q: %s", expected, manifest)
		}
	}
	prior := filepath.Join(result.ArtifactDir, "invoice.txt")
	if err := os.WriteFile(prior, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	request.Input = map[string]any{"invoice_number": "F-2", "grand_total": 250.0}
	if _, err := RenderDocument(context.Background(), request); err == nil {
		t.Fatal("rerunning an existing job ID unexpectedly replaced prior artifacts")
	}
	content, err := os.ReadFile(prior)
	if err != nil || string(content) != "keep" {
		t.Fatalf("prior artifact changed: %q, %v", content, err)
	}
}

func TestRenderDocumentRejectsInvalidInputBeforeStartingRenderer(t *testing.T) {
	root, profile := rendererFixture(t)
	marker := filepath.Join(root, "renderer-was-started")
	if err := os.WriteFile(filepath.Join(root, "capabilities", "invoice-pdf", "renderer.sh"), []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	outputRoot := filepath.Join(t.TempDir(), "runs")
	request := RenderRequest{Profile: profile, Input: map[string]any{"invoice_number": 7}, OutputRoot: outputRoot, JobID: "invalid"}
	if _, err := RenderDocument(context.Background(), request); err == nil {
		t.Fatal("invalid input unexpectedly rendered")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("renderer ran before input validation, marker error = %v", err)
	}
	if _, err := os.Stat(outputRoot); !os.IsNotExist(err) {
		t.Fatalf("invalid input created output root, stat error = %v", err)
	}
}

func TestRenderDocumentPersistsFailedStageManifest(t *testing.T) {
	root, profile := rendererFixture(t)
	workerPath := filepath.Join(root, "capabilities", "invoice-pdf", "renderer.sh")
	if err := os.WriteFile(workerPath, []byte("#!/bin/sh\nprintf 'render failed\\n' >&2\nexit 7\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	request := RenderRequest{Profile: profile, Input: map[string]any{"invoice_number": "F-1", "grand_total": 125.0}, OutputRoot: filepath.Join(t.TempDir(), "runs"), JobID: "failed-run"}
	result, err := RenderDocument(context.Background(), request)
	if err == nil || result.Status != RunStatusFailed {
		t.Fatalf("RenderDocument() = (%#v, %v), want failed", result, err)
	}
	manifest, readErr := os.ReadFile(result.ManifestPath)
	if readErr != nil {
		t.Fatalf("failed run manifest absent: %v", readErr)
	}
	if !strings.Contains(string(manifest), `"name": "render"`) || !strings.Contains(string(manifest), `"status": "failed"`) || !strings.Contains(string(manifest), "render failed") {
		t.Fatalf("failed manifest does not explain render stage: %s", manifest)
	}
}

func rendererFixture(t *testing.T) (string, orgconfig.ResolvedProfile) {
	t.Helper()
	root := t.TempDir()
	profile := goldenProfile(t, "henko", "#173F5F")
	profile.Organization.Root = root
	profile.Brand.Assets = nil
	profile.DocumentType.RequestSchema = "invoice.schema.json"
	schema := `{"type":"object","required":["invoice_number","grand_total"],"properties":{"invoice_number":{"type":"string"},"grand_total":{"type":"number"}},"additionalProperties":false}`
	if err := os.WriteFile(filepath.Join(root, profile.DocumentType.RequestSchema), []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}
	capDir := filepath.Join(root, "capabilities", "invoice-pdf")
	if err := os.MkdirAll(capDir, 0o755); err != nil {
		t.Fatal(err)
	}
	workerPath := filepath.Join(capDir, "renderer.sh")
	workerScript := "#!/bin/sh\nprintf 'invoice\\n' > invoice.txt\nprintf '{\"protocol_version\":1,\"job_id\":\"run-001\",\"status\":\"succeeded\",\"outputs\":[{\"path\":\"invoice.txt\",\"media_type\":\"text/plain\"}]}\\n'\n"
	if err := os.WriteFile(workerPath, []byte(workerScript), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "schema_version: 1\nid: invoice-pdf\nversion: 1.2.0\ndescription: test renderer\ninput_schema: input.json\noutput_schema: output.json\nruntime: native\nentrypoint:\n  - " + workerPath + "\nresource_class: cpu\nplatforms:\n  - linux\n  - darwin\npermissions: []\ntimeout_seconds: 5\nmax_output_bytes: 4096\n"
	if err := os.WriteFile(filepath.Join(capDir, "capability.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(capDir, "input.json"), []byte(`{"type":"object"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(capDir, "output.json"), []byte(`{"type":"object"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, profile
}
