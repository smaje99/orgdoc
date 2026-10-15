package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/smaje/orgdoc/internal/capability"
	"github.com/smaje/orgdoc/internal/document"
	"github.com/smaje/orgdoc/internal/orgconfig"
	"github.com/smaje/orgdoc/internal/scheduler"
	"github.com/smaje/orgdoc/internal/worker"
)

type batchOptions struct {
	JobsFile     string
	Organization string
	OutputRoot   string
	MaxWorkers   int
}

type batchDefinition struct {
	SchemaVersion int               `json:"schema_version"`
	Organization  string            `json:"organization"`
	OutputRoot    string            `json:"output_root"`
	Jobs          []batchJobRequest `json:"jobs"`
}

type batchJobRequest struct {
	JobID        string          `json:"job_id"`
	Operation    string          `json:"operation"`
	DocumentType string          `json:"document_type"`
	Input        json.RawMessage `json:"input"`
}

type resolvedBatchProfile struct {
	profile orgconfig.ResolvedProfile
	cap     capability.Capability
	err     error
}

type batchDocumentSummary struct {
	JobID        string                    `json:"job_id"`
	Status       document.RunStatus        `json:"status"`
	ArtifactDir  string                    `json:"artifact_dir"`
	ManifestPath string                    `json:"manifest_path"`
	Outputs      []worker.WorkerOutput     `json:"outputs"`
	Diagnostics  []worker.WorkerDiagnostic `json:"diagnostics"`
}

func parseBatchOptions(args []string) (batchOptions, error) {
	options := batchOptions{MaxWorkers: scheduler.DefaultMaxWorkers}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if !strings.HasPrefix(arg, "--") {
			if options.JobsFile != "" {
				return options, fmt.Errorf("unexpected argument %q", arg)
			}
			options.JobsFile = arg
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		if !hasValue {
			if index+1 >= len(args) {
				return options, fmt.Errorf("--%s requires a value", name)
			}
			index++
			value = args[index]
		}
		switch name {
		case "organization":
			options.Organization = value
		case "output-root":
			options.OutputRoot = value
		case "max-workers":
			maxWorkers, err := strconv.Atoi(value)
			if err != nil || maxWorkers < 1 {
				return options, fmt.Errorf("--max-workers must be a positive integer")
			}
			options.MaxWorkers = maxWorkers
		default:
			return options, fmt.Errorf("unknown option --%s", name)
		}
	}
	if options.JobsFile == "" {
		return options, fmt.Errorf("usage: orgdoc batch run JOBS.json [--organization PATH] [--output-root PATH] [--max-workers N]")
	}
	return options, nil
}

func runBatch(ctx context.Context, args []string, format string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "run" {
		writeError(stderr, format, "usage_error", "usage: orgdoc batch run JOBS.json [--organization PATH] [--output-root PATH] [--max-workers N]")
		return ExitUsage
	}
	options, err := parseBatchOptions(args[1:])
	if err != nil {
		writeError(stderr, format, "usage_error", err.Error())
		return ExitUsage
	}
	definition, batchDir, err := loadBatchDefinition(options.JobsFile)
	if err != nil {
		writeError(stderr, format, "batch_input_error", err.Error())
		return ExitError
	}
	organizationRoot := options.Organization
	if organizationRoot == "" {
		organizationRoot = definition.Organization
	}
	outputRoot := options.OutputRoot
	if outputRoot == "" {
		outputRoot = definition.OutputRoot
	}
	if organizationRoot == "" || outputRoot == "" {
		writeError(stderr, format, "batch_input_error", "jobs file must define organization and output_root, or pass both --organization and --output-root")
		return ExitError
	}
	organizationRoot = resolveBatchPath(batchDir, organizationRoot)
	outputRoot = resolveBatchPath(batchDir, outputRoot)
	if len(definition.Jobs) == 0 {
		writeError(stderr, format, "batch_input_error", "jobs file must contain at least one job")
		return ExitError
	}

	limits := scheduler.ResourceLimitsForMaxWorkers(options.MaxWorkers)
	runner := worker.NewRunner(limits.ByClass, 5*time.Minute)
	jobs := make([]scheduler.Job, len(definition.Jobs))
	profiles := make(map[string]resolvedBatchProfile)
	for index, request := range definition.Jobs {
		job := scheduler.Job{ID: request.JobID, ResourceClass: worker.ResourceClassCPU}
		input := any(nil)
		if len(request.Input) == 0 {
			job.Run = func(context.Context) (any, error) {
				return nil, fmt.Errorf("job %q has no input object", request.JobID)
			}
			jobs[index] = job
			continue
		}
		if err := json.Unmarshal(request.Input, &input); err != nil {
			job.Run = func(context.Context) (any, error) {
				return nil, fmt.Errorf("job %q input is invalid JSON: %w", request.JobID, err)
			}
			jobs[index] = job
			continue
		}
		profileKey := request.Operation + "\x00" + request.DocumentType
		resolved, exists := profiles[profileKey]
		if !exists {
			resolved.profile, resolved.err = orgconfig.ResolveProfile(organizationRoot, request.Operation, request.DocumentType)
			if resolved.err == nil {
				resolved.cap, resolved.err = capability.Describe(organizationRoot, resolved.profile.DocumentType.RendererCapability)
			}
			profiles[profileKey] = resolved
		}
		if resolved.err != nil {
			resolveErr := resolved.err
			job.Run = func(context.Context) (any, error) {
				return nil, fmt.Errorf("resolve job %q document profile or capability: %w", request.JobID, resolveErr)
			}
			jobs[index] = job
			continue
		}
		profile := resolved.profile
		job.ResourceClass = resolved.cap.ResourceClass
		job.Run = func(jobCtx context.Context) (any, error) {
			rendered, err := document.RenderDocument(jobCtx, document.RenderRequest{
				Profile: profile, Input: input, OutputRoot: outputRoot, JobID: request.JobID, Runner: runner,
			})
			return batchDocumentSummary{
				JobID: rendered.JobID, Status: rendered.Status, ArtifactDir: rendered.ArtifactDir,
				ManifestPath: rendered.ManifestPath, Outputs: rendered.Outputs, Diagnostics: rendered.Diagnostics,
			}, err
		}
		jobs[index] = job
	}

	result := scheduler.RunBatch(ctx, jobs, limits)
	if format == "json" {
		_ = json.NewEncoder(stdout).Encode(result)
	} else {
		_, _ = fmt.Fprintf(stdout, "Batch %s: %d jobs\n", result.Status, len(result.Results))
		for _, job := range result.Results {
			if job.Error != "" {
				_, _ = fmt.Fprintf(stdout, "  %s\t%s\t%s\n", job.JobID, job.Status, job.Error)
			} else {
				_, _ = fmt.Fprintf(stdout, "  %s\t%s\n", job.JobID, job.Status)
			}
		}
	}
	if result.Status != scheduler.BatchStatusSucceeded {
		return ExitError
	}
	return ExitSuccess
}

func loadBatchDefinition(path string) (batchDefinition, string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return batchDefinition{}, "", fmt.Errorf("resolve jobs file path: %w", err)
	}
	contents, err := os.ReadFile(absolute)
	if err != nil {
		return batchDefinition{}, "", fmt.Errorf("read jobs file: %w", err)
	}
	var definition batchDefinition
	decoder := json.NewDecoder(strings.NewReader(string(contents)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&definition); err != nil {
		return batchDefinition{}, "", fmt.Errorf("parse jobs file: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return batchDefinition{}, "", fmt.Errorf("parse jobs file: trailing JSON value")
		}
		return batchDefinition{}, "", fmt.Errorf("parse jobs file: trailing content: %w", err)
	}
	if definition.SchemaVersion != 1 {
		return batchDefinition{}, "", fmt.Errorf("jobs file schema_version must be 1")
	}
	return definition, filepath.Dir(absolute), nil
}

func resolveBatchPath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}
