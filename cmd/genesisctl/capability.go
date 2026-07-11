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

	"genesis/internal/capabilitypackage"
)

type capabilityManifest = capabilitypackage.Manifest
type capabilityProjection = capabilitypackage.Projection

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
	return capabilitypackage.Discover(root)
}

func inspectCapabilityPackage(root string, id string) capabilityProjection {
	return capabilitypackage.Inspect(root, id)
}

func readCapabilityManifest(path string) (capabilityManifest, error) {
	return capabilitypackage.ReadManifest(path)
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
	return capabilitypackage.SafeRelativePath(path)
}

func defaultCapabilityRoot() string {
	return capabilitypackage.DefaultRoot()
}

func encodeIndented(stdout io.Writer, payload any) error {
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}
