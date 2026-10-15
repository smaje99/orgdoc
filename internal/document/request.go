package document

import (
	"github.com/smaje/orgdoc/internal/orgconfig"
	"github.com/smaje/orgdoc/internal/worker"
)

type RenderRequest struct {
	Profile          orgconfig.ResolvedProfile
	Input            any
	InputReferences  []string
	OutputRoot       string
	JobID            string
	PreHumanizedText *string
	Runner           *worker.Runner
}

type RunStatus string

const (
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
)

type DocumentResult struct {
	JobID        string                    `json:"job_id"`
	Status       RunStatus                 `json:"status"`
	ArtifactDir  string                    `json:"artifact_dir"`
	ManifestPath string                    `json:"manifest_path"`
	Outputs      []worker.WorkerOutput     `json:"outputs"`
	Diagnostics  []worker.WorkerDiagnostic `json:"diagnostics"`
	Context      DocumentContext           `json:"context"`
}

type RunManifest struct {
	SchemaVersion     int                       `json:"schema_version"`
	JobID             string                    `json:"job_id"`
	Status            RunStatus                 `json:"status"`
	StartedAt         string                    `json:"started_at"`
	CompletedAt       string                    `json:"completed_at"`
	OperationID       string                    `json:"operation_id"`
	BrandID           string                    `json:"brand_id"`
	DocumentTypeID    string                    `json:"document_type_id"`
	ProfileVersions   ProfileVersions           `json:"profile_versions"`
	CapabilityID      string                    `json:"capability_id"`
	CapabilityVersion string                    `json:"capability_version"`
	InputReferences   []string                  `json:"input_references"`
	OutputPaths       []string                  `json:"output_paths"`
	Stages            []RunStage                `json:"stages"`
	Diagnostics       []worker.WorkerDiagnostic `json:"diagnostics"`
	PreHumanizedText  *string                   `json:"pre_humanized_text,omitempty"`
}

type ProfileVersions struct {
	OrganizationSchema int    `json:"organization_schema"`
	OperationSchema    int    `json:"operation_schema"`
	Brand              string `json:"brand"`
	DocumentType       string `json:"document_type"`
}

type RunStage struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	StartedAt string `json:"started_at"`
	EndedAt   string `json:"ended_at"`
	Message   string `json:"message,omitempty"`
}
