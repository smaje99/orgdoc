package orgconfig

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestLoadOrganizationResolvesMultipleBrandsAndSharedOperationBrand(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, fixtureFiles(map[string]string{
		"brands/henko.yaml":           brandYAML("henko", "assets/henko.svg"),
		"brands/henko-academy.yaml":   brandYAML("academy", ""),
		"operations/consulting.yaml":  operationYAML("consulting", "henko"),
		"operations/training.yaml":    operationYAML("training", "henko"),
		"document-types/invoice.yaml": documentTypeYAML("invoice"),
		"document-types/report.yaml":  documentTypeYAML("report"),
		"assets/henko.svg":            "<svg/>\n",
	}))

	if diagnostics := ValidateOrganization(root); len(diagnostics) != 0 {
		t.Fatalf("ValidateOrganization() diagnostics = %#v", diagnostics)
	}
	profile, err := ResolveProfile(root, "consulting", "invoice")
	if err != nil {
		t.Fatalf("ResolveProfile() error = %v", err)
	}
	if profile.Operation.ID != "consulting" || profile.Brand.ID != "henko" || profile.DocumentType.ID != "invoice" {
		t.Fatalf("unexpected resolved profile: %#v", profile)
	}
}

func TestCommittedExampleOrganizationResolvesBothBrands(t *testing.T) {
	root := filepath.Join("..", "..", "examples", "organization-docs")
	if diagnostics := ValidateOrganization(root); len(diagnostics) != 0 {
		t.Fatalf("example organization diagnostics = %#v", diagnostics)
	}
	consulting, err := ResolveProfile(root, "consulting", "invoice")
	if err != nil || consulting.Brand.ID != "henko" {
		t.Fatalf("consulting profile = (%#v, %v)", consulting, err)
	}
	academy, err := ResolveProfile(root, "academy", "report")
	if err != nil || academy.Brand.ID != "henko-academy" {
		t.Fatalf("academy profile = (%#v, %v)", academy, err)
	}
	spreadsheet, err := ResolveProfile(root, "consulting", "spreadsheet")
	if err != nil || spreadsheet.DocumentType.RendererCapability != "spreadsheet-xlsx" || spreadsheet.Brand.ID != "henko" {
		t.Fatalf("spreadsheet profile = (%#v, %v)", spreadsheet, err)
	}
	presentation, err := ResolveProfile(root, "consulting", "presentation")
	if err != nil || presentation.DocumentType.RendererCapability != "presentation-pptx" || presentation.Brand.ID != "henko" {
		t.Fatalf("presentation profile = (%#v, %v)", presentation, err)
	}
	for _, documentType := range []string{"media-scan", "media-transcription", "media-report", "media-pdf"} {
		media, err := ResolveProfile(root, "media", documentType)
		if err != nil || media.DocumentType.RendererCapability != documentType {
			t.Fatalf("media profile %q = (%#v, %v)", documentType, media, err)
		}
	}
}

func TestValidateOrganizationReportsMissingBrandReference(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, fixtureFiles(map[string]string{
		"brands/henko.yaml":           brandYAML("henko", ""),
		"operations/consulting.yaml":  operationYAML("consulting", "missing-brand"),
		"document-types/invoice.yaml": documentTypeYAML("invoice"),
	}))

	diagnostics := ValidateOrganization(root)
	if !hasDiagnostic(diagnostics, "operations/consulting.yaml", "brand_id", "unknown brand") {
		t.Fatalf("missing brand diagnostic not found: %#v", diagnostics)
	}
}

func TestValidateOrganizationRejectsAssetPathTraversal(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, fixtureFiles(map[string]string{
		"brands/henko.yaml":           brandYAML("henko", "../outside.svg"),
		"operations/consulting.yaml":  operationYAML("consulting", "henko"),
		"document-types/invoice.yaml": documentTypeYAML("invoice"),
	}))

	diagnostics := ValidateOrganization(root)
	if !hasDiagnostic(diagnostics, "brands/henko.yaml", "assets[0]", "must stay inside") {
		t.Fatalf("asset traversal diagnostic not found: %#v", diagnostics)
	}
}

func TestValidateOrganizationReportsUnsupportedSchemaVersion(t *testing.T) {
	root := t.TempDir()
	files := fixtureFiles(map[string]string{
		"brands/henko.yaml":           brandYAML("henko", ""),
		"operations/consulting.yaml":  operationYAML("consulting", "henko"),
		"document-types/invoice.yaml": documentTypeYAML("invoice"),
	})
	files["organization.yaml"] = strings.Replace(files["organization.yaml"], "schema_version: 1", "schema_version: 99", 1)
	writeFixture(t, root, files)

	diagnostics := ValidateOrganization(root)
	if !hasDiagnostic(diagnostics, "organization.yaml", "schema_version", "unsupported schema version") {
		t.Fatalf("schema version diagnostic not found: %#v", diagnostics)
	}
}

func TestValidateOrganizationStopsAfterMissingManifest(t *testing.T) {
	diagnostics := ValidateOrganization(t.TempDir())
	if len(diagnostics) != 1 || diagnostics[0].Code != "invalid_yaml" {
		t.Fatalf("missing manifest diagnostics = %#v, want one manifest error", diagnostics)
	}
}

func TestValidateOrganizationUsesJSONSchemaAndReportsFieldLocation(t *testing.T) {
	root := t.TempDir()
	files := fixtureFiles(map[string]string{
		"brands/henko.yaml":           brandYAML("henko", ""),
		"operations/consulting.yaml":  operationYAML("consulting", "henko"),
		"document-types/invoice.yaml": documentTypeYAML("invoice"),
	})
	files["brands/henko.yaml"] += "unexpected_property: true\n"
	writeFixture(t, root, files)

	diagnostics := ValidateOrganization(root)
	if !hasDiagnosticCode(diagnostics, "brands/henko.yaml", "unexpected_property", "schema_validation") {
		t.Fatalf("JSON Schema field diagnostic not found: %#v", diagnostics)
	}
}

func TestValidateOrganizationCompilesDocumentRequestSchemas(t *testing.T) {
	root := t.TempDir()
	files := fixtureFiles(map[string]string{
		"brands/henko.yaml":           brandYAML("henko", ""),
		"operations/consulting.yaml":  operationYAML("consulting", "henko"),
		"document-types/invoice.yaml": documentTypeYAML("invoice"),
	})
	files["schemas/invoice.schema.json"] = `{"type":"not-a-json-schema-type"}`
	writeFixture(t, root, files)
	if diagnostics := ValidateOrganization(root); !hasDiagnosticCode(diagnostics, "document-types/invoice.yaml", "request_schema", "invalid_request_schema") {
		t.Fatalf("invalid document request schema diagnostic not found: %#v", diagnostics)
	}
}

func TestResolveProfileRequiresOneExplicitOperation(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, fixtureFiles(map[string]string{
		"brands/henko.yaml":           brandYAML("henko", ""),
		"operations/one.yaml":         operationYAML("consulting", "henko"),
		"operations/two.yaml":         operationYAML("consulting", "henko"),
		"document-types/invoice.yaml": documentTypeYAML("invoice"),
	}))

	if _, err := ResolveProfile(root, "", "invoice"); err == nil || !strings.Contains(err.Error(), "operation ID is required") {
		t.Fatalf("ResolveProfile() with empty operation ID error = %v", err)
	}
	if _, err := ResolveProfile(root, "consulting", "invoice"); err == nil || !strings.Contains(err.Error(), "ambiguous operation ID") {
		t.Fatalf("ResolveProfile() with duplicate operation ID error = %v", err)
	}
}

func TestResolveProfileRejectsUnknownDocumentType(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, fixtureFiles(map[string]string{
		"brands/henko.yaml":           brandYAML("henko", ""),
		"operations/consulting.yaml":  operationYAML("consulting", "henko"),
		"document-types/invoice.yaml": documentTypeYAML("invoice"),
	}))

	if _, err := ResolveProfile(root, "consulting", "unknown"); err == nil || !strings.Contains(err.Error(), "unknown document type ID") {
		t.Fatalf("ResolveProfile() with unknown document type error = %v", err)
	}
}

func fixtureFiles(profiles map[string]string) map[string]string {
	brandPaths, operationPaths, documentPaths := make([]string, 0), make([]string, 0), make([]string, 0)
	for name := range profiles {
		switch {
		case strings.HasPrefix(name, "brands/"):
			brandPaths = append(brandPaths, name)
		case strings.HasPrefix(name, "operations/"):
			operationPaths = append(operationPaths, name)
		case strings.HasPrefix(name, "document-types/"):
			documentPaths = append(documentPaths, name)
		}
	}
	sort.Strings(brandPaths)
	sort.Strings(operationPaths)
	sort.Strings(documentPaths)
	files := map[string]string{
		"organization.yaml": "schema_version: 1\nid: test-org\nname: Test Org\nbrands:\n" + yamlList(brandPaths) + "operations:\n" + yamlList(operationPaths) + "document_types:\n" + yamlList(documentPaths),
	}
	for path, body := range profiles {
		files[path] = body
		if strings.HasPrefix(path, "document-types/") {
			base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			files[filepath.Join("schemas", base+".schema.json")] = `{"type":"object"}`
		}
	}
	return files
}

func yamlList(paths []string) string {
	var result strings.Builder
	for _, path := range paths {
		result.WriteString("  - ")
		result.WriteString(path)
		result.WriteByte('\n')
	}
	return result.String()
}

func brandYAML(id, asset string) string {
	assets := "assets: []\n"
	if asset != "" {
		assets = "assets:\n  - " + asset + "\n"
	}
	return "schema_version: 1\nid: " + id + "\nname: " + id + "\nversion: 1.0.0\n" + assets + "visual:\n  primary_color: '#123456'\nvoice:\n  language: es\n"
}

func operationYAML(id, brandID string) string {
	return "schema_version: 1\nid: " + id + "\nname: " + id + "\nbrand_id: " + brandID + "\n"
}

func documentTypeYAML(id string) string {
	return "schema_version: 1\nid: " + id + "\nname: " + id + "\nversion: 1.0.0\nrequest_schema: schemas/" + id + ".schema.json\nrequired_fields:\n  - number\nprotected_fields:\n  - total\nrenderer_capability: " + id + "-pdf\n"
}

func writeFixture(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for path, body := range files {
		if body == "" {
			continue
		}
		fullPath := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func hasDiagnostic(diagnostics []Diagnostic, file, field, messagePart string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.File == file && diagnostic.Field == field && strings.Contains(diagnostic.Message, messagePart) {
			return true
		}
	}
	return false
}

func hasDiagnosticCode(diagnostics []Diagnostic, file, field, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.File == file && diagnostic.Field == field && diagnostic.Code == code {
			return true
		}
	}
	return false
}
