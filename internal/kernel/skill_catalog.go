package kernel

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func loadSkillCatalog(roots []string) []SkillDescriptor {
	var skills []SkillDescriptor
	for _, root := range roots {
		cleanRoot := strings.TrimSpace(root)
		if cleanRoot == "" {
			continue
		}
		absRoot, err := filepath.Abs(expandHome(cleanRoot))
		if err != nil {
			continue
		}
		info, err := os.Stat(absRoot)
		if err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(absRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || entry.Name() != "SKILL.md" {
				return nil
			}
			payload, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			name, description, ok := parseSkillMetadata(string(payload))
			if !ok || !isSafeSkillMetadata(name, description) {
				return nil
			}
			instructionPath, err := filepath.Abs(path)
			if err != nil {
				return nil
			}
			skills = append(skills, SkillDescriptor{
				Name:            name,
				Description:     redactEvidenceText(description),
				InstructionPath: filepath.Clean(instructionPath),
			})
			return nil
		})
	}
	sort.Slice(skills, func(i, j int) bool {
		if skills[i].Name == skills[j].Name {
			return skills[i].InstructionPath < skills[j].InstructionPath
		}
		return skills[i].Name < skills[j].Name
	})
	return skills
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
