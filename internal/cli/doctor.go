package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/smaje99/orgdoc/internal/capability"
)

type doctorCheck struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type doctorResponse struct {
	Checks []doctorCheck `json:"checks"`
}

func runDoctor(args []string, format string, stdout, stderr io.Writer) int {
	organization, err := parseDoctorOptions(args)
	if err != nil {
		writeError(stderr, format, "usage_error", err.Error())
		return ExitUsage
	}
	manifest := filepath.Join(organization, "organization.yaml")
	if _, err := os.Stat(manifest); err != nil {
		if os.IsNotExist(err) {
			return writeDoctorChecks([]doctorCheck{{
				ID: "organization", Status: "missing",
				Message: fmt.Sprintf("%s was not found; select an organization repository or run orgdoc organization init.", manifest),
			}}, format, stdout)
		}
		writeError(stderr, format, "organization_error", fmt.Sprintf("inspect organization manifest: %v", err))
		return ExitError
	}

	capabilities, err := capability.Discover(organization)
	if err != nil {
		writeError(stderr, format, "capability_error", err.Error())
		return ExitError
	}
	if len(capabilities) == 0 {
		return writeDoctorChecks([]doctorCheck{{
			ID: "capabilities", Status: "warning",
			Message: "No capabilities are declared; install a worker and add its capability manifest when a workflow needs one.",
		}}, format, stdout)
	}

	checks := make([]doctorCheck, 0, len(capabilities)*2)
	for _, item := range capabilities {
		checks = append(checks, checkPlatform(item))
		checks = append(checks, checkEntrypoint(item))
		for _, prerequisite := range item.Prerequisites {
			checks = append(checks, checkPrerequisite(item.ID, prerequisite))
		}
	}
	sort.Slice(checks, func(i, j int) bool { return checks[i].ID < checks[j].ID })
	return writeDoctorChecks(checks, format, stdout)
}

func parseDoctorOptions(args []string) (string, error) {
	organization := "."
	for index := 0; index < len(args); index++ {
		arg := args[index]
		var value string
		switch {
		case arg == "--organization":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
				return "", fmt.Errorf("--organization requires a path")
			}
			index++
			value = args[index]
		case strings.HasPrefix(arg, "--organization="):
			value = strings.TrimPrefix(arg, "--organization=")
		default:
			return "", fmt.Errorf("unknown doctor argument %q", arg)
		}
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("--organization requires a non-empty path")
		}
		organization = value
	}
	return organization, nil
}

func checkPlatform(item capability.Capability) doctorCheck {
	platform := runtime.GOOS
	for _, supported := range item.Platforms {
		if supported == platform {
			return doctorCheck{
				ID: fmt.Sprintf("capability:%s:platform", item.ID), Status: "ok",
				Message: fmt.Sprintf("Capability %q supports %s.", item.ID, platform),
			}
		}
	}
	return doctorCheck{
		ID: fmt.Sprintf("capability:%s:platform", item.ID), Status: "unsupported",
		Message: fmt.Sprintf("Capability %q does not declare support for %s.", item.ID, platform),
	}
}

func checkEntrypoint(item capability.Capability) doctorCheck {
	command := item.EntryPoint[0]
	path, err := exec.LookPath(command)
	if err != nil {
		return doctorCheck{
			ID: fmt.Sprintf("capability:%s:entrypoint", item.ID), Status: "missing",
			Message: fmt.Sprintf("Worker command %q is unavailable on PATH; install worker %q with its release installer.", command, item.ID),
		}
	}
	return doctorCheck{
		ID: fmt.Sprintf("capability:%s:entrypoint", item.ID), Status: "ok",
		Message: fmt.Sprintf("Worker command %q is available at %s.", command, path),
	}
}

func checkPrerequisite(capabilityID string, prerequisite capability.Prerequisite) doctorCheck {
	available := make([]string, 0, len(prerequisite.Executables))
	missing := make([]string, 0, len(prerequisite.Executables))
	for _, executable := range prerequisite.Executables {
		if _, err := exec.LookPath(executable); err != nil {
			missing = append(missing, executable)
		} else {
			available = append(available, executable)
		}
	}
	mode := prerequisite.Mode
	if mode == "" {
		mode = "all"
	}
	status := "ok"
	message := fmt.Sprintf("Required executable(s) %s for capability %q are available.", strings.Join(prerequisite.Executables, ", "), capabilityID)
	if (mode == "all" && len(missing) > 0) || (mode == "any" && len(available) == 0) {
		status = "missing"
		message = fmt.Sprintf("Install required executable(s) %s for capability %q.", strings.Join(missing, ", "), capabilityID)
		if mode == "any" {
			message = fmt.Sprintf("Install at least one of these executables for capability %q: %s.", capabilityID, strings.Join(prerequisite.Executables, " or "))
		}
	}
	if prerequisite.VersionNote != "" {
		message += " " + prerequisite.VersionNote
	}
	key := strings.Join(prerequisite.Executables, "|")
	return doctorCheck{
		ID: fmt.Sprintf("capability:%s:prerequisite:%s", capabilityID, key), Status: status, Message: message,
	}
}

func writeDoctorChecks(checks []doctorCheck, format string, stdout io.Writer) int {
	if format == "json" {
		_ = json.NewEncoder(stdout).Encode(doctorResponse{Checks: checks})
	} else {
		for _, check := range checks {
			_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\n", check.Status, check.ID, check.Message)
		}
	}
	for _, check := range checks {
		if check.Status == "missing" || check.Status == "unsupported" {
			return ExitError
		}
	}
	return ExitSuccess
}
