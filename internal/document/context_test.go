package document

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/smaje/orgdoc/internal/orgconfig"
)

func TestBuildDocumentContextMatchesGolden(t *testing.T) {
	profile := goldenProfile(t, "henko", "#173F5F")
	got, err := BuildDocumentContext(profile)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	expected, err := os.ReadFile(filepath.Join("testdata", "context.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != string(expected) {
		t.Fatalf("context differs from golden:\n%s", encoded)
	}
}

func TestBuildDocumentContextKeepsBrandAssetsAndTokensIsolated(t *testing.T) {
	henko, err := BuildDocumentContext(goldenProfile(t, "henko", "#173F5F"))
	if err != nil {
		t.Fatal(err)
	}
	academy, err := BuildDocumentContext(goldenProfile(t, "academy", "#ED553B"))
	if err != nil {
		t.Fatal(err)
	}
	if henko.Brand.ID == academy.Brand.ID || henko.Brand.Visual["primary_color"] == academy.Brand.Visual["primary_color"] {
		t.Fatalf("brand contexts were mixed: Henko=%#v Academy=%#v", henko.Brand, academy.Brand)
	}
	if henko.Brand.Assets[0] == academy.Brand.Assets[0] {
		t.Fatalf("brand assets were mixed: %#v", henko.Brand.Assets)
	}
}

func goldenProfile(t *testing.T, brandID, color string) orgconfig.ResolvedProfile {
	t.Helper()
	root := t.TempDir()
	asset := filepath.Join("brands", brandID+".svg")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, asset)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, asset), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	return orgconfig.ResolvedProfile{
		Organization: orgconfig.Organization{SchemaVersion: 1, ID: "henko", Name: "Henko Technologies", Root: root},
		Operation:    orgconfig.Operation{SchemaVersion: 1, ID: "consulting", Name: "Consultoría", BrandID: brandID, LegalName: "Henko Technologies S.A.S."},
		Brand:        orgconfig.Brand{SchemaVersion: 1, ID: brandID, Name: brandID, Version: "2.1.0", Assets: []string{asset}, Visual: map[string]any{"primary_color": color}, Voice: map[string]any{"language": "es", "tone": "claro"}},
		DocumentType: orgconfig.DocumentType{SchemaVersion: 1, ID: "invoice", Name: "Factura", Version: "1.0.0", RequiredFields: []string{"number", "total"}, ProtectedFields: []string{"total"}, RendererCapability: "invoice-pdf"},
	}
}
