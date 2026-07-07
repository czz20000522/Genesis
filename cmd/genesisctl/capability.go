package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type capabilityManifest struct {
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

type capabilityProjection struct {
	ID           string   `json:"id"`
	Name         string   `json:"name,omitempty"`
	Description  string   `json:"description,omitempty"`
	Root         string   `json:"root"`
	ManifestPath string   `json:"manifest_path"`
	Readiness    string   `json:"readiness"`
	Reason       string   `json:"reason,omitempty"`
	Entrypoint   string   `json:"entrypoint,omitempty"`
	Skill        string   `json:"skill,omitempty"`
	RuntimeRef   string   `json:"runtime_ref,omitempty"`
	Inputs       []string `json:"inputs,omitempty"`
	Outputs      []string `json:"outputs,omitempty"`
}

func runCapability(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("capability command is required: list, doctor, or run")
	}
	switch args[0] {
	case "list":
		return runCapabilityList(args[1:], stdout)
	case "doctor":
		return runCapabilityDoctor(args[1:], stdout)
	case "run":
		return runCapabilityRun(args[1:], stdout)
	default:
		return fmt.Errorf("unknown capability command %q", args[0])
	}
}

func runCapabilityList(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("capability list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", defaultCapabilityRoot(), "Genesis capability root")
	if err := fs.Parse(args); err != nil {
		return err
	}
	items, err := listCapabilityPackages(*root)
	if err != nil {
		return err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return encodeIndented(stdout, map[string]any{"items": items})
}

func runCapabilityDoctor(args []string, stdout io.Writer) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("capability id is required")
	}
	id := strings.TrimSpace(args[0])
	fs := flag.NewFlagSet("capability doctor", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", defaultCapabilityRoot(), "Genesis capability root")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	item := inspectCapabilityPackage(*root, id)
	if err := encodeIndented(stdout, item); err != nil {
		return err
	}
	if item.Readiness != "ready" {
		return fmt.Errorf("capability %s not ready: %s", id, item.Reason)
	}
	return nil
}

func runCapabilityRun(args []string, stdout io.Writer) error {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return errors.New("capability id is required")
	}
	id := strings.TrimSpace(args[0])
	fs := flag.NewFlagSet("capability run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", defaultCapabilityRoot(), "Genesis capability root")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	item := inspectCapabilityPackage(*root, id)
	if item.Readiness != "ready" {
		return fmt.Errorf("capability %s not ready: %s", id, item.Reason)
	}
	manifest, err := readCapabilityManifest(item.ManifestPath)
	if err != nil {
		return err
	}
	cmd, err := capabilityCommand(item.Root, manifest.Entrypoint, fs.Args())
	if err != nil {
		return err
	}
	cmd.Dir = item.Root
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func listCapabilityPackages(root string) ([]capabilityProjection, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("capability root is required")
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return []capabilityProjection{}, nil
	}
	if err != nil {
		return nil, err
	}
	var items []capabilityProjection
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		items = append(items, inspectCapabilityPackage(root, entry.Name()))
	}
	return items, nil
}

func inspectCapabilityPackage(root string, id string) capabilityProjection {
	root = strings.TrimSpace(root)
	id = strings.TrimSpace(id)
	packageRoot := filepath.Join(root, id)
	manifestPath := filepath.Join(packageRoot, "genesis.capability.json")
	item := capabilityProjection{
		ID:           id,
		Root:         packageRoot,
		ManifestPath: manifestPath,
		Readiness:    "not_ready",
	}
	manifest, err := readCapabilityManifest(manifestPath)
	if err != nil {
		item.Reason = "manifest_unavailable"
		return item
	}
	item.Name = manifest.Name
	item.Description = manifest.Description
	item.Entrypoint = manifest.Entrypoint
	item.Skill = manifest.Skill
	item.RuntimeRef = manifest.RuntimeRef
	item.Inputs = append([]string(nil), manifest.Inputs...)
	item.Outputs = append([]string(nil), manifest.Outputs...)
	if manifest.ID != id {
		item.Reason = "manifest_id_mismatch"
		return item
	}
	if strings.TrimSpace(manifest.Entrypoint) == "" {
		item.Reason = "entrypoint_missing"
		return item
	}
	if !safeCapabilityRelativePath(manifest.Entrypoint) {
		item.Reason = "entrypoint_unsafe"
		return item
	}
	if _, err := os.Stat(filepath.Join(packageRoot, filepath.FromSlash(manifest.Entrypoint))); err != nil {
		item.Reason = "entrypoint_unavailable"
		return item
	}
	if strings.TrimSpace(manifest.Skill) != "" {
		if !safeCapabilityRelativePath(manifest.Skill) {
			item.Reason = "skill_unsafe"
			return item
		}
		if _, err := os.Stat(filepath.Join(packageRoot, filepath.FromSlash(manifest.Skill))); err != nil {
			item.Reason = "skill_unavailable"
			return item
		}
	}
	item.Readiness = "ready"
	item.Reason = ""
	return item
}

func readCapabilityManifest(path string) (capabilityManifest, error) {
	var manifest capabilityManifest
	payload, err := os.ReadFile(path)
	if err != nil {
		return manifest, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return manifest, err
	}
	manifest.ID = strings.TrimSpace(manifest.ID)
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Description = strings.TrimSpace(manifest.Description)
	manifest.RuntimeRef = strings.TrimSpace(manifest.RuntimeRef)
	manifest.Entrypoint = strings.TrimSpace(manifest.Entrypoint)
	manifest.Skill = strings.TrimSpace(manifest.Skill)
	manifest.DataDir = strings.TrimSpace(manifest.DataDir)
	return manifest, nil
}

func capabilityCommand(packageRoot string, entrypoint string, args []string) (*exec.Cmd, error) {
	entry := filepath.Join(packageRoot, filepath.FromSlash(strings.TrimSpace(entrypoint)))
	switch strings.ToLower(filepath.Ext(entry)) {
	case ".ps1":
		command, err := powerShellCommand()
		if err != nil {
			return nil, err
		}
		cmdArgs := append([]string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File", entry}, args...)
		return exec.Command(command, cmdArgs...), nil
	default:
		return exec.Command(entry, args...), nil
	}
}

func powerShellCommand() (string, error) {
	command := "pwsh.exe"
	if _, err := exec.LookPath(command); err == nil {
		return command, nil
	}
	return "", errors.New("pwsh.exe not found")
}

func safeCapabilityRelativePath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" || filepath.IsAbs(path) {
		return false
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, ".."+string(filepath.Separator))
}

func defaultCapabilityRoot() string {
	if root := strings.TrimSpace(os.Getenv("GENESIS_CAPABILITY_ROOT")); root != "" {
		return root
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".genesis", "capabilities")
	}
	return filepath.Join(".genesis", "capabilities")
}

func encodeIndented(stdout io.Writer, payload any) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}
