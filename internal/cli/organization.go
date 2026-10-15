package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/smaje99/orgdoc/examples"
	"github.com/smaje99/orgdoc/internal/document"
	"github.com/smaje99/orgdoc/internal/orgconfig"
)

type organizationOptions struct {
	Organization     string
	Operation        string
	DocumentType     string
	Input            string
	OutputRoot       string
	JobID            string
	PreHumanizedText string
}

func parseOrganizationOptions(args []string) (organizationOptions, error) {
	options := organizationOptions{Organization: "."}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if !strings.HasPrefix(arg, "--") {
			return options, fmt.Errorf("unexpected argument %q", arg)
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
		case "operation":
			options.Operation = value
		case "document-type":
			options.DocumentType = value
		case "input":
			options.Input = value
		case "output-root":
			options.OutputRoot = value
		case "job-id":
			options.JobID = value
		case "pre-humanized-text":
			options.PreHumanizedText = value
		default:
			return options, fmt.Errorf("unknown option --%s", name)
		}
	}
	return options, nil
}

func runOrganization(args []string, format string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeError(stderr, format, "usage_error", "usage: orgdoc organization <init|validate>")
		return ExitUsage
	}
	if args[0] == "init" {
		destination, err := parseOrganizationInitPath(args[1:])
		if err != nil {
			writeError(stderr, format, "usage_error", err.Error())
			return ExitUsage
		}
		return initializeOrganization(destination, stdout, stderr, format)
	}
	if args[0] != "validate" {
		writeError(stderr, format, "usage_error", "usage: orgdoc organization <init|validate>")
		return ExitUsage
	}
	options, err := parseOrganizationOptions(args[1:])
	if err != nil {
		writeError(stderr, format, "usage_error", err.Error())
		return ExitUsage
	}
	diagnostics := orgconfig.ValidateOrganization(options.Organization)
	valid := len(diagnostics) == 0
	if format == "json" {
		_ = json.NewEncoder(stdout).Encode(struct {
			Valid       bool                   `json:"valid"`
			Diagnostics []orgconfig.Diagnostic `json:"diagnostics"`
		}{Valid: valid, Diagnostics: append([]orgconfig.Diagnostic{}, diagnostics...)})
	} else if valid {
		_, _ = fmt.Fprintln(stdout, "Organization configuration is valid.")
	} else {
		for _, diagnostic := range diagnostics {
			_, _ = fmt.Fprintln(stderr, diagnostic.Error())
		}
	}
	if !valid {
		return ExitError
	}
	return ExitSuccess
}

func parseOrganizationInitPath(args []string) (string, error) {
	var destination string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		var value string
		switch {
		case arg == "--path":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
				return "", fmt.Errorf("--path requires a destination path")
			}
			index++
			value = args[index]
		case strings.HasPrefix(arg, "--path="):
			value = strings.TrimPrefix(arg, "--path=")
		default:
			return "", fmt.Errorf("unknown organization init argument %q", arg)
		}
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("--path requires a non-empty destination path")
		}
		if destination != "" {
			return "", fmt.Errorf("--path may be provided only once")
		}
		destination = value
	}
	if destination == "" {
		return "", fmt.Errorf("organization init requires --path PATH")
	}
	return destination, nil
}

func initializeOrganization(destination string, stdout, stderr io.Writer, format string) int {
	absoluteDestination, err := filepath.Abs(destination)
	if err != nil {
		writeError(stderr, format, "organization_error", fmt.Sprintf("resolve destination: %v", err))
		return ExitError
	}
	if _, err := os.Lstat(absoluteDestination); err == nil {
		writeError(stderr, format, "already_exists", fmt.Sprintf("destination %q already exists; choose a new path", destination))
		return ExitError
	} else if !os.IsNotExist(err) {
		writeError(stderr, format, "organization_error", fmt.Sprintf("inspect destination: %v", err))
		return ExitError
	}
	parent := filepath.Dir(absoluteDestination)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		writeError(stderr, format, "organization_error", fmt.Sprintf("create destination parent: %v", err))
		return ExitError
	}
	stage, err := os.MkdirTemp(parent, ".orgdoc-organization-init-*")
	if err != nil {
		writeError(stderr, format, "organization_error", fmt.Sprintf("stage organization scaffold: %v", err))
		return ExitError
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0o755); err != nil {
		writeError(stderr, format, "organization_error", fmt.Sprintf("set scaffold directory permissions: %v", err))
		return ExitError
	}
	if err := copyOrganizationScaffold(stage); err != nil {
		writeError(stderr, format, "organization_error", fmt.Sprintf("copy organization scaffold: %v", err))
		return ExitError
	}
	if _, err := os.Lstat(absoluteDestination); err == nil {
		writeError(stderr, format, "already_exists", fmt.Sprintf("destination %q already exists; choose a new path", destination))
		return ExitError
	} else if !os.IsNotExist(err) {
		writeError(stderr, format, "organization_error", fmt.Sprintf("inspect destination: %v", err))
		return ExitError
	}
	if err := os.Rename(stage, absoluteDestination); err != nil {
		writeError(stderr, format, "organization_error", fmt.Sprintf("install organization scaffold: %v", err))
		return ExitError
	}
	if format == "json" {
		_ = json.NewEncoder(stdout).Encode(struct {
			Path        string   `json:"path"`
			NextCommand []string `json:"next_command"`
		}{
			Path: destination, NextCommand: []string{"orgdoc", "organization", "validate", "--organization", destination},
		})
		return ExitSuccess
	}
	_, _ = fmt.Fprintf(stdout, "Initialized organization scaffold at %s\nNext: orgdoc organization validate --organization %s\n", destination, shellQuote(destination))
	return ExitSuccess
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func copyOrganizationScaffold(destination string) error {
	return fs.WalkDir(examples.OrganizationDocs, "organization-docs", func(source string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative := strings.TrimPrefix(source, "organization-docs/")
		if relative == "organization-docs" {
			relative = "."
		}
		if relative == "." {
			return nil
		}
		target := filepath.Join(destination, filepath.FromSlash(relative))
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		contents, err := fs.ReadFile(examples.OrganizationDocs, source)
		if err != nil {
			return err
		}
		return os.WriteFile(target, contents, 0o644)
	})
}

func runBrands(args []string, format string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "list" {
		writeError(stderr, format, "usage_error", "usage: orgdoc brands list [--organization PATH]")
		return ExitUsage
	}
	options, err := parseOrganizationOptions(args[1:])
	if err != nil {
		writeError(stderr, format, "usage_error", err.Error())
		return ExitUsage
	}
	organization, err := orgconfig.LoadOrganization(options.Organization)
	if err != nil {
		writeError(stderr, format, "organization_error", err.Error())
		return ExitError
	}
	brands := append([]orgconfig.Brand(nil), organization.Brands...)
	sort.Slice(brands, func(i, j int) bool { return brands[i].ID < brands[j].ID })
	if format == "json" {
		_ = json.NewEncoder(stdout).Encode(brands)
		return ExitSuccess
	}
	for _, brand := range brands {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\n", brand.ID, brand.Name, brand.Version)
	}
	return ExitSuccess
}

func runContext(args []string, format string, stdout, stderr io.Writer) int {
	options, err := parseOrganizationOptions(args)
	if err != nil {
		writeError(stderr, format, "usage_error", err.Error())
		return ExitUsage
	}
	profile, err := orgconfig.ResolveProfile(options.Organization, options.Operation, options.DocumentType)
	if err != nil {
		writeError(stderr, format, "profile_error", err.Error())
		return ExitError
	}
	context, err := document.BuildDocumentContext(profile)
	if err != nil {
		writeError(stderr, format, "context_error", err.Error())
		return ExitError
	}
	if format == "json" {
		_ = json.NewEncoder(stdout).Encode(context)
		return ExitSuccess
	}
	_, _ = fmt.Fprintf(stdout, "%s / %s / %s\nBrand: %s (%s)\nRenderer: %s\n", profile.Organization.Name, profile.Operation.Name, profile.DocumentType.Name, profile.Brand.Name, profile.Brand.Version, profile.DocumentType.RendererCapability)
	return ExitSuccess
}

func runDocument(ctx context.Context, args []string, format string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "render" {
		writeError(stderr, format, "usage_error", "usage: orgdoc document render --operation ID --document-type ID --input PATH --output-root PATH")
		return ExitUsage
	}
	options, err := parseOrganizationOptions(args[1:])
	if err != nil {
		writeError(stderr, format, "usage_error", err.Error())
		return ExitUsage
	}
	if options.Input == "" || options.OutputRoot == "" {
		writeError(stderr, format, "usage_error", "--input and --output-root are required")
		return ExitUsage
	}
	input, err := readDocumentInput(options.Input, stdin)
	if err != nil {
		writeError(stderr, format, "input_error", err.Error())
		return ExitError
	}
	profile, err := orgconfig.ResolveProfile(options.Organization, options.Operation, options.DocumentType)
	if err != nil {
		writeError(stderr, format, "profile_error", err.Error())
		return ExitError
	}
	var preHumanizedText *string
	if options.PreHumanizedText != "" {
		contents, readErr := os.ReadFile(options.PreHumanizedText)
		if readErr != nil {
			writeError(stderr, format, "input_error", fmt.Sprintf("read pre-humanized text: %v", readErr))
			return ExitError
		}
		text := string(contents)
		preHumanizedText = &text
	}
	result, err := document.RenderDocument(ctx, document.RenderRequest{Profile: profile, Input: input, InputReferences: []string{options.Input}, OutputRoot: options.OutputRoot, JobID: options.JobID, PreHumanizedText: preHumanizedText})
	if err != nil {
		writeError(stderr, format, "render_error", err.Error())
		return ExitError
	}
	if format == "json" {
		_ = json.NewEncoder(stdout).Encode(result)
		return ExitSuccess
	}
	_, _ = fmt.Fprintf(stdout, "Rendered %s to %s\nRun manifest: %s\n", profile.DocumentType.ID, result.ArtifactDir, result.ManifestPath)
	for _, output := range result.Outputs {
		_, _ = fmt.Fprintf(stdout, "  %s\n", output.Path)
	}
	return ExitSuccess
}

func readDocumentInput(path string, stdin io.Reader) (any, error) {
	var contents []byte
	var err error
	if path == "-" {
		contents, err = io.ReadAll(stdin)
	} else {
		contents, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, fmt.Errorf("read document input: %w", err)
	}
	var input any
	if err := json.Unmarshal(contents, &input); err != nil {
		return nil, fmt.Errorf("parse document input JSON: %w", err)
	}
	return input, nil
}
