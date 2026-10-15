package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type doctorTestResponse struct {
	Checks []doctorTestCheck `json:"checks"`
}

type doctorTestCheck struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

func TestDoctorReportsMissingOrganization(t *testing.T) {
	root := t.TempDir()
	checks, code, stdout, stderr := runDoctorJSON(t, root)

	if code != ExitError || len(checks) != 1 || checks[0].ID != "organization" || checks[0].Status != "missing" {
		t.Fatalf("doctor = (%d, %#v, %q, %q), want missing organization", code, checks, stdout, stderr)
	}
	if !strings.Contains(checks[0].Message, "organization.yaml") {
		t.Fatalf("missing-organization message = %q", checks[0].Message)
	}
}

func TestDoctorWarnsWhenNoCapabilitiesAreDeclared(t *testing.T) {
	root := writeDoctorOrganization(t)
	checks, code, stdout, stderr := runDoctorJSON(t, root)

	if code != ExitSuccess || len(checks) != 1 || checks[0].ID != "capabilities" || checks[0].Status != "warning" {
		t.Fatalf("doctor = (%d, %#v, %q, %q), want warning and success exit", code, checks, stdout, stderr)
	}
}

func TestDoctorReportsMissingWorkerEntrypoint(t *testing.T) {
	root := writeDoctorOrganization(t)
	writeDoctorCapability(t, root, "missing-worker", "nonexistent-orgdoc-worker", nil, []string{runtime.GOOS})
	path := t.TempDir()
	t.Setenv("PATH", path)

	checks, code, _, stderr := runDoctorJSON(t, root)
	check := findDoctorCheck(t, checks, "capability:missing-worker:entrypoint")
	if code != ExitError || check.Status != "missing" || !strings.Contains(check.Message, "nonexistent-orgdoc-worker") {
		t.Fatalf("entrypoint check = (%d, %#v), stderr %q", code, check, stderr)
	}
}

func TestDoctorReportsMissingSystemExecutable(t *testing.T) {
	root := writeDoctorOrganization(t)
	writeDoctorCapability(t, root, "needs-tool", "/bin/true", []doctorPrerequisite{{
		Executables: []string{"orgdoc-missing-tool"}, Mode: "all",
	}}, []string{runtime.GOOS})
	path := t.TempDir()
	t.Setenv("PATH", path)

	checks, code, _, stderr := runDoctorJSON(t, root)
	check := findDoctorCheck(t, checks, "capability:needs-tool:prerequisite:orgdoc-missing-tool")
	if code != ExitError || check.Status != "missing" || !strings.Contains(check.Message, "orgdoc-missing-tool") {
		t.Fatalf("prerequisite check = (%d, %#v), stderr %q", code, check, stderr)
	}
}

func TestDoctorReportsUnsupportedPlatform(t *testing.T) {
	root := writeDoctorOrganization(t)
	platform := "linux"
	if runtime.GOOS == "linux" {
		platform = "darwin"
	}
	writeDoctorCapability(t, root, "other-platform", "/bin/true", nil, []string{platform})

	checks, code, _, stderr := runDoctorJSON(t, root)
	check := findDoctorCheck(t, checks, "capability:other-platform:platform")
	if code != ExitError || check.Status != "unsupported" || !strings.Contains(check.Message, runtime.GOOS) {
		t.Fatalf("platform check = (%d, %#v), stderr %q", code, check, stderr)
	}
}

func TestDoctorReportsSuccessfulWorkerPresenceInStableOrder(t *testing.T) {
	root := writeDoctorOrganization(t)
	bin := t.TempDir()
	marker := filepath.Join(t.TempDir(), "worker-was-run")
	workerPath := filepath.Join(bin, "orgdoc-present-worker")
	if err := os.WriteFile(workerPath, []byte("#!/bin/sh\ntouch \"$DOCTOR_EXEC_MARKER\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	prerequisitePath := filepath.Join(bin, "orgdoc-present-tool")
	if err := os.WriteFile(prerequisitePath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("DOCTOR_EXEC_MARKER", marker)
	writeDoctorCapability(t, root, "ready-worker", "orgdoc-present-worker", []doctorPrerequisite{{
		Executables: []string{"orgdoc-present-tool"}, Mode: "all",
		VersionNote: "version 2 or newer recommended",
	}}, []string{runtime.GOOS})

	checks, code, _, stderr := runDoctorJSON(t, root)
	if code != ExitSuccess || stderr != "" {
		t.Fatalf("doctor = (%d, %#v, stderr %q), want success", code, checks, stderr)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("doctor executed worker unexpectedly; marker stat error = %v", err)
	}
	if len(checks) != 3 {
		t.Fatalf("doctor returned %d checks, want 3: %#v", len(checks), checks)
	}
	for index, wantID := range []string{
		"capability:ready-worker:entrypoint",
		"capability:ready-worker:platform",
		"capability:ready-worker:prerequisite:orgdoc-present-tool",
	} {
		if checks[index].ID != wantID || checks[index].Status != "ok" || checks[index].Message == "" {
			t.Fatalf("check[%d] = %#v, want %q status ok with message", index, checks[index], wantID)
		}
	}
}

func TestDoctorHumanFormatPrintsActionableChecks(t *testing.T) {
	root := writeDoctorOrganization(t)
	writeDoctorCapability(t, root, "needs-human", "/bin/true", []doctorPrerequisite{{
		Executables: []string{"orgdoc-human-tool"}, Mode: "all", VersionNote: "version 1 required",
	}}, []string{runtime.GOOS})
	t.Setenv("PATH", t.TempDir())
	var stdout, stderr bytes.Buffer

	code := Run([]string{"doctor", "--organization", root}, nil, &stdout, &stderr)
	if code != ExitError || !strings.Contains(stdout.String(), "missing") || !strings.Contains(stdout.String(), "orgdoc-human-tool") {
		t.Fatalf("human doctor = (%d, %q, %q)", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "version 1 required") {
		t.Fatalf("human doctor did not include prerequisite version note: %q", stdout.String())
	}
}

func TestDoctorAcceptsOneOfEquivalentPrerequisiteExecutables(t *testing.T) {
	root := writeDoctorOrganization(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "orgdoc-alternative-tool"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	writeDoctorCapability(t, root, "alternative-worker", "/bin/true", []doctorPrerequisite{{
		Executables: []string{"orgdoc-missing-alternative", "orgdoc-alternative-tool"}, Mode: "any",
	}}, []string{runtime.GOOS})

	checks, code, _, stderr := runDoctorJSON(t, root)
	check := findDoctorCheck(t, checks, "capability:alternative-worker:prerequisite:orgdoc-missing-alternative|orgdoc-alternative-tool")
	if code != ExitSuccess || check.Status != "ok" {
		t.Fatalf("alternative prerequisite = (%d, %#v), stderr %q", code, check, stderr)
	}
}

func runDoctorJSON(t *testing.T, root string) ([]doctorTestCheck, int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"doctor", "--organization", root, "--format", "json"}, nil, &stdout, &stderr)
	var response doctorTestResponse
	if err := json.Unmarshal(stdout.Bytes(), &response); err != nil {
		t.Fatalf("doctor output is not JSON: %v: %s", err, stdout.String())
	}
	return response.Checks, code, stdout.String(), stderr.String()
}

func findDoctorCheck(t *testing.T, checks []doctorTestCheck, id string) doctorTestCheck {
	t.Helper()
	for _, check := range checks {
		if check.ID == id {
			return check
		}
	}
	t.Fatalf("doctor checks %#v do not contain ID %q", checks, id)
	return doctorTestCheck{}
}

type doctorPrerequisite struct {
	Executables []string
	Mode        string
	VersionNote string
}

func writeDoctorOrganization(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "organization.yaml"), []byte("schema_version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func writeDoctorCapability(t *testing.T, root, id, executable string, prerequisites []doctorPrerequisite, platforms []string) {
	t.Helper()
	directory := filepath.Join(root, "capabilities", id)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	platformYAML := ""
	for _, platform := range platforms {
		platformYAML += "  - " + platform + "\n"
	}
	prerequisiteYAML := ""
	for _, prerequisite := range prerequisites {
		prerequisiteYAML += "  - executables:\n"
		for _, executable := range prerequisite.Executables {
			prerequisiteYAML += "      - " + executable + "\n"
		}
		prerequisiteYAML += "    mode: " + prerequisite.Mode + "\n"
		if prerequisite.VersionNote != "" {
			prerequisiteYAML += "    version_note: " + prerequisite.VersionNote + "\n"
		}
	}
	manifest := "schema_version: 1\n" +
		"id: " + id + "\n" +
		"version: 1.0.0\n" +
		"description: test worker\n" +
		"input_schema: input.schema.json\n" +
		"output_schema: output.schema.json\n" +
		"runtime: native\n" +
		"entrypoint:\n  - " + executable + "\n" +
		"resource_class: cpu\n" +
		"platforms:\n" + platformYAML +
		"permissions: []\n" +
		"timeout_seconds: 5\n" +
		"max_output_bytes: 1024\n"
	if len(prerequisites) > 0 {
		manifest += "prerequisites:\n" + prerequisiteYAML
	}
	if err := os.WriteFile(filepath.Join(directory, "capability.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"input.schema.json", "output.schema.json"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(`{"type":"object"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
