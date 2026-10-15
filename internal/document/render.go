package document

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/smaje/orgdoc/internal/capability"
	"github.com/smaje/orgdoc/internal/worker"
)

var safeJobID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,79}$`)

// RenderDocument validates input before creating artifacts or starting a worker.
func RenderDocument(ctx context.Context, request RenderRequest) (DocumentResult, error) {
	if err := ValidateDocumentInput(request.Profile, request.Input); err != nil {
		return DocumentResult{}, err
	}
	documentContext, err := BuildDocumentContext(request.Profile)
	if err != nil {
		return DocumentResult{}, err
	}
	cap, err := capability.Describe(request.Profile.Organization.Root, request.Profile.DocumentType.RendererCapability)
	if err != nil {
		return DocumentResult{}, fmt.Errorf("resolve renderer capability %q: %w", request.Profile.DocumentType.RendererCapability, err)
	}
	jobID := request.JobID
	if jobID == "" {
		jobID, err = newJobID()
		if err != nil {
			return DocumentResult{}, fmt.Errorf("create job ID: %w", err)
		}
	}
	if !safeJobID.MatchString(jobID) {
		return DocumentResult{}, fmt.Errorf("job ID %q may contain only letters, numbers, underscore, and hyphen", jobID)
	}
	outputRoot, err := filepath.Abs(request.OutputRoot)
	if err != nil || request.OutputRoot == "" {
		return DocumentResult{}, fmt.Errorf("output root must be a non-empty path")
	}
	artifactDir := filepath.Join(outputRoot, jobID)
	if _, err := os.Lstat(artifactDir); err == nil {
		return DocumentResult{}, fmt.Errorf("job %q already has an artifact directory; choose a new job ID", jobID)
	} else if !os.IsNotExist(err) {
		return DocumentResult{}, fmt.Errorf("inspect artifact directory: %w", err)
	}
	started := time.Now().UTC()
	startedAt := started.Format(time.RFC3339Nano)
	result := DocumentResult{JobID: jobID, Status: RunStatusFailed, ArtifactDir: artifactDir, ManifestPath: filepath.Join(artifactDir, "run-manifest.json"), Outputs: []worker.WorkerOutput{}, Diagnostics: []worker.WorkerDiagnostic{}, Context: documentContext}
	manifest := RunManifest{
		SchemaVersion: 1, JobID: jobID, Status: RunStatusFailed, StartedAt: startedAt,
		OperationID: request.Profile.Operation.ID, BrandID: request.Profile.Brand.ID, DocumentTypeID: request.Profile.DocumentType.ID,
		ProfileVersions: ProfileVersions{OrganizationSchema: request.Profile.Organization.SchemaVersion, OperationSchema: request.Profile.Operation.SchemaVersion, Brand: request.Profile.Brand.Version, DocumentType: request.Profile.DocumentType.Version},
		CapabilityID:    cap.ID, CapabilityVersion: cap.Version, InputReferences: append([]string{}, request.InputReferences...), OutputPaths: []string{}, Diagnostics: []worker.WorkerDiagnostic{},
		Stages: []RunStage{{Name: "validate_input", Status: "succeeded", StartedAt: startedAt, EndedAt: startedAt}, {Name: "resolve_capability", Status: "succeeded", StartedAt: startedAt, EndedAt: startedAt}},
	}
	if request.Runner == nil {
		request.Runner = worker.NewRunner(nil, 5*time.Minute)
	}
	workerRequest := worker.WorkerRequest{
		ProtocolVersion: worker.ProtocolVersion, JobID: jobID,
		OperationID: request.Profile.Operation.ID, BrandID: request.Profile.Brand.ID,
		DocumentType: request.Profile.DocumentType.ID,
		Input:        map[string]any{"document": request.Input, "context": documentContext, "organization_root": request.Profile.Organization.Root},
		ArtifactDir:  artifactDir,
	}
	workerResult, runErr := request.Runner.Run(ctx, cap, workerRequest)
	if runErr != nil && len(workerResult.Diagnostics) > 0 && workerResult.Diagnostics[0].Code == "artifact_dir" {
		return result, fmt.Errorf("create artifact directory for run %q: %w", jobID, runErr)
	}
	finished := time.Now().UTC()
	endedAt := finished.Format(time.RFC3339Nano)
	stage := RunStage{Name: "render", StartedAt: startedAt, EndedAt: endedAt, Status: "succeeded"}
	if runErr != nil {
		stage.Status, stage.Message = "failed", runErr.Error()
		result.Diagnostics = append(result.Diagnostics, workerResult.Diagnostics...)
		manifest.Diagnostics = append(manifest.Diagnostics, workerResult.Diagnostics...)
	} else {
		result.Status = RunStatusSucceeded
		result.Outputs = workerResult.Outputs
		manifest.Status = RunStatusSucceeded
		for _, output := range workerResult.Outputs {
			manifest.OutputPaths = append(manifest.OutputPaths, filepath.ToSlash(filepath.Join(jobID, output.Path)))
		}
	}
	manifest.Stages = append(manifest.Stages, stage)
	manifest.CompletedAt = endedAt
	if err := writeManifest(result.ManifestPath, manifest); err != nil {
		return result, fmt.Errorf("persist run manifest: %w", err)
	}
	if runErr != nil {
		return result, fmt.Errorf("render document: %w", runErr)
	}
	return result, nil
}

func newJobID() (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return "run-" + time.Now().UTC().Format("20060102T150405Z") + "-" + hex.EncodeToString(random[:]), nil
}

func writeManifest(path string, manifest RunManifest) error {
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	temp := path + ".tmp"
	file, err := os.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temp) }()
	if _, err := file.Write(encoded); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Link(temp, path); err != nil {
		return err
	}
	if err := os.Remove(temp); err != nil {
		return err
	}
	return nil
}
