package kernel

import (
	"regexp"
	"strings"
)

var credentialShapedInspectionTokenPattern = regexp.MustCompile(`(?i)((^|[._-])sk-(proj-)?[A-Za-z0-9_-]{6,}($|[._-])|^[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}$)`)

func (k *Kernel) toolCapabilityProjections() []ToolCapabilityProjection {
	return k.toolGateway().CapabilityProjections()
}

func safeProviderStatusForInspection(status ProviderStatus) ProviderStatus {
	return ProviderStatus{
		Name:            safeInspectionToken(status.Name, "provider"),
		Readiness:       safeInspectionReadiness(status.Readiness),
		ReadinessReason: safeInspectionReadinessReason(status.ReadinessReason),
	}
}

func safeInspectionReadiness(readiness string) string {
	switch strings.TrimSpace(readiness) {
	case ReadinessReady:
		return ReadinessReady
	default:
		return ReadinessNotReady
	}
}

func safeInspectionReadinessReason(reason string) string {
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
	if value == "" || len(value) > 120 || containsCredentialShapedText(value) || credentialShapedInspectionTokenPattern.MatchString(value) {
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
	exclusions := make([]SkillCatalogExclusionProjection, 0, len(k.skillExclusions))
	exclusions = append(exclusions, k.skillExclusions...)
	roots := make([]SkillCatalogRootProjection, 0, len(k.skillRoots))
	roots = append(roots, k.skillRoots...)
	return SkillCatalogProjection{
		Status:     status,
		Count:      len(items),
		Items:      items,
		Roots:      roots,
		Exclusions: exclusions,
		Warnings:   skillIndexWarnings(items, k.contextPolicy.SkillIndexChars),
	}
}
