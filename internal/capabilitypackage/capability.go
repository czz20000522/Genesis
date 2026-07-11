// Package capabilitypackage inspects user-space Genesis capability packages.
package capabilitypackage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Manifest struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	RuntimeRef  string   `json:"runtime_ref,omitempty"`
	Entrypoint  string   `json:"entrypoint"`
	Skill       string   `json:"skill,omitempty"`
	DataDir     string   `json:"data_dir,omitempty"`
	Inputs      []string `json:"inputs,omitempty"`
	Outputs     []string `json:"outputs,omitempty"`
}

type Projection struct {
	ID           string   `json:"id"`
	Name         string   `json:"name,omitempty"`
	Description  string   `json:"description,omitempty"`
	Readiness    string   `json:"readiness"`
	Reason       string   `json:"reason,omitempty"`
	RuntimeRef   string   `json:"runtime_ref,omitempty"`
	Inputs       []string `json:"inputs,omitempty"`
	Outputs      []string `json:"outputs,omitempty"`
	Root         string   `json:"-"`
	ManifestPath string   `json:"-"`
	Entrypoint   string   `json:"-"`
	Skill        string   `json:"-"`
}

func Discover(root string) ([]Projection, error) {
	if root = strings.TrimSpace(root); root == "" {
		return nil, errors.New("capability root is required")
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []Projection{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]Projection, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			items = append(items, Inspect(root, entry.Name()))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func DefaultRoot() string {
	if root := strings.TrimSpace(os.Getenv("GENESIS_CAPABILITY_ROOT")); root != "" {
		return root
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".genesis", "capabilities")
	}
	return filepath.Join(".genesis", "capabilities")
}

func Inspect(root, id string) Projection {
	root, id = strings.TrimSpace(root), strings.TrimSpace(id)
	pkg := filepath.Join(root, id)
	item := Projection{ID: id, Root: pkg, ManifestPath: filepath.Join(pkg, "genesis.capability.json"), Readiness: "not_ready"}
	manifest, err := ReadManifest(item.ManifestPath)
	if err != nil {
		item.Reason = "manifest_unavailable"
		return item
	}
	item.Name, item.Description, item.RuntimeRef = manifest.Name, manifest.Description, manifest.RuntimeRef
	item.Entrypoint, item.Skill, item.Inputs, item.Outputs = manifest.Entrypoint, manifest.Skill, append([]string(nil), manifest.Inputs...), append([]string(nil), manifest.Outputs...)
	if manifest.ID != id {
		item.Reason = "manifest_id_mismatch"
		return item
	}
	if manifest.Entrypoint == "" {
		item.Reason = "entrypoint_missing"
		return item
	}
	if !SafeRelativePath(manifest.Entrypoint) {
		item.Reason = "entrypoint_unsafe"
		return item
	}
	if _, err := os.Stat(filepath.Join(pkg, filepath.FromSlash(manifest.Entrypoint))); err != nil {
		item.Reason = "entrypoint_unavailable"
		return item
	}
	if manifest.Skill != "" {
		if !SafeRelativePath(manifest.Skill) {
			item.Reason = "skill_unsafe"
			return item
		}
		if _, err := os.Stat(filepath.Join(pkg, filepath.FromSlash(manifest.Skill))); err != nil {
			item.Reason = "skill_unavailable"
			return item
		}
	}
	item.Readiness = "ready"
	item.Reason = ""
	return item
}

func ReadManifest(path string) (Manifest, error) {
	var manifest Manifest
	payload, err := os.ReadFile(path)
	if err != nil {
		return manifest, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, err
	}
	manifest.ID, manifest.Name, manifest.Description = strings.TrimSpace(manifest.ID), strings.TrimSpace(manifest.Name), strings.TrimSpace(manifest.Description)
	manifest.RuntimeRef, manifest.Entrypoint, manifest.Skill, manifest.DataDir = strings.TrimSpace(manifest.RuntimeRef), strings.TrimSpace(manifest.Entrypoint), strings.TrimSpace(manifest.Skill), strings.TrimSpace(manifest.DataDir)
	return manifest, nil
}

func SafeRelativePath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}
