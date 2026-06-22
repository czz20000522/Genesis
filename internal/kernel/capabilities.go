package kernel

import (
	"regexp"
	"strings"
)

var credentialShapedInspectionTokenPattern = regexp.MustCompile(`(?i)((^|[._-])sk-(proj-)?[A-Za-z0-9_-]{6,}($|[._-])|^[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}$)`)

func (k *Kernel) toolCapabilityProjections() []ToolCapabilityProjection {
	descriptors := k.modelToolDescriptors()
	projections := make([]ToolCapabilityProjection, 0, len(descriptors))
	for _, descriptor := range descriptors {
		projections = append(projections, ToolCapabilityProjection{
			Name:   descriptor.Name,
			Kind:   toolCapabilityKind(descriptor.Name),
			Status: "ok",
		})
	}
	return projections
}

func toolCapabilityKind(name string) string {
	switch name {
	case "shell.exec":
		return "effect"
	case "skill.read":
		return "read"
	default:
		return "unknown"
	}
}

func safeProviderStatusForInspection(status ProviderStatus) ProviderStatus {
	return ProviderStatus{
		Name:   safeInspectionToken(status.Name, "provider"),
		Status: safeInspectionStatus(status.Status),
		Reason: safeInspectionReason(status.Reason),
	}
}

func safeInspectionStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "ok":
		return "ok"
	default:
		return "blocked"
	}
}

func safeInspectionReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(reason), "provider_") {
		return "provider_status_unavailable"
	}
	return safeInspectionToken(reason, "provider_status_unavailable")
}

func safeInspectionToken(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 120 || redactEvidenceText(value) != value || credentialShapedInspectionTokenPattern.MatchString(value) {
		return fallback
	}
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
			continue
		case char >= 'A' && char <= 'Z':
			continue
		case char >= '0' && char <= '9':
			continue
		case char == '.', char == '_', char == '-':
			continue
		default:
			return fallback
		}
	}
	return value
}

func (k *Kernel) skillCatalogProjection() SkillCatalogProjection {
	items := make([]SkillCatalogItemProjection, 0, len(k.skillCatalog))
	for _, skill := range k.skillCatalog {
		items = append(items, SkillCatalogItemProjection{
			Name:        skill.Name,
			Description: skill.Description,
		})
	}
	status := "ok"
	if len(items) == 0 {
		status = "empty"
	}
	exclusions := append([]SkillCatalogExclusionProjection(nil), k.skillExclusions...)
	return SkillCatalogProjection{
		Status:     status,
		Count:      len(items),
		Items:      items,
		Exclusions: exclusions,
	}
}
