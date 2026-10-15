package capability

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/smaje99/orgdoc/schemas"
	"gopkg.in/yaml.v3"
)

// Discover validates and returns capabilities under <root>/capabilities.
func Discover(root string) ([]Capability, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve organization root: %w", err)
	}
	capRoot := filepath.Join(absRoot, "capabilities")
	entries, err := os.ReadDir(capRoot)
	if errors.Is(err, os.ErrNotExist) {
		return []Capability{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read capabilities directory: %w", err)
	}
	var found []Capability
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		directory := filepath.Join(capRoot, entry.Name())
		manifest := filepath.Join(directory, "capability.yaml")
		capability, err := loadManifest(manifest, directory)
		if err != nil {
			return nil, err
		}
		if capability.ID != entry.Name() {
			return nil, fmt.Errorf("capability id %q does not match directory %q", capability.ID, entry.Name())
		}
		if err := validateCapabilitySchemas(capability); err != nil {
			return nil, err
		}
		found = append(found, capability)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].ID < found[j].ID })
	return found, nil
}

// Describe returns one exact capability ID from the validated registry.
func Describe(root, id string) (Capability, error) {
	capabilities, err := Discover(root)
	if err != nil {
		return Capability{}, err
	}
	for _, capability := range capabilities {
		if capability.ID == id {
			return capability, nil
		}
	}
	return Capability{}, fmt.Errorf("unknown capability %q", id)
}

func loadManifest(path, directory string) (Capability, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Capability{}, fmt.Errorf("read capability manifest %s: %w", path, err)
	}
	var raw any
	if err := yaml.Unmarshal(contents, &raw); err != nil {
		return Capability{}, fmt.Errorf("parse capability manifest %s: %w", path, err)
	}
	instance, err := normalizeYAML(raw)
	if err != nil {
		return Capability{}, fmt.Errorf("parse capability manifest %s: %w", path, err)
	}
	if err := schemas.Validate("capability.schema.json", instance); err != nil {
		return Capability{}, fmt.Errorf("validate capability manifest %s: %w", path, err)
	}
	encoded, err := yaml.Marshal(instance)
	if err != nil {
		return Capability{}, fmt.Errorf("decode capability manifest %s: %w", path, err)
	}
	var capability Capability
	if err := yaml.Unmarshal(encoded, &capability); err != nil {
		return Capability{}, fmt.Errorf("decode capability manifest %s: %w", path, err)
	}
	capability.ManifestPath, capability.Directory = path, directory
	for index, part := range capability.EntryPoint {
		if strings.TrimSpace(part) == "" {
			return Capability{}, fmt.Errorf("capability %q entrypoint[%d] must not be blank", capability.ID, index)
		}
	}
	return capability, nil
}

func normalizeYAML(value any) (any, error) {
	switch node := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(node))
		for key, item := range node {
			normalized, err := normalizeYAML(item)
			if err != nil {
				return nil, err
			}
			result[key] = normalized
		}
		return result, nil
	case map[any]any:
		result := make(map[string]any, len(node))
		for key, item := range node {
			text, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("manifest keys must be strings")
			}
			normalized, err := normalizeYAML(item)
			if err != nil {
				return nil, err
			}
			result[text] = normalized
		}
		return result, nil
	case []any:
		result := make([]any, len(node))
		for i, item := range node {
			normalized, err := normalizeYAML(item)
			if err != nil {
				return nil, err
			}
			result[i] = normalized
		}
		return result, nil
	default:
		return node, nil
	}
}

func validateCapabilitySchemas(capability Capability) error {
	for _, name := range []string{capability.InputSchema, capability.OutputSchema} {
		if filepath.IsAbs(name) || filepath.Clean(name) != name || name == "." || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("capability %q schema path %q must stay inside its directory", capability.ID, name)
		}
		root, err := filepath.EvalSymlinks(capability.Directory)
		if err != nil {
			return fmt.Errorf("resolve capability directory: %w", err)
		}
		path := filepath.Join(capability.Directory, name)
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("capability %q schema %q: %w", capability.ID, name, err)
		}
		rel, err := filepath.Rel(root, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("capability %q schema %q escapes its directory", capability.ID, name)
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			return fmt.Errorf("read capability %q schema %q: %w", capability.ID, name, err)
		}
		var schema map[string]any
		if err := json.Unmarshal(data, &schema); err != nil || schema == nil {
			return fmt.Errorf("capability %q schema %q must contain a JSON object", capability.ID, name)
		}
	}
	return nil
}
