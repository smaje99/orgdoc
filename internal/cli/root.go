package cli

import (
	"fmt"
	"io"
	"strings"
)

// Version is replaced by release builds with -ldflags. Local builds report dev.
var Version = "dev"

// Run executes the root command and returns a stable process exit code.
func Run(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	format, commandArgs, err := parseRootArgs(args)
	if err != nil {
		writeError(stderr, format, "usage_error", err.Error())
		return ExitUsage
	}

	if len(commandArgs) == 0 {
		_, _ = io.WriteString(stdout, helpText)
		return ExitSuccess
	}
	switch commandArgs[0] {
	case "--help", "-h", "help":
		_, _ = io.WriteString(stdout, helpText)
		return ExitSuccess
	case "--version", "version":
		writeVersion(stdout, format, Version)
		return ExitSuccess
	case "capabilities":
		return runCapabilities(commandArgs[1:], format, stdout, stderr)
	default:
		writeError(stderr, format, "usage_error", fmt.Sprintf("unknown command %q", commandArgs[0]))
		return ExitUsage
	}
}

const helpText = `OrgDoc Runtime applies organization document rules and brand profiles.

Usage: orgdoc [--format human|json] [--help|--version|capabilities]

Options:
  --format FORMAT  Select human or JSON output for diagnostics.
  -h, --help       Show this help.
  --version        Show the CLI version.
  capabilities list [--organization PATH]       List validated capabilities.
  capabilities describe ID [--organization PATH] Show one capability manifest.

Exit codes: 0 success, 1 runtime error, 2 usage error.
`

func parseRootArgs(args []string) (string, []string, error) {
	format := "human"
	commandArgs := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--format":
			if i+1 >= len(args) {
				return format, nil, fmt.Errorf("--format requires human or json")
			}
			i++
			format = args[i]
			if format != "human" && format != "json" {
				return "human", nil, fmt.Errorf("unsupported output format %q", format)
			}
		case strings.HasPrefix(arg, "--format="):
			format = strings.TrimPrefix(arg, "--format=")
			if format != "human" && format != "json" {
				return "human", nil, fmt.Errorf("unsupported output format %q", format)
			}
		default:
			commandArgs = append(commandArgs, arg)
		}
	}
	return format, commandArgs, nil
}
