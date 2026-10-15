package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestOrganizationValidateReturnsStructuredDiagnostics(t *testing.T) {
	root := filepath.Join("..", "..", "examples", "organization-docs")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"organization", "validate", "--organization", root, "--format=json"}, nil, &stdout, &stderr)
	var result struct {
		Valid       bool             `json:"valid"`
		Diagnostics []map[string]any `json:"diagnostics"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v: %s", err, stdout.String())
	}
	if code != ExitSuccess || !result.Valid || len(result.Diagnostics) != 0 {
		t.Fatalf("Run() = (%d, %#v, %q)", code, result, stderr.String())
	}
}

func TestBrandsListReturnsAllOrganizationBrands(t *testing.T) {
	root := filepath.Join("..", "..", "examples", "organization-docs")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"brands", "list", "--organization", root, "--format", "json"}, nil, &stdout, &stderr)
	var brands []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &brands); err != nil {
		t.Fatalf("invalid JSON output: %v: %s", err, stdout.String())
	}
	if code != ExitSuccess || len(brands) != 2 {
		t.Fatalf("Run() = (%d, %#v, %q)", code, brands, stderr.String())
	}
}

func TestContextCommandUsesOnlySelectedBrand(t *testing.T) {
	root := filepath.Join("..", "..", "examples", "organization-docs")
	var stdout, stderr bytes.Buffer
	code := Run([]string{"context", "--organization", root, "--operation", "academy", "--document-type", "report", "--format=json"}, nil, &stdout, &stderr)
	var context struct {
		Brand struct {
			ID     string         `json:"id"`
			Visual map[string]any `json:"visual"`
			Assets []string       `json:"assets"`
		} `json:"brand"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &context); err != nil {
		t.Fatalf("invalid JSON output: %v: %s", err, stdout.String())
	}
	if code != ExitSuccess || context.Brand.ID != "henko-academy" || context.Brand.Visual["colors"] == nil || len(context.Brand.Assets) != 0 {
		t.Fatalf("Run() = (%d, %#v, %q)", code, context, stderr.String())
	}
}
