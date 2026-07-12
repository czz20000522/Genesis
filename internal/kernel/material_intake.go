package kernel

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"genesis/internal/kernel/resource"
)

func (k *Kernel) IntakeMaterial(req MaterialIntakeRequest) (MaterialIntakeProjection, error) {
	kind := strings.TrimSpace(req.Locator.Kind)
	if kind == "" {
		return refusedMaterialIntake("invalid_locator", "locator.kind is required"), errors.New("locator.kind is required")
	}
	if kind != MaterialLocatorKindLocalPath {
		return refusedMaterialIntake("unsupported_locator", fmt.Sprintf("unsupported material locator kind %q", kind)), fmt.Errorf("unsupported material locator kind %q", kind)
	}
	purpose := strings.TrimSpace(req.Purpose)
	if purpose == "" {
		purpose = SourcePurposeAnalysis
	}
	descriptor, err := k.resourceRegistry.RegisterLocalZipSnapshot(req.Locator.Path, resource.SourceSnapshotOptions{
		Purpose:      purpose,
		SessionID:    strings.TrimSpace(req.SessionID),
		DisplayLabel: "",
	})
	if err != nil {
		reason := resource.SourceErrorReason(err)
		if reason == "" {
			reason = "material_intake_failed"
		}
		return refusedMaterialIntake(reason, err.Error()), err
	}
	return MaterialIntakeProjection{
		AdmissionResult:     "admitted",
		SourceSnapshotRef:   descriptor.SourceSnapshotRef,
		Root:                descriptor,
		AvailableOperations: append([]string(nil), descriptor.AvailableOperations...),
		Diagnostics:         append([]SourceDiagnostic(nil), descriptor.Diagnostics...),
	}, nil
}

func (k *Kernel) IntakeUploadedMaterial(sessionID string, purpose string, filename string, body io.Reader) (MaterialIntakeProjection, error) {
	if body == nil {
		return refusedMaterialIntake("invalid_upload", "upload file is required"), errors.New("upload file is required")
	}
	if err := os.MkdirAll(k.materialStorePath, 0o755); err != nil {
		return refusedMaterialIntake("object_store_unavailable", "material object store is unavailable"), err
	}
	tmp, err := os.CreateTemp(k.materialStorePath, "upload-*.zip")
	if err != nil {
		return refusedMaterialIntake("object_store_unavailable", "material object store is unavailable"), err
	}
	tmpPath := tmp.Name()
	keep := false
	defer func() {
		if !keep {
			_ = os.Remove(tmpPath)
		}
	}()
	uploadLimit := k.materialUploadByteLimit()
	written, copyErr := io.Copy(tmp, io.LimitReader(body, uploadLimit+1))
	closeErr := tmp.Close()
	if copyErr != nil {
		return refusedMaterialIntake("upload_read_failed", "upload body could not be read"), copyErr
	}
	if closeErr != nil {
		return refusedMaterialIntake("object_store_unavailable", "material object store write failed"), closeErr
	}
	if written > uploadLimit {
		return refusedMaterialIntake("upload_too_large", "upload exceeds material intake byte budget"), errors.New("upload exceeds material intake byte budget")
	}
	purpose = strings.TrimSpace(purpose)
	if purpose == "" {
		purpose = SourcePurposeAnalysis
	}
	descriptor, err := k.resourceRegistry.RegisterOwnedUploadZipSnapshot(tmpPath, resource.SourceSnapshotOptions{
		Purpose:      purpose,
		SessionID:    strings.TrimSpace(sessionID),
		DisplayLabel: safeDisplayFilename(filename),
	})
	if err != nil {
		reason := resource.SourceErrorReason(err)
		if reason == "" {
			reason = "material_intake_failed"
		}
		return refusedMaterialIntake(reason, err.Error()), err
	}
	if err := k.appendMaterialIntakeEvent(strings.TrimSpace(sessionID), descriptor); err != nil {
		if rollbackErr := k.resourceRegistry.RemoveDurableSourceSnapshot(descriptor.SourceSnapshotRef); rollbackErr != nil {
			k.setSourceSnapshotRecovery(ReadyCheck{Readiness: ReadinessNotReady, ReadinessReason: "source_snapshot_index_unavailable"})
		}
		return refusedMaterialIntake("ledger_unavailable", "material intake could not be recorded"), err
	}
	keep = true
	return MaterialIntakeProjection{
		AdmissionResult:     "admitted",
		SourceSnapshotRef:   descriptor.SourceSnapshotRef,
		Root:                descriptor,
		AvailableOperations: append([]string(nil), descriptor.AvailableOperations...),
		Diagnostics:         append([]SourceDiagnostic(nil), descriptor.Diagnostics...),
	}, nil
}

func (k *Kernel) appendMaterialIntakeEvent(sessionID string, descriptor SourceSnapshotDescriptor) error {
	now := k.clock()
	return k.appendEvent(StoredEvent{
		EventID:   newID("evt", now),
		SessionID: strings.TrimSpace(sessionID),
		Type:      "material.intake.admitted",
		CreatedAt: now,
		Data: EventData{
			SourceSnapshots: []SourceSnapshotDescriptor{descriptor},
		},
	})
}

func (k *Kernel) materialUploadByteLimit() int64 {
	policy := k.resourceRegistry.SourceSnapshotPolicy()
	return policy.MaxTotalUncompressedBytes + policy.MaxPerFileUncompressedBytes
}

func refusedMaterialIntake(reason string, message string) MaterialIntakeProjection {
	return MaterialIntakeProjection{
		AdmissionResult:    "refused",
		RefusalReasonClass: strings.TrimSpace(reason),
		Diagnostics: []SourceDiagnostic{{
			Code:    strings.TrimSpace(reason),
			Message: strings.TrimSpace(message),
		}},
	}
}

func safeDisplayFilename(filename string) string {
	filename = strings.TrimSpace(strings.ReplaceAll(filename, "\\", "/"))
	if filename == "" {
		return "upload.zip"
	}
	base := pathBase(filename)
	if base == "." || base == "/" || base == "" {
		return "upload.zip"
	}
	return base
}

func pathBase(filename string) string {
	base := filepath.Base(filename)
	if strings.Contains(filename, "/") {
		base = filepath.Base(strings.ReplaceAll(filename, "\\", "/"))
	}
	return strings.TrimSpace(base)
}
