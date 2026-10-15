package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCapabilitiesListReturnsStableJSON(t *testing.T) {
	root := t.TempDir()
	manifest := "schema_version: 1\nid: invoice-pdf\nversion: 1.0.0\ndescription: Invoice renderer\ninput_schema: input.json\noutput_schema: output.json\nruntime: native\nentrypoint:\n  - /bin/true\nresource_class: cpu\nplatforms:\n  - linux\npermissions: []\ntimeout_seconds: 5\nmax_output_bytes: 1024\n"
	manifestPath := filepath.Join(root, "capabilities", "invoice-pdf", "capability.yaml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(manifestPath), "input.json"), []byte(`{"type":"object"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(manifestPath), "output.json"), []byte(`{"type":"object"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"capabilities", "list", "--organization", root, "--format", "json"}, nil, &stdout, &stderr)
	var capabilities []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &capabilities); err != nil {
		t.Fatalf("output is not JSON: %v: %s", err, stdout.String())
	}
	if code != ExitSuccess || len(capabilities) != 1 || capabilities[0]["id"] != "invoice-pdf" {
		t.Fatalf("Run() = (%d, %#v, %q)", code, capabilities, stderr.String())
	}
}

func TestCapabilitiesDescribeUnknownIDReturnsJSONError(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := Run([]string{"capabilities", "describe", "missing", "--organization", root, "--format=json"}, nil, &stdout, &stderr)
	var envelope ErrorEnvelope
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("error output is not JSON: %v: %s", err, stderr.String())
	}
	if code != ExitError || envelope.Error.Code != "not_found" {
		t.Fatalf("Run() = (%d, %#v, %q)", code, envelope, stdout.String())
	}
}
