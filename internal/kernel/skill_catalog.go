package kernel

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxSkillCatalogScanDepth     = 3
	maxSkillCatalogCandidates    = 200
	maxSkillCatalogMetadataBytes = 64 * 1024
)

func loadSkillCatalog(roots []string) []SkillDescriptor {
	return loadSkillCatalogWithDiagnostics(roots).Items
}

type skillCatalogLoadResult struct {
	Items      []SkillDescriptor
	Roots      []SkillCatalogRootProjection
	Exclusions []SkillCatalogExclusionProjection
}

type skillCatalogExclusionCounter map[string]int

func loadSkillCatalogWithDiagnostics(roots []string) skillCatalogLoadResult {
	var skills []SkillDescriptor
	var rootProjections []SkillCatalogRootProjection
	exclusions := skillCatalogExclusionCounter{}
	for rootOrdinal, root := range roots {
		cleanRoot := strings.TrimSpace(root)
		if cleanRoot == "" {
			continue
		}
		absRoot, err := filepath.Abs(expandHome(cleanRoot))
		if err != nil {
			exclusions.add("root_missing")
			rootProjections = append(rootProjections, SkillCatalogRootProjection{Ordinal: rootOrdinal, Status: ReadinessNotReady, Reason: "root_missing"})
			continue
		}
		info, err := os.Stat(absRoot)
		if err != nil || !info.IsDir() {
			exclusions.add("root_missing")
			rootProjections = append(rootProjections, SkillCatalogRootProjection{Ordinal: rootOrdinal, Status: ReadinessNotReady, Reason: "root_missing"})
			continue
		}
		if pathHasLinkOrReparsePoint(absRoot) {
			exclusions.add("root_linked")
			rootProjections = append(rootProjections, SkillCatalogRootProjection{Ordinal: rootOrdinal, Status: ReadinessNotReady, Reason: "root_linked"})
			continue
		}
		candidateCount := 0
		rootSkillCount := 0
		_ = filepath.WalkDir(absRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			if path != absRoot {
				rel, err := filepath.Rel(absRoot, path)
				if err != nil {
					exclusions.add("path_invalid")
					if entry.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if entry.IsDir() && pathDepth(rel) > maxSkillCatalogScanDepth {
					exclusions.add("scan_depth_exceeded")
					return filepath.SkipDir
				}
			}
			if entry.IsDir() || entry.Name() != "SKILL.md" {
				return nil
			}
			if candidateCount >= maxSkillCatalogCandidates {
				exclusions.add("scan_count_exceeded")
				return filepath.SkipAll
			}
			candidateCount++
			if pathHasLinkOrReparsePoint(path) || !pathWithin(path, absRoot) {
				exclusions.add("path_linked")
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				exclusions.add("read_failed")
				return nil
			}
			if info.Size() > maxSkillCatalogMetadataBytes {
				exclusions.add("skill_file_too_large")
				return nil
			}
			payload, err := os.ReadFile(path)
			if err != nil {
				exclusions.add("read_failed")
				return nil
			}
			name, description, ok := parseSkillMetadata(string(payload))
			if !ok {
				exclusions.add("metadata_invalid")
				return nil
			}
			if !isSafeSkillMetadata(name, description) {
				exclusions.add("metadata_unsafe")
				return nil
			}
			instructionPath, err := filepath.Abs(path)
			if err != nil {
				exclusions.add("path_invalid")
				return nil
			}
			skills = append(skills, SkillDescriptor{
				Name:            name,
				Description:     description,
				InstructionPath: filepath.Clean(instructionPath),
				RootOrdinal:     rootOrdinal,
			})
			rootSkillCount++
			return nil
		})
		status := ReadinessReady
		if rootSkillCount == 0 {
			status = "empty"
		}
		rootProjections = append(rootProjections, SkillCatalogRootProjection{Ordinal: rootOrdinal, Status: status, SkillCount: rootSkillCount})
	}
	sort.Slice(skills, func(i, j int) bool {
		if skills[i].RootOrdinal != skills[j].RootOrdinal {
			return skills[i].RootOrdinal < skills[j].RootOrdinal
		}
		if skills[i].Name == skills[j].Name {
			return skills[i].InstructionPath < skills[j].InstructionPath
		}
		return skills[i].Name < skills[j].Name
	})
	unique, duplicateCount := excludeDuplicateSkillNames(skills)
	if duplicateCount > 0 {
		exclusions.addN("duplicate_name", duplicateCount)
	}
	return skillCatalogLoadResult{
		Items:      unique,
		Roots:      rootProjections,
		Exclusions: exclusions.projections(),
	}
}

func excludeDuplicateSkillNames(skills []SkillDescriptor) ([]SkillDescriptor, int) {
	counts := make(map[string]int, len(skills))
	for _, skill := range skills {
		counts[skill.Name]++
	}
	unique := make([]SkillDescriptor, 0, len(skills))
	duplicateCount := 0
	for _, skill := range skills {
		if counts[skill.Name] == 1 {
			unique = append(unique, skill)
		} else {
			duplicateCount++
		}
	}
	return unique, duplicateCount
}

func (c skillCatalogExclusionCounter) add(reason string) {
	c.addN(reason, 1)
}

func (c skillCatalogExclusionCounter) addN(reason string, count int) {
	reason = strings.TrimSpace(reason)
	if reason == "" || count <= 0 {
		return
	}
	c[reason] += count
}

func (c skillCatalogExclusionCounter) projections() []SkillCatalogExclusionProjection {
	reasons := make([]string, 0, len(c))
	for reason := range c {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	projections := make([]SkillCatalogExclusionProjection, 0, len(reasons))
	for _, reason := range reasons {
		projections = append(projections, SkillCatalogExclusionProjection{
			Reason: reason,
			Count:  c[reason],
		})
	}
	return projections
}

func skillIndexWarnings(skills []SkillCatalogItemProjection, budget int) []SkillCatalogWarningProjection {
	if len(skills) == 0 {
		return nil
	}
	included := skillIndexNames(skillIndexContext(skills, budget))
	missing := make([]string, 0)
	for _, skill := range skills {
		name := strings.TrimSpace(skill.Name)
		if name == "" || included[name] {
			continue
		}
		missing = append(missing, name)
	}
	if len(missing) == 0 {
		return nil
	}
	if len(missing) > 20 {
		missing = append([]string(nil), missing[:20]...)
	}
	return []SkillCatalogWarningProjection{{
		Reason: "skill_index_budget_excluded",
		Count:  len(skills) - len(included),
		Names:  missing,
	}}
}

func skillIndexNames(text string) map[string]bool {
	names := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		if before, _, ok := strings.Cut(name, ":"); ok {
			name = strings.TrimSpace(before)
		}
		if name != "" {
			names[name] = true
		}
	}
	return names
}

func pathDepth(rel string) int {
	if rel == "." || rel == "" {
		return 0
	}
	return len(strings.Split(filepath.Clean(rel), string(filepath.Separator)))
}

func isSafeSkillMetadata(name string, description string) bool {
	if hasInvisibleControlMarker(name) || hasInvisibleControlMarker(description) {
		return false
	}
	if validateKernelTextNotSecret("skill name", name) != nil ||
		validateKernelTextNotSecret("skill description", description) != nil {
		return false
	}
	risks, err := scanTurnIngressSecurity([]InputItem{
		{Type: "text", Text: name},
		{Type: "text", Text: description},
	})
	return err == nil && len(risks) == 0
}

func parseSkillMetadata(payload string) (string, string, bool) {
	normalized := strings.ReplaceAll(payload, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return "", "", false
	}
	end := strings.Index(normalized[4:], "\n---")
	if end < 0 {
		return "", "", false
	}
	frontMatter := normalized[4 : 4+end]
	var name string
	var description string
	for _, line := range strings.Split(frontMatter, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "name":
			name = cleanYAMLScalar(value)
		case "description":
			description = cleanYAMLScalar(value)
		}
	}
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" || description == "" || hasInvisibleControlMarker(name) || hasInvisibleControlMarker(description) {
		return "", "", false
	}
	return name, description, true
}

func cleanYAMLScalar(value string) string {
	text := strings.TrimSpace(value)
	if len(text) >= 2 {
		if (text[0] == '"' && text[len(text)-1] == '"') || (text[0] == '\'' && text[len(text)-1] == '\'') {
			text = text[1 : len(text)-1]
		}
	}
	return strings.TrimSpace(text)
}
