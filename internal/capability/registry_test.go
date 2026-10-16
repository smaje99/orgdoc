package capability

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverSortsCapabilitiesAndDescribeFindsByID(t *testing.T) {
	root := t.TempDir()
	writeCapability(t, root, "zeta", validManifest("zeta"))
	writeCapability(t, root, "alpha", validManifest("alpha"))

	capabilities, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(capabilities) != 2 || capabilities[0].ID != "alpha" || capabilities[1].ID != "zeta" {
		t.Fatalf("Discover() IDs = %#v", capabilities)
	}
	described, err := Describe(root, "zeta")
	if err != nil || described.ID != "zeta" {
		t.Fatalf("Describe() = (%#v, %v)", described, err)
	}
}

func TestDiscoverReturnsEmptyWhenCapabilitiesDirectoryIsAbsent(t *testing.T) {
	capabilities, err := Discover(t.TempDir())
	if err != nil || len(capabilities) != 0 {
		t.Fatalf("Discover() = (%#v, %v), want empty", capabilities, err)
	}
}

func TestDiscoverRejectsUnknownManifestFieldsWithLocation(t *testing.T) {
	root := t.TempDir()
	manifest := validManifest("alpha") + "untrusted: true\n"
	writeCapability(t, root, "alpha", manifest)

	_, err := Discover(root)
	if err == nil || !strings.Contains(err.Error(), "untrusted") {
		t.Fatalf("Discover() error = %v, want manifest field diagnostic", err)
	}
}

func TestDiscoverRejectsManifestDirectoryIDMismatch(t *testing.T) {
	root := t.TempDir()
	writeCapability(t, root, "alpha", validManifest("other"))

	_, err := Discover(root)
	if err == nil || !strings.Contains(err.Error(), "does not match directory") {
		t.Fatalf("Discover() error = %v, want directory mismatch", err)
	}
}

func TestDescribeReturnsActionableErrorForUnknownID(t *testing.T) {
	_, err := Describe(t.TempDir(), "missing")
	if err == nil || !strings.Contains(err.Error(), "unknown capability") {
		t.Fatalf("Describe() error = %v", err)
	}
}

func TestExampleOrganizationDiscoversOptionalMediaCapabilities(t *testing.T) {
	root := filepath.Join("..", "..", "examples", "organization-docs")
	capabilities, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover(example organization) error = %v", err)
	}
	want := map[string]ResourceClass{
		"pdf-form-generate":   ResourceClassMemory,
		"pdf-form-fill":       ResourceClassMemory,
		"spreadsheet-xlsx":    ResourceClassMemory,
		"presentation-pptx":   ResourceClassMemory,
		"media-scan":          ResourceClassCPU,
		"media-transcription": ResourceClassMemory,
		"media-report":        ResourceClassMemory,
		"media-pdf":           ResourceClassCPU,
	}
	for _, capability := range capabilities {
		if expected, exists := want[capability.ID]; exists {
			if capability.ResourceClass != expected {
				t.Errorf("%s resource class = %q, want %q", capability.ID, capability.ResourceClass, expected)
			}
			delete(want, capability.ID)
		}
	}
	if len(want) > 0 {
		t.Fatalf("missing optional media capabilities: %v", want)
	}
}

func validManifest(id string) string {
	return "schema_version: 1\n" +
		"id: " + id + "\n" +
		"version: 1.0.0\n" +
		"description: Example capability\n" +
		"input_schema: input.schema.json\n" +
		"output_schema: output.schema.json\n" +
		"runtime: native\n" +
		"entrypoint:\n  - /bin/true\n" +
		"resource_class: cpu\n" +
		"platforms:\n  - linux\n  - darwin\n" +
		"permissions:\n  - filesystem:read_organization\n  - filesystem:write_artifact_dir\n" +
		"timeout_seconds: 5\n" +
		"max_output_bytes: 1024\n"
}

func writeCapability(t *testing.T, root, directory, manifest string) {
	t.Helper()
	path := filepath.Join(root, "capabilities", directory, "capability.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, schemaName := range []string{"input.schema.json", "output.schema.json"} {
		if err := os.WriteFile(filepath.Join(filepath.Dir(path), schemaName), []byte(`{"type":"object"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
