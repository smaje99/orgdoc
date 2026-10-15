# Testing Context

## Unit Coverage Priorities

- Media classification by extension
- Artifact directory naming and overwrite policy
- Prompt template loading from package resources
- Config loading from TOML plus environment variables
- Secret redaction
- Command construction for external adapters
- Error mapping across application and infrastructure boundaries

## Integration Coverage Priorities

- `media-report --help`
- `media-report templates list`
- `media-report config init`
- `media-report doctor`
- `media-report process` bootstrap planning and artifact creation
- OrgDoc invoice and report capability workers through the JSON worker contract
- PDF page size, extracted totals, visual brand selection, and overflow rendering for two brand profiles
- Clear worker diagnostics when an isolated Python capability environment is unavailable

## Strategy

- Keep core logic free of subprocess calls where possible.
- Mock heavyweight integrations.
- Keep fixtures small and repository-local.
- Keep Python worker dependencies in worker-specific environments; never add ReportLab to the Go CLI dependencies.
- Render generated PDFs with Poppler for geometry and visual review; use typed fixtures rather than customer documents.
