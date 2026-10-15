package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
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

func TestOrganizationInitCopiesEmbeddedScaffoldAndPrintsValidationCommand(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "organization-docs")
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, "codex"))
	var stdout, stderr bytes.Buffer

	code := Run([]string{"organization", "init", "--path", destination}, nil, &stdout, &stderr)
	if code != ExitSuccess || !strings.Contains(stdout.String(), "Next: orgdoc organization validate --organization '"+destination+"'") {
		t.Fatalf("organization init = (%d, %q, %q)", code, stdout.String(), stderr.String())
	}
	for _, relative := range []string{
		"organization.yaml",
		"brands/henko.yaml",
		"brands/assets/henko-mark.svg",
		"capabilities/invoice-pdf/capability.yaml",
		"skills/humanizer.md",
	} {
		if _, err := os.Stat(filepath.Join(destination, relative)); err != nil {
			t.Errorf("initialized scaffold is missing %s: %v", relative, err)
		}
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(destination)
		if err != nil || info.Mode().Perm() != 0o755 {
			t.Fatalf("initialized organization directory mode = (%v, %v), want 0755", info, err)
		}
	}
	for _, userSkillPath := range []string{
		filepath.Join(home, ".codex", "skills", "orgdoc"),
		filepath.Join(home, "codex", "skills", "orgdoc"),
	} {
		if _, err := os.Stat(userSkillPath); !os.IsNotExist(err) {
			t.Fatalf("organization init installed a user-level skill at %s", userSkillPath)
		}
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"organization", "validate", "--organization", destination}, nil, &stdout, &stderr)
	if code != ExitSuccess || !strings.Contains(stdout.String(), "is valid") {
		t.Fatalf("initialized organization failed validation: (%d, %q, %q)", code, stdout.String(), stderr.String())
	}
}

func TestOrganizationInitReturnsNextCommandAsArgumentsInJSON(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "organization docs")
	var stdout, stderr bytes.Buffer

	code := Run([]string{"organization", "init", "--path", destination, "--format", "json"}, nil, &stdout, &stderr)
	var result struct {
		Path        string   `json:"path"`
		NextCommand []string `json:"next_command"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("organization init output is not JSON: %v: %s", err, stdout.String())
	}
	wantCommand := []string{"orgdoc", "organization", "validate", "--organization", destination}
	if code != ExitSuccess || result.Path != destination || !reflect.DeepEqual(result.NextCommand, wantCommand) {
		t.Fatalf("organization init JSON = (%d, %#v, %q)", code, result, stderr.String())
	}
}

func TestOrganizationInitRefusesExistingDestinationWithoutOverwriting(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "existing")
	if err := os.Mkdir(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(destination, "keep.txt")
	if err := os.WriteFile(sentinel, []byte("preserve me"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	code := Run([]string{"organization", "init", "--path", destination}, nil, &stdout, &stderr)
	contents, err := os.ReadFile(sentinel)
	if err != nil || string(contents) != "preserve me" {
		t.Fatalf("existing destination was changed: (%q, %v)", contents, err)
	}
	if code != ExitError || !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("organization init = (%d, %q, %q), want existing destination refusal", code, stdout.String(), stderr.String())
	}
	if _, err := os.Stat(filepath.Join(destination, "organization.yaml")); !os.IsNotExist(err) {
		t.Fatalf("existing destination received scaffold files; stat error = %v", err)
	}
}
