package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type DesktopCatalogProjection struct {
	Projects []DesktopProjectCatalogProjection `json:"projects"`
	Sessions []DesktopSessionCatalogProjection `json:"sessions"`
}

type DesktopProjectCatalogProjection struct {
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	Root      string `json:"root"`
}

type DesktopSessionCatalogProjection struct {
	SessionID string `json:"sessionId"`
	Kind      string `json:"kind"`
	ProjectID string `json:"projectId,omitempty"`
	Root      string `json:"root,omitempty"`
	Name      string `json:"name,omitempty"`
}

var desktopCatalogHomeDir = os.UserHomeDir

func (a *App) LoadDesktopCatalog() (DesktopCatalogProjection, error) {
	return loadDesktopCatalog()
}

func (a *App) SaveDesktopCatalog(catalog DesktopCatalogProjection) error {
	return saveDesktopCatalog(catalog)
}

func loadDesktopCatalog() (DesktopCatalogProjection, error) {
	path, err := desktopCatalogPath()
	if err != nil {
		return DesktopCatalogProjection{}, err
	}
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DesktopCatalogProjection{Projects: []DesktopProjectCatalogProjection{}, Sessions: []DesktopSessionCatalogProjection{}}, nil
	}
	if err != nil {
		return DesktopCatalogProjection{}, err
	}
	var catalog DesktopCatalogProjection
	if err := json.Unmarshal(payload, &catalog); err != nil {
		return DesktopCatalogProjection{}, err
	}
	return normalizedDesktopCatalog(catalog), nil
}

func saveDesktopCatalog(catalog DesktopCatalogProjection) error {
	path, err := desktopCatalogPath()
	if err != nil {
		return err
	}
	catalog = normalizedDesktopCatalog(catalog)
	payload, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".catalog-*.json")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.Write(append(payload, '\n')); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func desktopCatalogPath() (string, error) {
	home, err := desktopCatalogHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return "", errors.New("Genesis home directory is unavailable")
	}
	return filepath.Join(home, ".genesis", "desktop", "catalog.json"), nil
}

func normalizedDesktopCatalog(catalog DesktopCatalogProjection) DesktopCatalogProjection {
	projects := make([]DesktopProjectCatalogProjection, 0, len(catalog.Projects))
	seenProjects := map[string]bool{}
	seenRoots := map[string]bool{}
	for _, project := range catalog.Projects {
		project.ProjectID = strings.TrimSpace(project.ProjectID)
		project.Name = strings.TrimSpace(project.Name)
		project.Root = strings.TrimSpace(project.Root)
		rootKey := strings.ToLower(filepath.Clean(project.Root))
		if project.ProjectID == "" || project.Name == "" || project.Root == "" || seenProjects[project.ProjectID] || seenRoots[rootKey] {
			continue
		}
		seenProjects[project.ProjectID] = true
		seenRoots[rootKey] = true
		projects = append(projects, project)
	}
	sessions := make([]DesktopSessionCatalogProjection, 0, len(catalog.Sessions))
	seenSessions := map[string]bool{}
	for _, session := range catalog.Sessions {
		session.SessionID = strings.TrimSpace(session.SessionID)
		session.Kind = strings.TrimSpace(session.Kind)
		session.ProjectID = strings.TrimSpace(session.ProjectID)
		session.Root = strings.TrimSpace(session.Root)
		session.Name = strings.TrimSpace(session.Name)
		if session.SessionID == "" || seenSessions[session.SessionID] {
			continue
		}
		if session.Kind != "project" && session.Kind != "task" && session.Kind != "chat" {
			continue
		}
		if session.Kind == "task" && session.Root == "" {
			continue
		}
		if session.Kind == "project" && session.ProjectID == "" && session.Root == "" {
			continue
		}
		seenSessions[session.SessionID] = true
		sessions = append(sessions, session)
	}
	return DesktopCatalogProjection{Projects: projects, Sessions: sessions}
}
