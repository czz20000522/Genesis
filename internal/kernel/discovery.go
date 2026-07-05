package kernel

import (
	"errors"
	"fmt"
	"strings"
)

const (
	defaultDiscoveryLimit = 5
	maxDiscoveryLimit     = 20
	maxDiscoveryTextRunes = 1000
)

func (k *Kernel) DiscoverContext(req DiscoveryQueryRequest) (DiscoveryQueryResponse, error) {
	query, requestedKinds, limit, err := normalizeDiscoveryQuery(req)
	if err != nil {
		return DiscoveryQueryResponse{}, err
	}
	candidates := make([]DiscoveryCandidateProjection, 0, limit)
	memories, err := k.MemoryCandidates(MemoryCandidateApproved)
	if err != nil {
		return DiscoveryQueryResponse{}, err
	}
	for _, memory := range memories {
		if len(candidates) >= limit {
			break
		}
		if !discoveryKindRequested(requestedKinds, memory.Kind) {
			continue
		}
		if !discoveryTextMatches(query, strings.Join([]string{memory.Text, memory.AppliesWhen, memory.Kind, memory.Scope}, "\n")) {
			continue
		}
		candidates = append(candidates, discoveryCandidateFromMemory(memory))
	}
	if discoveryKindRequested(requestedKinds, MemoryKindCapabilityHint) {
		for _, descriptor := range k.capabilityDescriptors {
			if len(candidates) >= limit {
				break
			}
			if !discoveryCapabilityDescriptorMatches(query, descriptor) {
				continue
			}
			candidates = append(candidates, discoveryCandidateFromCapabilityDescriptor(descriptor))
		}
		for _, skill := range k.skillCatalogProjection().Items {
			if len(candidates) >= limit {
				break
			}
			if !discoveryTextMatches(query, strings.Join([]string{skill.Name, skill.Description}, "\n")) {
				continue
			}
			candidates = append(candidates, discoveryCandidateFromSkill(skill))
		}
	}
	return DiscoveryQueryResponse{Candidates: candidates}, nil
}

func normalizeCapabilityDescriptors(descriptors []CapabilityDescriptor) ([]CapabilityDescriptor, error) {
	out := make([]CapabilityDescriptor, 0, len(descriptors))
	seen := map[string]bool{}
	for _, descriptor := range descriptors {
		next := CapabilityDescriptor{
			CapabilityRef: strings.TrimSpace(descriptor.CapabilityRef),
			Name:          oneLine(descriptor.Name),
			Summary:       oneLine(descriptor.Summary),
			Intents:       normalizedDiscoveryStringList(descriptor.Intents),
			InputSummary:  oneLine(descriptor.InputSummary),
			HealthSummary: oneLine(descriptor.HealthSummary),
		}
		if next.CapabilityRef == "" {
			return nil, errors.New("capability_descriptor capability_ref is required")
		}
		if next.Name == "" {
			return nil, errors.New("capability_descriptor name is required")
		}
		if strings.ContainsAny(next.CapabilityRef, `\/`) {
			return nil, fmt.Errorf("capability_descriptor %q must not contain host path separators", next.CapabilityRef)
		}
		if seen[next.CapabilityRef] {
			return nil, fmt.Errorf("duplicate capability_descriptor ref %q", next.CapabilityRef)
		}
		seen[next.CapabilityRef] = true
		out = append(out, next)
	}
	return out, nil
}

func normalizeDiscoveryQuery(req DiscoveryQueryRequest) (string, map[string]bool, int, error) {
	intent := strings.TrimSpace(req.Intent)
	if intent == "" {
		return "", nil, 0, errors.New("intent is required")
	}
	if len([]rune(intent)) > maxDiscoveryTextRunes {
		return "", nil, 0, fmt.Errorf("intent must be at most %d characters", maxDiscoveryTextRunes)
	}
	contextSummary := strings.TrimSpace(req.CurrentContextSummary)
	if len([]rune(contextSummary)) > maxDiscoveryTextRunes {
		return "", nil, 0, fmt.Errorf("current_context_summary must be at most %d characters", maxDiscoveryTextRunes)
	}
	limit := req.Limit
	if limit == 0 {
		limit = defaultDiscoveryLimit
	}
	if limit < 0 {
		return "", nil, 0, errors.New("limit must not be negative")
	}
	if limit > maxDiscoveryLimit {
		limit = maxDiscoveryLimit
	}
	requestedKinds := map[string]bool{}
	for _, kind := range req.RequestedKinds {
		kind = strings.TrimSpace(kind)
		if kind == "" {
			continue
		}
		if !validMemoryKind(kind) {
			return "", nil, 0, fmt.Errorf("unsupported requested kind %q", kind)
		}
		requestedKinds[kind] = true
	}
	query := strings.TrimSpace(intent + "\n" + contextSummary)
	return query, requestedKinds, limit, nil
}

func normalizedDiscoveryStringList(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		item = oneLine(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func discoveryKindRequested(requested map[string]bool, kind string) bool {
	kind = strings.TrimSpace(kind)
	if kind == "" {
		return false
	}
	if len(requested) == 0 {
		return true
	}
	return requested[kind]
}

func discoveryTextMatches(query string, text string) bool {
	query = strings.ToLower(oneLine(query))
	text = strings.ToLower(oneLine(text))
	if query == "" || text == "" {
		return false
	}
	if strings.Contains(text, query) {
		return true
	}
	for _, token := range strings.Fields(query) {
		if len([]rune(token)) < 2 {
			continue
		}
		if strings.Contains(text, token) {
			return true
		}
	}
	return false
}

func discoveryCandidateFromMemory(memory MemoryCandidateProjection) DiscoveryCandidateProjection {
	return DiscoveryCandidateProjection{
		Ref:           "memory:" + strings.TrimSpace(memory.CandidateID),
		Kind:          strings.TrimSpace(memory.Kind),
		Summary:       boundedTimelinePreview(memory.Text),
		Scope:         strings.TrimSpace(memory.Scope),
		AppliesWhen:   strings.TrimSpace(memory.AppliesWhen),
		Confidence:    discoveryConfidence(memory.Strength),
		SourceSummary: memoryDiscoverySourceSummary(memory),
	}
}

func discoveryCandidateFromSkill(skill SkillCatalogItemProjection) DiscoveryCandidateProjection {
	name := strings.TrimSpace(skill.Name)
	summary := oneLine(skill.Description)
	if summary == "" {
		summary = name
	}
	return DiscoveryCandidateProjection{
		Ref:           "capability:" + name,
		Kind:          MemoryKindCapabilityHint,
		Summary:       boundedTimelinePreview(summary),
		Scope:         MemoryScopeCapability,
		Confidence:    "medium",
		SourceSummary: "skill catalog descriptor",
	}
}

func discoveryCapabilityDescriptorMatches(query string, descriptor CapabilityDescriptor) bool {
	return discoveryTextMatches(query, strings.Join([]string{
		descriptor.Name,
		descriptor.Summary,
		strings.Join(descriptor.Intents, " "),
		descriptor.InputSummary,
		descriptor.HealthSummary,
	}, "\n"))
}

func discoveryCandidateFromCapabilityDescriptor(descriptor CapabilityDescriptor) DiscoveryCandidateProjection {
	summary := descriptor.Summary
	if summary == "" {
		summary = descriptor.Name
	}
	return DiscoveryCandidateProjection{
		Ref:           strings.TrimSpace(descriptor.CapabilityRef),
		Kind:          MemoryKindCapabilityHint,
		Summary:       boundedTimelinePreview(summary),
		Scope:         MemoryScopeCapability,
		AppliesWhen:   boundedTimelinePreview(strings.Join(descriptor.Intents, "; ")),
		Confidence:    "medium",
		SourceSummary: capabilityDescriptorSourceSummary(descriptor),
	}
}

func discoveryConfidence(strength string) string {
	switch strings.TrimSpace(strength) {
	case MemoryStrengthStrongRule:
		return "high"
	case MemoryStrengthPreference, MemoryStrengthContractHint:
		return "medium"
	default:
		return "low"
	}
}

func capabilityDescriptorSourceSummary(descriptor CapabilityDescriptor) string {
	parts := []string{"capability descriptor"}
	if health := oneLine(descriptor.HealthSummary); health != "" {
		parts = append(parts, "health="+health)
	}
	if input := oneLine(descriptor.InputSummary); input != "" {
		parts = append(parts, "inputs="+input)
	}
	return strings.Join(parts, "; ")
}

func memoryDiscoverySourceSummary(memory MemoryCandidateProjection) string {
	sourceRef := strings.TrimSpace(memory.SourceRef)
	if sourceRef == "" {
		return ""
	}
	return "approved memory from " + sourceRef
}
