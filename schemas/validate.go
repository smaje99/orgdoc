package schemas

import (
	"bytes"
	"encoding/json"
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

// ValidateBytes validates an instance against an organization-provided schema.
// External references are rejected so validation never fetches network resources.
func ValidateBytes(contents []byte, instance any) error {
	schema, err := compileBytes(contents)
	if err != nil {
		return err
	}
	if err := schema.Validate(instance); err != nil {
		return ValidationError{Field: schemaField(err), Err: err}
	}
	return nil
}

// ValidateSchemaBytes verifies that a user-provided JSON Schema is valid and offline-safe.
func ValidateSchemaBytes(contents []byte) error {
	_, err := compileBytes(contents)
	return err
}

func compileBytes(contents []byte) (*jsonschema.Schema, error) {
	var document any
	if err := json.Unmarshal(contents, &document); err != nil {
		return nil, fmt.Errorf("parse JSON Schema: %w", err)
	}
	if err := rejectExternalReferences(document); err != nil {
		return nil, err
	}
	resource, err := jsonschema.UnmarshalJSON(bytes.NewReader(contents))
	if err != nil {
		return nil, fmt.Errorf("parse JSON Schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	const location = "https://orgdoc.dev/organization/request.schema.json"
	if err := compiler.AddResource(location, resource); err != nil {
		return nil, fmt.Errorf("register request schema: %w", err)
	}
	schema, err := compiler.Compile(location)
	if err != nil {
		return nil, fmt.Errorf("compile request schema: %w", err)
	}
	return schema, nil
}

func rejectExternalReferences(value any) error {
	switch node := value.(type) {
	case map[string]any:
		for _, name := range []string{"$ref", "$dynamicRef", "$recursiveRef"} {
			if reference, ok := node[name].(string); ok && !strings.HasPrefix(reference, "#") {
				return fmt.Errorf("request schema external references are not allowed")
			}
		}
		for _, child := range node {
			if err := rejectExternalReferences(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range node {
			if err := rejectExternalReferences(child); err != nil {
				return err
			}
		}
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
