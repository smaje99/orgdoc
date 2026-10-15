package document

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
