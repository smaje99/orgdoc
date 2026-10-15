// Package examples contains organization scaffolds distributed with OrgDoc.
package examples

import "embed"

// OrganizationDocs is the versioned example organization copied by `orgdoc organization init`.
//
//go:embed organization-docs
var OrganizationDocs embed.FS
