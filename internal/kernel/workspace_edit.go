package kernel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type workspaceEditRequest struct {
	RelativePath string
	AbsolutePath string
	Edits        []workspaceEditChange
}

type workspaceEditChange struct {
	OldString string
	NewString string
}

type WorkspaceEditResult struct {
	Status       string `json:"status"`
	Tool         string `json:"tool"`
	Executed     bool   `json:"executed"`
	Path         string `json:"path"`
	Replacements int    `json:"replacements"`
	BytesBefore  int    `json:"bytes_before"`
	BytesAfter   int    `json:"bytes_after"`
}

func (k *Kernel) admitWorkspaceEditRequest(args workspaceEditToolArguments) (workspaceEditRequest, string, error) {
	return k.admitWorkspaceEditRequestWithRoot(k.toolPolicy.WorkspaceRoot, args)
}

func (k *Kernel) admitWorkspaceEditRequestWithRoot(workspaceRoot string, args workspaceEditToolArguments) (workspaceEditRequest, string, error) {
	relativePath, absolutePath, code, err := resolveWorkspaceEditPath(workspaceRoot, args.Path)
	if err != nil {
		return workspaceEditRequest{}, code, err
	}
	edits, code, err := workspaceEditChanges(args)
	if err != nil {
		return workspaceEditRequest{}, code, err
	}
	return workspaceEditRequest{
		RelativePath: relativePath,
		AbsolutePath: absolutePath,
		Edits:        edits,
	}, "", nil
}

func workspaceEditChanges(args workspaceEditToolArguments) ([]workspaceEditChange, string, error) {
	if len(args.Edits) > 0 {
		edits := make([]workspaceEditChange, 0, len(args.Edits))
		for _, edit := range args.Edits {
			if edit.OldString == "" {
				return nil, "workspace_edit_old_string_required", errors.New("old_string is required")
			}
			edits = append(edits, workspaceEditChange{
				OldString: edit.OldString,
				NewString: edit.NewString,
			})
		}
		return edits, "", nil
	}
	if args.OldString == "" {
		return nil, "workspace_edit_old_string_required", errors.New("old_string is required")
	}
	return []workspaceEditChange{{
		OldString: args.OldString,
		NewString: args.NewString,
	}}, "", nil
}

func resolveWorkspaceEditPath(workspaceRoot string, requestedPath string) (string, string, string, error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return "", "", "workspace_root_required", errors.New("workspace root is required")
	}
	if !filepath.IsAbs(root) {
		return "", "", "workspace_root_required", errors.New("workspace root must be absolute")
	}
	path := filepath.Clean(strings.TrimSpace(requestedPath))
	if path == "" || path == "." {
		return "", "", "invalid_workspace_edit_path", errors.New("path is required")
	}
	if filepath.IsAbs(path) {
		return "", "", "invalid_workspace_edit_path", errors.New("path must be relative to the workspace root")
	}
	cleanRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", "workspace_root_required", errors.New("workspace root is unavailable")
	}
	cleanRoot = filepath.Clean(cleanRoot)
	candidate := filepath.Clean(filepath.Join(cleanRoot, path))
	relative, err := filepath.Rel(cleanRoot, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", "path_outside_workspace", errors.New("path is outside the workspace")
	}
	realCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "", "workspace_edit_target_missing", errors.New("workspace edit target is missing")
		}
		return "", "", "invalid_workspace_edit_path", errors.New("workspace edit target cannot be resolved")
	}
	realCandidate = filepath.Clean(realCandidate)
	realRelative, err := filepath.Rel(cleanRoot, realCandidate)
	if err != nil || realRelative == ".." || strings.HasPrefix(realRelative, ".."+string(filepath.Separator)) {
		return "", "", "path_outside_workspace", errors.New("path is outside the workspace")
	}
	if realRelative == "." {
		return "", "", "invalid_workspace_edit_path", errors.New("path must identify a file")
	}
	return filepath.ToSlash(realRelative), realCandidate, "", nil
}

func (k *Kernel) workspaceEditModelToolResult(eventID string, providerCallID string, name string, req workspaceEditRequest) (ModelToolResult, error) {
	result, code, err := applyWorkspaceEdit(req)
	if err != nil {
		return invalidModelToolResult(eventID, providerCallID, name, code, fmt.Sprintf("invalid workspace_edit request: %v", err))
	}
	content, err := json.Marshal(result)
	if err != nil {
		return ModelToolResult{}, err
	}
	return ModelToolResult{
		ToolCallID:      strings.TrimSpace(providerCallID),
		ToolCallEventID: strings.TrimSpace(eventID),
		Name:            strings.TrimSpace(name),
		Content:         string(content),
	}, nil
}

func applyWorkspaceEdit(req workspaceEditRequest) (WorkspaceEditResult, string, error) {
	info, err := os.Stat(req.AbsolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return WorkspaceEditResult{}, "workspace_edit_target_missing", errors.New("workspace edit target is missing")
		}
		return WorkspaceEditResult{}, "workspace_edit_read_failed", errors.New("workspace edit target cannot be inspected")
	}
	if info.IsDir() {
		return WorkspaceEditResult{}, "workspace_edit_target_not_file", errors.New("workspace edit target is not a file")
	}
	contentBytes, err := os.ReadFile(req.AbsolutePath)
	if err != nil {
		return WorkspaceEditResult{}, "workspace_edit_read_failed", errors.New("workspace edit target cannot be read")
	}
	content := string(contentBytes)
	updated := content
	replacements := 0
	for _, edit := range req.Edits {
		count := strings.Count(updated, edit.OldString)
		switch count {
		case 0:
			return WorkspaceEditResult{}, "workspace_edit_old_string_not_found", errors.New("old_string was not found")
		case 1:
			updated = strings.Replace(updated, edit.OldString, edit.NewString, 1)
			replacements++
		default:
			return WorkspaceEditResult{}, "workspace_edit_old_string_not_unique", errors.New("old_string is not unique")
		}
	}
	updatedBytes := []byte(updated)
	if err := os.WriteFile(req.AbsolutePath, updatedBytes, info.Mode().Perm()); err != nil {
		return WorkspaceEditResult{}, "workspace_edit_write_failed", errors.New("workspace edit target cannot be written")
	}
	return WorkspaceEditResult{
		Status:       "completed",
		Tool:         "workspace_edit",
		Executed:     true,
		Path:         req.RelativePath,
		Replacements: replacements,
		BytesBefore:  len(contentBytes),
		BytesAfter:   len(updatedBytes),
	}, "", nil
}
