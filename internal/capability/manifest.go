package capability

import "path/filepath"

type ResourceClass string

const (
	ResourceClassCPU    ResourceClass = "cpu"
	ResourceClassGPU    ResourceClass = "gpu"
	ResourceClassMemory ResourceClass = "memory"
)

// Capability describes a trusted, versioned worker available to an organization.
type Capability struct {
	SchemaVersion  int            `json:"schema_version" yaml:"schema_version"`
	ID             string         `json:"id" yaml:"id"`
	Version        string         `json:"version" yaml:"version"`
	Description    string         `json:"description" yaml:"description"`
	InputSchema    string         `json:"input_schema" yaml:"input_schema"`
	OutputSchema   string         `json:"output_schema" yaml:"output_schema"`
	Runtime        string         `json:"runtime" yaml:"runtime"`
	EntryPoint     []string       `json:"entrypoint" yaml:"entrypoint"`
	ResourceClass  ResourceClass  `json:"resource_class" yaml:"resource_class"`
	Platforms      []string       `json:"platforms" yaml:"platforms"`
	Permissions    []string       `json:"permissions" yaml:"permissions"`
	Prerequisites  []Prerequisite `json:"prerequisites,omitempty" yaml:"prerequisites,omitempty"`
	TimeoutSeconds int            `json:"timeout_seconds" yaml:"timeout_seconds"`
	MaxOutputBytes int64          `json:"max_output_bytes" yaml:"max_output_bytes"`
	ManifestPath   string         `json:"-" yaml:"-"`
	Directory      string         `json:"-" yaml:"-"`
}

// Prerequisite declares executable presence required by a capability. Mode is
// "all" when every executable is required and "any" for equivalent choices.
type Prerequisite struct {
	Executables []string `json:"executables" yaml:"executables"`
	Mode        string   `json:"mode" yaml:"mode"`
	VersionNote string   `json:"version_note,omitempty" yaml:"version_note,omitempty"`
}

func (c Capability) InputSchemaPath() string  { return filepath.Join(c.Directory, c.InputSchema) }
func (c Capability) OutputSchemaPath() string { return filepath.Join(c.Directory, c.OutputSchema) }
