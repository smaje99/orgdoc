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
)

func TestDocumentRenderReadsJSONFromStdinAndReturnsManifest(t *testing.T) {
	root := writeDocumentOrganization(t)
	outputRoot := filepath.Join(t.TempDir(), "renders")
	var stdout, stderr bytes.Buffer
	args := []string{"document", "render", "--organization", root, "--operation", "consulting", "--document-type", "invoice", "--input", "-", "--output-root", outputRoot, "--job-id", "cli-run", "--format", "json"}
	code := Run(args, strings.NewReader(`{"invoice_number":"F-1","grand_total":42}`), &stdout, &stderr)
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v: %s", err, stdout.String())
	}
	if code != ExitSuccess || result["status"] != "succeeded" {
		t.Fatalf("Run() = (%d, %#v, %q)", code, result, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(outputRoot, "cli-run", "run-manifest.json")); err != nil {
		t.Fatalf("run manifest absent: %v", err)
	}
}

func TestDocumentRenderRejectsInvalidStdinBeforeCreatingArtifacts(t *testing.T) {
	root := writeDocumentOrganization(t)
	outputRoot := filepath.Join(t.TempDir(), "must-not-exist")
	var stdout, stderr bytes.Buffer
	args := []string{"document", "render", "--organization", root, "--operation", "consulting", "--document-type", "invoice", "--input", "-", "--output-root", outputRoot, "--job-id", "invalid", "--format=json"}
	code := Run(args, strings.NewReader(`{"invoice_number":7}`), &stdout, &stderr)
	if code != ExitError || stdout.Len() != 0 {
		t.Fatalf("Run() = (%d, %q, %q)", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(outputRoot); !os.IsNotExist(err) {
		t.Fatalf("invalid input created artifacts, stat error = %v", err)
	}
}

func TestRunContextCancelsDocumentWorker(t *testing.T) {
	root := writeDocumentOrganization(t)
	if err := os.WriteFile(filepath.Join(root, "capabilities", "invoice-pdf", "renderer.sh"), []byte("#!/bin/sh\nsleep 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	outputRoot := filepath.Join(t.TempDir(), "renders")
	var stdout, stderr bytes.Buffer
	args := []string{"document", "render", "--organization", root, "--operation", "consulting", "--document-type", "invoice", "--input", "-", "--output-root", outputRoot, "--job-id", "cancelled"}
	start := time.Now()
	code := RunContext(ctx, args, strings.NewReader(`{"invoice_number":"F-1","grand_total":42}`), &stdout, &stderr)
	if code != ExitError || time.Since(start) > time.Second {
		t.Fatalf("RunContext() = (%d, %q) after %s", code, stderr.String(), time.Since(start))
	}
	if _, err := os.Stat(filepath.Join(outputRoot, "cancelled", "run-manifest.json")); err != nil {
		t.Fatalf("canceled run manifest absent: %v", err)
	}
}

func TestDocumentRenderCanPersistPreHumanizedTextByRequest(t *testing.T) {
	root := writeDocumentOrganization(t)
	outputRoot := filepath.Join(t.TempDir(), "renders")
	originalPath := filepath.Join(t.TempDir(), "original.md")
	original := "Texto previo a la edición.\n"
	if err := os.WriteFile(originalPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	args := []string{"document", "render", "--organization", root, "--operation", "consulting", "--document-type", "invoice", "--input", "-", "--output-root", outputRoot, "--job-id", "cli-run", "--pre-humanized-text", originalPath}
	code := Run(args, strings.NewReader(`{"invoice_number":"F-1","grand_total":42}`), &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("Run() = (%d, %q)", code, stderr.String())
	}
	manifest, err := os.ReadFile(filepath.Join(outputRoot, "cli-run", "run-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), `"pre_humanized_text": "Texto previo a la edición.\n"`) {
		t.Fatalf("original text was not preserved: %s", manifest)
	}
}

func writeDocumentOrganization(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(relative, contents string, mode os.FileMode) {
		t.Helper()
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), mode); err != nil {
			t.Fatal(err)
		}
	}
	write("organization.yaml", "schema_version: 1\nid: test\nname: Test Org\nbrands:\n  - brands/test.yaml\noperations:\n  - operations/consulting.yaml\ndocument_types:\n  - document-types/invoice.yaml\n", 0o644)
	write("brands/test.yaml", "schema_version: 1\nid: test\nname: Test\nversion: 1.0.0\nassets: []\nvisual:\n  color: '#123456'\nvoice:\n  language: es\n", 0o644)
	write("operations/consulting.yaml", "schema_version: 1\nid: consulting\nname: Consulting\nbrand_id: test\n", 0o644)
	write("document-types/invoice.yaml", "schema_version: 1\nid: invoice\nname: Invoice\nversion: 1.0.0\nrequest_schema: schemas/invoice.json\nrequired_fields:\n  - invoice_number\n  - grand_total\nprotected_fields:\n  - grand_total\nrenderer_capability: invoice-pdf\n", 0o644)
	write("schemas/invoice.json", `{"type":"object","required":["invoice_number","grand_total"],"properties":{"invoice_number":{"type":"string"},"grand_total":{"type":"number"}},"additionalProperties":false}`, 0o644)
	workerScript := "#!/bin/sh\nprintf 'invoice\\n' > invoice.txt\nprintf '{\"protocol_version\":1,\"job_id\":\"cli-run\",\"status\":\"succeeded\",\"outputs\":[{\"path\":\"invoice.txt\",\"media_type\":\"text/plain\"}]}\\n'\n"
	workerPath := filepath.Join(root, "capabilities", "invoice-pdf", "renderer.sh")
	write("capabilities/invoice-pdf/renderer.sh", workerScript, 0o755)
	manifest := "schema_version: 1\nid: invoice-pdf\nversion: 1.0.0\ndescription: test\ninput_schema: input.json\noutput_schema: output.json\nruntime: native\nentrypoint:\n  - " + workerPath + "\nresource_class: cpu\nplatforms:\n  - linux\n  - darwin\npermissions: []\ntimeout_seconds: 5\nmax_output_bytes: 4096\n"
	write("capabilities/invoice-pdf/capability.yaml", manifest, 0o644)
	write("capabilities/invoice-pdf/input.json", `{"type":"object"}`, 0o644)
	write("capabilities/invoice-pdf/output.json", `{"type":"object"}`, 0o644)
	return root
}
