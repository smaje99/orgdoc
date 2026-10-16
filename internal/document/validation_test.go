package document

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/smaje99/orgdoc/internal/orgconfig"
)

func TestValidateBrandedSpreadsheetExampleAgainstDocumentSchema(t *testing.T) {
	organizationRoot := filepath.Join("..", "..", "examples", "organization-docs")
	profile, err := orgconfig.ResolveProfile(organizationRoot, "consulting", "spreadsheet")
	if err != nil {
		t.Fatal(err)
	}
	example, err := os.ReadFile(filepath.Join("..", "..", "workers", "python", "spreadsheet", "examples", "quarterly-revenue.json"))
	if err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	if err := json.Unmarshal(example, &input); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDocumentInput(profile, input); err != nil {
		t.Fatalf("valid spreadsheet example error = %v", err)
	}

	sheets := input["sheets"].([]any)
	rows := sheets[0].(map[string]any)["rows"].([]any)
	cell := rows[1].([]any)[1].(map[string]any)
	cell["value"] = "twelve million"
	if err := ValidateDocumentInput(profile, input); err == nil {
		t.Fatal("spreadsheet schema accepted text for a currency cell")
	}
}

func TestValidateBrandedPresentationExampleAgainstDocumentSchema(t *testing.T) {
	organizationRoot := filepath.Join("..", "..", "examples", "organization-docs")
	profile, err := orgconfig.ResolveProfile(organizationRoot, "consulting", "presentation")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join("..", "..", "workers", "python", "presentation", "examples", "quarterly-review.json"))
	if err != nil {
		t.Fatal(err)
	}
	var request map[string]any
	if err := json.Unmarshal(content, &request); err != nil {
		t.Fatal(err)
	}
	input := request["input"].(map[string]any)["document"].(map[string]any)
	if err := ValidateDocumentInput(profile, input); err != nil {
		t.Fatalf("valid presentation example error = %v", err)
	}
}

func TestValidateDocumentInputUsesSelectedRequestSchema(t *testing.T) {
	profile := goldenProfile(t, "henko", "#173F5F")
	schemaPath := filepath.Join(profile.Organization.Root, "schemas", "invoice.json")
	if err := os.MkdirAll(filepath.Dir(schemaPath), 0o755); err != nil {
		t.Fatal(err)
	}
	schema := `{"type":"object","required":["invoice_number","grand_total"],"properties":{"invoice_number":{"type":"string"},"grand_total":{"type":"number"}},"additionalProperties":false}`
	if err := os.WriteFile(schemaPath, []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}
	profile.DocumentType.RequestSchema = "schemas/invoice.json"
	if err := ValidateDocumentInput(profile, map[string]any{"invoice_number": "F-1", "grand_total": 125.0}); err != nil {
		t.Fatalf("valid input error = %v", err)
	}
	if err := ValidateDocumentInput(profile, map[string]any{"invoice_number": "F-1", "grand_total": "125"}); err == nil || !strings.Contains(err.Error(), "grand_total") {
		t.Fatalf("invalid input error = %v", err)
	}
}

func TestValidateDocumentInputRejectsSchemaPathEscape(t *testing.T) {
	profile := goldenProfile(t, "henko", "#173F5F")
	profile.DocumentType.RequestSchema = "../outside.json"
	if err := ValidateDocumentInput(profile, map[string]any{}); err == nil || !strings.Contains(err.Error(), "inside the organization") {
		t.Fatalf("path escape error = %v", err)
	}
}

func TestValidateDocumentInputRejectsExternalSchemaReferences(t *testing.T) {
	profile := goldenProfile(t, "henko", "#173F5F")
	profile.DocumentType.RequestSchema = "request.schema.json"
	if err := os.WriteFile(filepath.Join(profile.Organization.Root, profile.DocumentType.RequestSchema), []byte(`{"$dynamicRef":"https://example.invalid/schema.json"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDocumentInput(profile, map[string]any{}); err == nil || !strings.Contains(err.Error(), "external references are not allowed") {
		t.Fatalf("external reference error = %v", err)
	}
}
