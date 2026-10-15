package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/smaje/orgdoc/internal/capability"
)

func runCapabilities(args []string, format string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		writeError(stderr, format, "usage_error", "capabilities requires list or describe")
		return ExitUsage
	}
	organization := "."
	positional := make([]string, 0, len(args))
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--organization":
			if i+1 >= len(args) {
				writeError(stderr, format, "usage_error", "--organization requires a path")
				return ExitUsage
			}
			i++
			organization = args[i]
		case strings.HasPrefix(arg, "--organization="):
			organization = strings.TrimPrefix(arg, "--organization=")
		default:
			positional = append(positional, arg)
		}
	}
	switch args[0] {
	case "list":
		if len(positional) != 0 {
			writeError(stderr, format, "usage_error", "capabilities list accepts no positional arguments")
			return ExitUsage
		}
		capabilities, err := capability.Discover(organization)
		if err != nil {
			writeError(stderr, format, "capability_error", err.Error())
			return ExitError
		}
		if format == "json" {
			_ = json.NewEncoder(stdout).Encode(capabilities)
			return ExitSuccess
		}
		for _, item := range capabilities {
			_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\n", item.ID, item.Version, item.Description)
		}
		return ExitSuccess
	case "describe":
		if len(positional) != 1 {
			writeError(stderr, format, "usage_error", "capabilities describe requires one capability ID")
			return ExitUsage
		}
		item, err := capability.Describe(organization, positional[0])
		if err != nil {
			code := "capability_error"
			if strings.Contains(err.Error(), "unknown capability") {
				code = "not_found"
			}
			writeError(stderr, format, code, err.Error())
			return ExitError
		}
		if format == "json" {
			_ = json.NewEncoder(stdout).Encode(item)
			return ExitSuccess
		}
		_, _ = fmt.Fprintf(stdout, "%s %s\n%s\nRuntime: %s\nResource class: %s\n", item.ID, item.Version, item.Description, item.Runtime, item.ResourceClass)
		return ExitSuccess
	default:
		writeError(stderr, format, "usage_error", fmt.Sprintf("unknown capabilities command %q", args[0]))
		return ExitUsage
	}
}
