package cli

import (
	"encoding/json"
	"fmt"
	"io"
)

// ErrorEnvelope is the stable machine-readable error response written to stderr.
type ErrorEnvelope struct {
	Version int      `json:"version"`
	Error   CLIError `json:"error"`
}

// CLIError identifies a failure without requiring agents to parse prose.
type CLIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(stderr io.Writer, format, code, message string) {
	if format == "json" {
		_ = json.NewEncoder(stderr).Encode(ErrorEnvelope{
			Version: 1,
			Error:   CLIError{Code: code, Message: message},
		})
		return
	}
	_, _ = fmt.Fprintf(stderr, "Error: %s\n", message)
}

func writeVersion(stdout io.Writer, format, version string) {
	if format == "json" {
		_ = json.NewEncoder(stdout).Encode(struct {
			Version string `json:"version"`
		}{Version: version})
		return
	}
	_, _ = fmt.Fprintf(stdout, "orgdoc version %s\n", version)
}
