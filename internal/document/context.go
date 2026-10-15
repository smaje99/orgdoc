package document

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/smaje99/orgdoc/internal/orgconfig"
)

type DocumentContext struct {
	SchemaVersion        int                 `json:"schema_version"`
	Organization         OrganizationContext `json:"organization"`
	Operation            OperationContext    `json:"operation"`
	Brand                BrandContext        `json:"brand"`
	DocumentType         DocumentTypeContext `json:"document_type"`
	RendererCapabilityID string              `json:"renderer_capability_id"`
}

type OrganizationContext struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type OperationContext struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	BrandID   string `json:"brand_id"`
	LegalName string `json:"legal_name,omitempty"`
	TaxID     string `json:"tax_id,omitempty"`
	Address   string `json:"address,omitempty"`
}
type BrandContext struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Version string         `json:"version"`
	Assets  []string       `json:"assets"`
	Visual  map[string]any `json:"visual"`
	Voice   map[string]any `json:"voice"`
}
type DocumentTypeContext struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	RequiredFields  []string `json:"required_fields"`
	ProtectedFields []string `json:"protected_fields"`
}

// BuildDocumentContext builds a renderer context from exactly one resolved profile.
func BuildDocumentContext(profile orgconfig.ResolvedProfile) (DocumentContext, error) {
	if profile.Organization.Root == "" {
		return DocumentContext{}, fmt.Errorf("organization root is required to build document context")
	}
	if profile.Operation.BrandID != profile.Brand.ID {
		return DocumentContext{}, fmt.Errorf("operation %q selects brand %q but resolved brand is %q", profile.Operation.ID, profile.Operation.BrandID, profile.Brand.ID)
	}
	assets := make([]string, 0, len(profile.Brand.Assets))
	for _, asset := range profile.Brand.Assets {
		if _, err := organizationPath(profile.Organization.Root, asset); err != nil {
			return DocumentContext{}, fmt.Errorf("brand %q asset %q: %w", profile.Brand.ID, asset, err)
		}
		assets = append(assets, filepath.ToSlash(asset))
	}
	return DocumentContext{
		SchemaVersion:        orgconfig.SchemaVersion,
		Organization:         OrganizationContext{ID: profile.Organization.ID, Name: profile.Organization.Name},
		Operation:            OperationContext{ID: profile.Operation.ID, Name: profile.Operation.Name, BrandID: profile.Operation.BrandID, LegalName: profile.Operation.LegalName, TaxID: profile.Operation.TaxID, Address: profile.Operation.Address},
		Brand:                BrandContext{ID: profile.Brand.ID, Name: profile.Brand.Name, Version: profile.Brand.Version, Assets: assets, Visual: profile.Brand.Visual, Voice: profile.Brand.Voice},
		DocumentType:         DocumentTypeContext{ID: profile.DocumentType.ID, Name: profile.DocumentType.Name, Version: profile.DocumentType.Version, RequiredFields: profile.DocumentType.RequiredFields, ProtectedFields: profile.DocumentType.ProtectedFields},
		RendererCapabilityID: profile.DocumentType.RendererCapability,
	}, nil
}

func organizationPath(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path must be a non-empty relative path inside the organization")
	}
	clean := filepath.Clean(relative)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must stay inside the organization")
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve organization root: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(root, clean))
	if err != nil {
		return "", fmt.Errorf("resolve organization path: %w", err)
	}
	rel, err := filepath.Rel(rootReal, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path must stay inside the organization")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return "", fmt.Errorf("path must reference a regular file inside the organization")
	}
	return resolved, nil
}
