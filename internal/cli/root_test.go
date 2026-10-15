package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunWithoutArgumentsShowsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run(nil, strings.NewReader(""), &stdout, &stderr)

	if code != ExitSuccess {
		t.Fatalf("Run() exit code = %d, want %d; stderr: %s", code, ExitSuccess, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Usage: orgdoc") {
		t.Fatalf("help output %q does not contain usage", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Exit codes: 0 success, 1 runtime error, 2 usage error") {
		t.Fatalf("help output %q does not document exit codes", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestRunHelpFlagShowsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--help"}, strings.NewReader(""), &stdout, &stderr)

	if code != ExitSuccess || !strings.Contains(stdout.String(), "Usage: orgdoc") {
		t.Fatalf("Run(--help) = (%d, %q, %q)", code, stdout.String(), stderr.String())
	}
}

func TestRunHelpListsDoctorCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if code != ExitSuccess || !strings.Contains(stdout.String(), "doctor [--organization PATH]") {
		t.Fatalf("help output does not document doctor: (%d, %q, %q)", code, stdout.String(), stderr.String())
	}
}

func TestRunVersionFlagShowsVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--version"}, strings.NewReader(""), &stdout, &stderr)

	if code != ExitSuccess || !strings.Contains(stdout.String(), "orgdoc version ") {
		t.Fatalf("Run(--version) = (%d, %q, %q)", code, stdout.String(), stderr.String())
	}
}

func TestRunVersionFlagSupportsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--version", "--format", "json"}, strings.NewReader(""), &stdout, &stderr)
	var response map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("version output is not JSON: %v: %s", err, stdout.String())
	}
	if code != ExitSuccess || response["version"] != Version {
		t.Fatalf("Run(--version --format json) = (%d, %#v, %q)", code, response, stderr.String())
	}
}

func TestRunJSONUsageErrorHasStableEnvelope(t *testing.T) {
	var stdout, stderr bytes.Buffer

	code := Run([]string{"--format", "json", "unknown"}, strings.NewReader(""), &stdout, &stderr)
	var envelope ErrorEnvelope
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("error output is not JSON: %v: %s", err, stderr.String())
	}
	if code != ExitUsage {
		t.Fatalf("Run() exit code = %d, want %d", code, ExitUsage)
	}
	if envelope.Version != 1 || envelope.Error.Code != "usage_error" || envelope.Error.Message == "" {
		t.Fatalf("unexpected error envelope: %#v", envelope)
	}
	if stdout.Len() != 0 {
		t.Fatalf("JSON error unexpectedly wrote to stdout: %s", stdout.String())
	}
}

func TestExitCodesAreStable(t *testing.T) {
	if ExitSuccess != 0 || ExitError != 1 || ExitUsage != 2 {
		t.Fatalf("exit codes = success:%d error:%d usage:%d", ExitSuccess, ExitError, ExitUsage)
	}
}
