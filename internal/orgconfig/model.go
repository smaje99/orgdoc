package orgconfig

// SchemaVersion is the first supported organization profile format.
const SchemaVersion = 1

type Organization struct {
	SchemaVersion int            `yaml:"schema_version" json:"schema_version"`
	ID            string         `yaml:"id" json:"id"`
	Name          string         `yaml:"name" json:"name"`
	Brands        []Brand        `yaml:"-" json:"brands"`
	Operations    []Operation    `yaml:"-" json:"operations"`
	DocumentTypes []DocumentType `yaml:"-" json:"document_types"`
	Root          string         `yaml:"-" json:"-"`
}

type Brand struct {
	SchemaVersion int            `yaml:"schema_version" json:"schema_version"`
	ID            string         `yaml:"id" json:"id"`
	Name          string         `yaml:"name" json:"name"`
	Version       string         `yaml:"version" json:"version"`
	Assets        []string       `yaml:"assets" json:"assets"`
	Visual        map[string]any `yaml:"visual" json:"visual"`
	Voice         map[string]any `yaml:"voice" json:"voice"`
	SourceFile    string         `yaml:"-" json:"source_file"`
}

type Operation struct {
	SchemaVersion int    `yaml:"schema_version" json:"schema_version"`
	ID            string `yaml:"id" json:"id"`
	Name          string `yaml:"name" json:"name"`
	BrandID       string `yaml:"brand_id" json:"brand_id"`
	LegalName     string `yaml:"legal_name" json:"legal_name,omitempty"`
	TaxID         string `yaml:"tax_id" json:"tax_id,omitempty"`
	Address       string `yaml:"address" json:"address,omitempty"`
	SourceFile    string `yaml:"-" json:"source_file"`
}

type DocumentType struct {
	SchemaVersion      int      `yaml:"schema_version" json:"schema_version"`
	ID                 string   `yaml:"id" json:"id"`
	Name               string   `yaml:"name" json:"name"`
	Version            string   `yaml:"version" json:"version"`
	RequiredFields     []string `yaml:"required_fields" json:"required_fields"`
	ProtectedFields    []string `yaml:"protected_fields" json:"protected_fields"`
	RendererCapability string   `yaml:"renderer_capability" json:"renderer_capability"`
	SourceFile         string   `yaml:"-" json:"source_file"`
}

type ResolvedProfile struct {
	Organization Organization `json:"organization"`
	Operation    Operation    `json:"operation"`
	Brand        Brand        `json:"brand"`
	DocumentType DocumentType `json:"document_type"`
}

type Diagnostic struct {
	File    string `json:"file"`
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (diagnostic Diagnostic) Error() string {
	location := diagnostic.File
	if diagnostic.Field != "" {
		location += ":" + diagnostic.Field
	}
	return location + ": " + diagnostic.Message
}

type organizationManifest struct {
	SchemaVersion int      `yaml:"schema_version"`
	ID            string   `yaml:"id"`
	Name          string   `yaml:"name"`
	BrandFiles    []string `yaml:"brands"`
	Operations    []string `yaml:"operations"`
	DocumentTypes []string `yaml:"document_types"`
}
