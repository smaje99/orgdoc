package worker

import "github.com/smaje/orgdoc/internal/capability"

const ProtocolVersion = 1

type WorkerRequest struct {
	ProtocolVersion int    `json:"protocol_version"`
	JobID           string `json:"job_id"`
	OperationID     string `json:"operation_id"`
	BrandID         string `json:"brand_id"`
	DocumentType    string `json:"document_type"`
	Input           any    `json:"input"`
	ArtifactDir     string `json:"artifact_dir"`
}

type WorkerStatus string

const (
	WorkerStatusSucceeded WorkerStatus = "succeeded"
	WorkerStatusFailed    WorkerStatus = "failed"
)

type WorkerResult struct {
	ProtocolVersion int                `json:"protocol_version"`
	JobID           string             `json:"job_id"`
	Status          WorkerStatus       `json:"status"`
	Outputs         []WorkerOutput     `json:"outputs"`
	Diagnostics     []WorkerDiagnostic `json:"diagnostics"`
}

type WorkerOutput struct {
	Path      string `json:"path"`
	MediaType string `json:"media_type"`
}

type WorkerDiagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Capability = capability.Capability
type ResourceClass = capability.ResourceClass

const (
	ResourceClassCPU    = capability.ResourceClassCPU
	ResourceClassGPU    = capability.ResourceClassGPU
	ResourceClassMemory = capability.ResourceClassMemory
)
