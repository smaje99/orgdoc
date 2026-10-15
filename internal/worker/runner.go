package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type ResourceLimits map[ResourceClass]int

type Runner struct {
	limits     ResourceLimits
	timeout    time.Duration
	mu         sync.Mutex
	semaphores map[ResourceClass]chan struct{}
}

func NewRunner(limits ResourceLimits, defaultTimeout time.Duration) *Runner {
	copyLimits := make(ResourceLimits, len(limits))
	for class, limit := range limits {
		if limit > 0 {
			copyLimits[class] = limit
		}
	}
	return &Runner{limits: copyLimits, timeout: defaultTimeout, semaphores: make(map[ResourceClass]chan struct{})}
}

func Run(ctx context.Context, capability Capability, request WorkerRequest) (WorkerResult, error) {
	return NewRunner(nil, 5*time.Minute).Run(ctx, capability, request)
}

func (r *Runner) acquire(ctx context.Context, class ResourceClass) (func(), error) {
	r.mu.Lock()
	semaphore := r.semaphores[class]
	if semaphore == nil {
		limit := r.limits[class]
		if limit == 0 {
			limit = 1
		}
		semaphore = make(chan struct{}, limit)
		r.semaphores[class] = semaphore
	}
	r.mu.Unlock()
	select {
	case semaphore <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	var once sync.Once
	return func() { once.Do(func() { <-semaphore }) }, nil
}

func (r *Runner) Run(ctx context.Context, capability Capability, request WorkerRequest) (WorkerResult, error) {
	fail := func(code, message string) (WorkerResult, error) {
		return WorkerResult{ProtocolVersion: ProtocolVersion, JobID: request.JobID, Status: WorkerStatusFailed, Diagnostics: []WorkerDiagnostic{{Code: code, Message: message}}}, errors.New(message)
	}
	if len(capability.EntryPoint) == 0 || strings.TrimSpace(capability.EntryPoint[0]) == "" {
		return fail("invalid_capability", "worker entrypoint is empty")
	}
	if request.JobID == "" {
		return fail("invalid_request", "job_id is required")
	}
	if request.ProtocolVersion != 0 && request.ProtocolVersion != ProtocolVersion {
		return fail("protocol_error", "unsupported worker protocol version")
	}
	for _, platform := range capability.Platforms {
		if platform == runtime.GOOS {
			goto platformSupported
		}
	}
	if len(capability.Platforms) > 0 {
		return fail("unsupported_platform", "capability does not support this operating system")
	}
platformSupported:
	release, err := r.acquire(ctx, capability.ResourceClass)
	if err != nil {
		return fail("canceled", "worker did not start because its resource limit wait was canceled")
	}
	defer release()
	artifactDir, err := filepath.Abs(request.ArtifactDir)
	if err != nil || request.ArtifactDir == "" {
		return fail("invalid_request", "artifact_dir must be a non-empty path")
	}
	if err := os.MkdirAll(filepath.Dir(artifactDir), 0o700); err != nil {
		return fail("artifact_dir", fmt.Sprintf("create artifact parent: %v", err))
	}
	if err := os.Mkdir(artifactDir, 0o700); err != nil {
		return fail("artifact_dir", fmt.Sprintf("create fresh artifact directory: %v", err))
	}
	request.ProtocolVersion = ProtocolVersion
	request.ArtifactDir = artifactDir
	encoded, err := json.Marshal(request)
	if err != nil {
		return fail("invalid_request", fmt.Sprintf("encode worker request: %v", err))
	}
	timeout := r.timeout
	if capability.TimeoutSeconds > 0 {
		timeout = time.Duration(capability.TimeoutSeconds) * time.Second
	}
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	workerCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(workerCtx, capability.EntryPoint[0], capability.EntryPoint[1:]...)
	configureProcess(command)
	command.Dir = artifactDir
	command.Stdin = bytes.NewReader(encoded)
	maxBytes := capability.MaxOutputBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	budget := &outputBudget{limit: maxBytes}
	stdout, stderr := &limitedBuffer{budget: budget}, &limitedBuffer{budget: budget}
	command.Stdout, command.Stderr = stdout, stderr
	runErr := command.Run()
	if budget.exceeded {
		return fail("output_limit", fmt.Sprintf("worker output exceeded %d bytes", maxBytes))
	}
	if workerCtx.Err() != nil {
		if errors.Is(workerCtx.Err(), context.DeadlineExceeded) {
			return fail("timeout", fmt.Sprintf("worker exceeded its %s timeout", timeout))
		}
		return fail("canceled", "worker was canceled")
	}
	if runErr != nil {
		var execError *exec.Error
		if errors.As(runErr, &execError) && errors.Is(execError.Err, exec.ErrNotFound) {
			return fail("worker_environment_missing", fmt.Sprintf("capability %q worker command %q is unavailable; install its isolated worker environment and make the command available on PATH", capability.ID, capability.EntryPoint[0]))
		}
		message := fmt.Sprintf("worker process failed: %v", runErr)
		if stderr.Len() > 0 {
			message += ": " + redact(stderr.String())
		}
		return fail("worker_exit", message)
	}
	var result WorkerResult
	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	if err := decoder.Decode(&result); err != nil {
		return fail("invalid_json", fmt.Sprintf("worker returned invalid JSON: %v", err))
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fail("invalid_json", "worker must return exactly one JSON result")
	}
	if result.ProtocolVersion != ProtocolVersion || result.JobID != request.JobID {
		return fail("protocol_error", "worker result protocol version or job_id does not match request")
	}
	if result.Status != WorkerStatusSucceeded && result.Status != WorkerStatusFailed {
		return fail("protocol_error", "worker result status must be succeeded or failed")
	}
	for _, output := range result.Outputs {
		if err := validateOutputPath(artifactDir, output.Path); err != nil {
			return fail("invalid_artifact_path", err.Error())
		}
	}
	if result.Status == WorkerStatusFailed {
		return result, errors.New("worker reported failure")
	}
	return result, nil
}

type limitedBuffer struct {
	buffer bytes.Buffer
	budget *outputBudget
}

type outputBudget struct {
	mu       sync.Mutex
	limit    int64
	used     int64
	exceeded bool
}

func (b *limitedBuffer) Len() int       { return b.buffer.Len() }
func (b *limitedBuffer) Bytes() []byte  { return b.buffer.Bytes() }
func (b *limitedBuffer) String() string { return b.buffer.String() }

func (b *limitedBuffer) Write(data []byte) (int, error) {
	b.budget.mu.Lock()
	defer b.budget.mu.Unlock()
	remaining := b.budget.limit - b.budget.used
	if remaining <= 0 {
		b.budget.exceeded = true
		return len(data), nil
	}
	if int64(len(data)) > remaining {
		written, _ := b.buffer.Write(data[:remaining])
		b.budget.used += int64(written)
		b.budget.exceeded = true
		return len(data), nil
	}
	written, err := b.buffer.Write(data)
	b.budget.used += int64(written)
	return written, err
}

func validateOutputPath(root, output string) error {
	if output == "" || filepath.IsAbs(output) {
		return fmt.Errorf("worker output path must be a non-empty relative path")
	}
	clean := filepath.Clean(output)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("worker output %q escapes the artifact directory", output)
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve artifact directory: %w", err)
	}
	path := filepath.Join(root, clean)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("worker output %q does not exist: %w", output, err)
	}
	rel, err := filepath.Rel(rootReal, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("worker output %q escapes the artifact directory", output)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("worker output %q must be a regular file", output)
	}
	return nil
}

func redact(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lower := strings.ToLower(line)
		for _, marker := range []string{"api_key=", "token=", "authorization:", "password="} {
			if index := strings.Index(lower, marker); index >= 0 {
				lines[i] = line[:index] + marker + "[REDACTED]"
				break
			}
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
