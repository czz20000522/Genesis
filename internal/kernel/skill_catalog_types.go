package kernel

type SkillDescriptor struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	InstructionPath string `json:"-"`
	RootOrdinal     int    `json:"-"`
}

type SkillCatalogProjection struct {
	Status     string                            `json:"status"`
	Count      int                               `json:"count"`
	Items      []SkillCatalogItemProjection      `json:"items"`
	Roots      []SkillCatalogRootProjection      `json:"roots"`
	Exclusions []SkillCatalogExclusionProjection `json:"exclusions"`
	Warnings   []SkillCatalogWarningProjection   `json:"warnings"`
}

type SkillCatalogItemProjection struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type SkillCatalogExclusionProjection struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

type SkillCatalogRootProjection struct {
	Ordinal    int    `json:"ordinal"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	SkillCount int    `json:"skill_count"`
}

type SkillCatalogWarningProjection struct {
	Reason string   `json:"reason"`
	Count  int      `json:"count"`
	Names  []string `json:"names,omitempty"`
}
