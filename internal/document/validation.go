package document

import (
	"fmt"
	"os"

	"github.com/smaje/orgdoc/internal/orgconfig"
	"github.com/smaje/orgdoc/schemas"
)

// ValidateDocumentInput checks document data against the selected profile's request schema.
func ValidateDocumentInput(profile orgconfig.ResolvedProfile, input any) error {
	if profile.DocumentType.RequestSchema == "" {
		return fmt.Errorf("document type %q has no request_schema", profile.DocumentType.ID)
	}
	path, err := organizationPath(profile.Organization.Root, profile.DocumentType.RequestSchema)
	if err != nil {
		return fmt.Errorf("request schema must be inside the organization: %w", err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read request schema: %w", err)
	}
	if err := schemas.ValidateBytes(contents, input); err != nil {
		return fmt.Errorf("validate document input: %w", err)
	}
	return nil
}
