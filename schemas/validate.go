package schemas

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var (
	validationMu   sync.Mutex
	compiledSchema = make(map[string]*jsonschema.Schema)
	schemaFiles    = []string{
		"organization.schema.json",
		"brand.schema.json",
		"operation.schema.json",
		"document-type.schema.json",
		"capability.schema.json",
	}
)

// ValidationError carries the most specific instance field available from JSON Schema.
type ValidationError struct {
	Field string
	Err   error
}

func (validationError ValidationError) Error() string { return validationError.Err.Error() }
func (validationError ValidationError) Unwrap() error { return validationError.Err }

// Validate checks an instance against a packaged JSON Schema without network access.
func Validate(name string, instance any) error {
	schema, err := schemaFor(name)
	if err != nil {
		return fmt.Errorf("load packaged schema %q: %w", name, err)
	}
	if err := schema.Validate(instance); err != nil {
		return ValidationError{Field: schemaField(err), Err: err}
	}
	return nil
}

func schemaFor(name string) (*jsonschema.Schema, error) {
	validationMu.Lock()
	defer validationMu.Unlock()
	if schema, ok := compiledSchema[name]; ok {
		return schema, nil
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	for _, file := range schemaFiles {
		contents, err := Read(file)
		if err != nil {
			return nil, err
		}
		resource, err := jsonschema.UnmarshalJSON(bytes.NewReader(contents))
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", file, err)
		}
		if err := compiler.AddResource("https://orgdoc.dev/schemas/"+file, resource); err != nil {
			return nil, fmt.Errorf("register %s: %w", file, err)
		}
	}
	schema, err := compiler.Compile("https://orgdoc.dev/schemas/" + name)
	if err != nil {
		return nil, err
	}
	compiledSchema[name] = schema
	return schema, nil
}

func schemaField(err error) string {
	var validationError *jsonschema.ValidationError
	if !errors.As(err, &validationError) {
		return "schema"
	}
	if len(validationError.InstanceLocation) > 0 {
		return strings.Join(validationError.InstanceLocation, ".")
	}
	const locationMarker = "at '"
	if _, after, ok := strings.Cut(err.Error(), locationMarker); ok {
		pointer, _, found := strings.Cut(after, "':")
		if found && pointer != "" && pointer != "/" {
			parts := strings.Split(strings.TrimPrefix(pointer, "/"), "/")
			return strings.ReplaceAll(strings.ReplaceAll(parts[len(parts)-1], "~1", "/"), "~0", "~")
		}
	}
	const marker = "additional properties '"
	if _, after, ok := strings.Cut(err.Error(), marker); ok {
		field, _, found := strings.Cut(after, "' not allowed")
		if found {
			return field
		}
	}
	return "schema"
}
