package kernel

import (
	"errors"
	"path/filepath"
	"strings"
)

const (
	SessionWorkspaceKindProject = "project"
	SessionWorkspaceKindTask    = "task"
	SessionWorkspaceKindNone    = "none"
)

var (
	ErrSessionWorkspaceInvalid      = errors.New("session workspace binding invalid")
	ErrSessionWorkspaceAlreadyBound = errors.New("session workspace already bound")
)

type SessionWorkspaceBindingRequest struct {
	Kind string
	Root string
}

type SessionWorkspaceBinding struct {
	Kind string `json:"kind"`
	Root string `json:"root"`
}

func (k *Kernel) BindSessionWorkspace(sessionID string, request SessionWorkspaceBindingRequest) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return ErrSessionWorkspaceInvalid
	}
	binding, err := normalizedSessionWorkspaceBinding(request)
	if err != nil {
		return err
	}
	events, err := k.loadEvents()
	if err != nil {
		return err
	}
	if existing, ok := sessionWorkspaceBindingFromEvents(events, sessionID); ok {
		if existing == binding {
			return nil
		}
		return ErrSessionWorkspaceAlreadyBound
	}
	now := k.clock()
	return k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: sessionID,
		Type:      "session.workspace_bound",
		CreatedAt: now,
		Data:      EventData{SessionWorkspace: &binding},
	})
}

func normalizedSessionWorkspaceBinding(request SessionWorkspaceBindingRequest) (SessionWorkspaceBinding, error) {
	kind := strings.ToLower(strings.TrimSpace(request.Kind))
	root := strings.TrimSpace(request.Root)
	if kind == SessionWorkspaceKindNone {
		if root != "" {
			return SessionWorkspaceBinding{}, ErrSessionWorkspaceInvalid
		}
		return SessionWorkspaceBinding{Kind: kind}, nil
	}
	if kind != SessionWorkspaceKindProject && kind != SessionWorkspaceKindTask {
		return SessionWorkspaceBinding{}, ErrSessionWorkspaceInvalid
	}
	if !filepath.IsAbs(root) {
		return SessionWorkspaceBinding{}, ErrSessionWorkspaceInvalid
	}
	canonicalRoot, err := canonicalExistingPath(root)
	if err != nil {
		return SessionWorkspaceBinding{}, ErrSessionWorkspaceInvalid
	}
	return SessionWorkspaceBinding{Kind: kind, Root: canonicalRoot}, nil
}

func sessionWorkspaceBindingFromEvents(events []StoredEvent, sessionID string) (SessionWorkspaceBinding, bool) {
	for _, event := range events {
		if event.SessionID != sessionID || event.Type != "session.workspace_bound" || event.Data.SessionWorkspace == nil {
			continue
		}
		return *event.Data.SessionWorkspace, true
	}
	return SessionWorkspaceBinding{}, false
}
