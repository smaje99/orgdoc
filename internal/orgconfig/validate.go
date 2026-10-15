package orgconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	orgschemas "github.com/smaje/orgdoc/schemas"
)

// ValidateOrganization reports deterministic, location-aware profile errors.
func ValidateOrganization(root string) []Diagnostic {
	organization, diagnostics := readOrganization(root)
	if organization.Root == "" && len(diagnostics) > 0 {
		return diagnostics
	}
	return append(diagnostics, validateOrganization(organization)...)
}

// ResolveProfile selects one operation, its brand, and a document type.
func ResolveProfile(root, operationID, documentTypeID string) (ResolvedProfile, error) {
	if strings.TrimSpace(operationID) == "" {
		return ResolvedProfile{}, fmt.Errorf("operation ID is required; select an operation explicitly")
	}
	if strings.TrimSpace(documentTypeID) == "" {
		return ResolvedProfile{}, fmt.Errorf("document type ID is required")
	}
	organization, err := LoadOrganization(root)
	if err != nil {
		if diagnostic, ok := err.(Diagnostic); ok && diagnostic.Code == "duplicate_operation" {
			return ResolvedProfile{}, fmt.Errorf("ambiguous operation ID: %s", diagnostic.Message)
		}
		return ResolvedProfile{}, err
	}
	operations := make([]Operation, 0, 1)
	for _, operation := range organization.Operations {
		if operation.ID == operationID {
			operations = append(operations, operation)
		}
	}
	if len(operations) == 0 {
		return ResolvedProfile{}, fmt.Errorf("unknown operation ID %q", operationID)
	}
	if len(operations) != 1 {
		return ResolvedProfile{}, fmt.Errorf("ambiguous operation ID %q", operationID)
	}
	var brand *Brand
	for index := range organization.Brands {
		if organization.Brands[index].ID == operations[0].BrandID {
			brand = &organization.Brands[index]
			break
		}
	}
	var selectedDocument *DocumentType
	for index := range organization.DocumentTypes {
		if organization.DocumentTypes[index].ID == documentTypeID {
			selectedDocument = &organization.DocumentTypes[index]
			break
		}
	}
	if brand == nil {
		return ResolvedProfile{}, fmt.Errorf("operation %q references unknown brand %q", operationID, operations[0].BrandID)
	}
	if selectedDocument == nil {
		return ResolvedProfile{}, fmt.Errorf("unknown document type ID %q", documentTypeID)
	}
	return ResolvedProfile{Organization: organization, Operation: operations[0], Brand: *brand, DocumentType: *selectedDocument}, nil
}

func validateOrganization(organization Organization) []Diagnostic {
	var diagnostics []Diagnostic
	add := func(file, field, code, message string) {
		diagnostics = append(diagnostics, Diagnostic{File: filepath.ToSlash(file), Field: field, Code: code, Message: message})
	}
	if organization.SchemaVersion != SchemaVersion {
		add("organization.yaml", "schema_version", "unsupported_schema_version", fmt.Sprintf("unsupported schema version %d; supported version is %d", organization.SchemaVersion, SchemaVersion))
	}
	if organization.ID == "" {
		add("organization.yaml", "id", "required", "organization ID is required")
	}
	if organization.Name == "" {
		add("organization.yaml", "name", "required", "organization name is required")
	}

	brandByID := make(map[string]Brand, len(organization.Brands))
	for _, brand := range organization.Brands {
		file := brand.SourceFile
		validateProfileHeader(file, brand.SchemaVersion, brand.ID, brand.Name, add)
		if _, exists := brandByID[brand.ID]; exists {
			add(file, "id", "duplicate_brand", fmt.Sprintf("duplicate brand ID %q", brand.ID))
		}
		brandByID[brand.ID] = brand
		if brand.Version == "" {
			add(file, "version", "required", "brand profile version is required")
		}
		for index, asset := range brand.Assets {
			if err := validateAssetPath(organization.Root, asset); err != nil {
				add(file, fmt.Sprintf("assets[%d]", index), "invalid_asset_path", err.Error())
			}
		}
	}

	operationIDs := make(map[string]bool, len(organization.Operations))
	for _, operation := range organization.Operations {
		file := operation.SourceFile
		validateProfileHeader(file, operation.SchemaVersion, operation.ID, operation.Name, add)
		if operationIDs[operation.ID] {
			add(file, "id", "duplicate_operation", fmt.Sprintf("duplicate operation ID %q", operation.ID))
		}
		operationIDs[operation.ID] = true
		if _, exists := brandByID[operation.BrandID]; !exists {
			add(file, "brand_id", "unknown_brand", fmt.Sprintf("operation references unknown brand %q", operation.BrandID))
		}
	}

	documentIDs := make(map[string]bool, len(organization.DocumentTypes))
	for _, documentType := range organization.DocumentTypes {
		file := documentType.SourceFile
		validateProfileHeader(file, documentType.SchemaVersion, documentType.ID, documentType.Name, add)
		if documentIDs[documentType.ID] {
			add(file, "id", "duplicate_document_type", fmt.Sprintf("duplicate document type ID %q", documentType.ID))
		}
		documentIDs[documentType.ID] = true
		if documentType.Version == "" {
			add(file, "version", "required", "document profile version is required")
		}
		if documentType.RequestSchema == "" {
			add(file, "request_schema", "required", "request schema path is required")
		} else if schemaPath, err := safePath(organization.Root, documentType.RequestSchema); err != nil {
			add(file, "request_schema", "invalid_request_schema", err.Error())
		} else if contents, err := os.ReadFile(schemaPath); err != nil {
			add(file, "request_schema", "invalid_request_schema", fmt.Sprintf("read request schema: %v", err))
		} else {
			if err := orgschemas.ValidateSchemaBytes(contents); err != nil {
				add(file, "request_schema", "invalid_request_schema", err.Error())
			}
		}
		if len(documentType.RequiredFields) == 0 {
			add(file, "required_fields", "required", "at least one required field must be declared")
		}
		if strings.TrimSpace(documentType.RendererCapability) == "" {
			add(file, "renderer_capability", "required", "renderer capability ID is required")
		}
	}
	return diagnostics
}

func validateProfileHeader(file string, schemaVersion int, id, name string, add func(string, string, string, string)) {
	if schemaVersion != SchemaVersion {
		add(file, "schema_version", "unsupported_schema_version", fmt.Sprintf("unsupported schema version %d; supported version is %d", schemaVersion, SchemaVersion))
	}
	if strings.TrimSpace(id) == "" {
		add(file, "id", "required", "profile ID is required")
	}
	if strings.TrimSpace(name) == "" {
		add(file, "name", "required", "profile name is required")
	}
}

func validateAssetPath(root, relative string) error {
	if relative == "" || filepath.IsAbs(relative) {
		return fmt.Errorf("asset path must be a non-empty relative path inside the organization repository")
	}
	clean := filepath.Clean(relative)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("asset path must stay inside the organization repository")
	}
	if root == "" {
		return fmt.Errorf("organization root is not available for asset validation")
	}
	if _, err := safePath(root, clean); err != nil {
		return fmt.Errorf("asset path must stay inside the organization repository: %w", err)
	}
	return nil
}
