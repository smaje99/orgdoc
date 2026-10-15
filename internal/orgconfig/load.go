package orgconfig

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	orgschemas "github.com/smaje/orgdoc/schemas"
	"gopkg.in/yaml.v3"
)

// LoadOrganization loads and validates all organization profiles below root.
func LoadOrganization(root string) (Organization, error) {
	organization, diagnostics := readOrganization(root)
	if organization.Root == "" && len(diagnostics) > 0 {
		return Organization{}, diagnostics[0]
	}
	diagnostics = append(diagnostics, validateOrganization(organization)...)
	if len(diagnostics) > 0 {
		return Organization{}, diagnostics[0]
	}
	return organization, nil
}

func readOrganization(root string) (Organization, []Diagnostic) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Organization{}, []Diagnostic{{File: "organization.yaml", Field: "", Code: "invalid_root", Message: err.Error()}}
	}
	manifestPath := filepath.Join(absRoot, "organization.yaml")
	var manifest organizationManifest
	if err := decodeYAML(manifestPath, "organization.schema.json", &manifest); err != nil {
		return Organization{}, []Diagnostic{decodeDiagnostic("organization.yaml", err)}
	}

	organization := Organization{
		SchemaVersion: manifest.SchemaVersion,
		ID:            manifest.ID,
		Name:          manifest.Name,
		Root:          absRoot,
	}
	var diagnostics []Diagnostic
	for index, relative := range manifest.BrandFiles {
		path, pathErr := safePath(absRoot, relative)
		if pathErr != nil {
			diagnostics = append(diagnostics, Diagnostic{File: "organization.yaml", Field: fmt.Sprintf("brands[%d]", index), Code: "invalid_path", Message: pathErr.Error()})
			continue
		}
		var brand Brand
		if err := decodeYAML(path, "brand.schema.json", &brand); err != nil {
			diagnostics = append(diagnostics, decodeDiagnostic(relative, err))
			continue
		}
		brand.SourceFile = relative
		organization.Brands = append(organization.Brands, brand)
	}
	for index, relative := range manifest.Operations {
		path, pathErr := safePath(absRoot, relative)
		if pathErr != nil {
			diagnostics = append(diagnostics, Diagnostic{File: "organization.yaml", Field: fmt.Sprintf("operations[%d]", index), Code: "invalid_path", Message: pathErr.Error()})
			continue
		}
		var operation Operation
		if err := decodeYAML(path, "operation.schema.json", &operation); err != nil {
			diagnostics = append(diagnostics, decodeDiagnostic(relative, err))
			continue
		}
		operation.SourceFile = relative
		organization.Operations = append(organization.Operations, operation)
	}
	for index, relative := range manifest.DocumentTypes {
		path, pathErr := safePath(absRoot, relative)
		if pathErr != nil {
			diagnostics = append(diagnostics, Diagnostic{File: "organization.yaml", Field: fmt.Sprintf("document_types[%d]", index), Code: "invalid_path", Message: pathErr.Error()})
			continue
		}
		var documentType DocumentType
		if err := decodeYAML(path, "document-type.schema.json", &documentType); err != nil {
			diagnostics = append(diagnostics, decodeDiagnostic(relative, err))
			continue
		}
		documentType.SourceFile = relative
		organization.DocumentTypes = append(organization.DocumentTypes, documentType)
	}
	return organization, diagnostics
}

func decodeYAML(path, schemaName string, destination any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	var raw any
	if err := decoder.Decode(&raw); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("multiple YAML documents are not supported")
		}
		return err
	}
	if err := orgschemas.Validate(schemaName, raw); err != nil {
		return err
	}
	encoded, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(encoded, destination)
}

func decodeDiagnostic(file string, err error) Diagnostic {
	var schemaErr orgschemas.ValidationError
	if errors.As(err, &schemaErr) {
		if schemaErr.Field == "schema_version" {
			return Diagnostic{File: filepath.ToSlash(file), Field: schemaErr.Field, Code: "unsupported_schema_version", Message: "unsupported schema version; this CLI supports schema version 1"}
		}
		return Diagnostic{File: filepath.ToSlash(file), Field: schemaErr.Field, Code: "schema_validation", Message: schemaErr.Error()}
	}
	message := err.Error()
	return Diagnostic{File: filepath.ToSlash(file), Field: "document", Code: "invalid_yaml", Message: message}
}

func safePath(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path must be a non-empty relative path")
	}
	clean := filepath.Clean(relative)
	if clean == ".." || len(clean) >= 3 && clean[:3] == ".."+string(filepath.Separator) {
		return "", fmt.Errorf("path must stay inside the organization repository")
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve organization repository: %w", err)
	}
	path := filepath.Join(root, clean)
	pathReal, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", relative, err)
	}
	relativeReal, err := filepath.Rel(rootReal, pathReal)
	if err != nil || relativeReal == ".." || len(relativeReal) >= 3 && relativeReal[:3] == ".."+string(filepath.Separator) {
		return "", fmt.Errorf("path must stay inside the organization repository")
	}
	return pathReal, nil
}
