package kernel

type SkillDescriptor struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	InstructionPath string `json:"-"`
}

type SkillCatalogProjection struct {
	Status     string                            `json:"status"`
	Count      int                               `json:"count"`
	Items      []SkillCatalogItemProjection      `json:"items"`
	Exclusions []SkillCatalogExclusionProjection `json:"exclusions"`
}

type SkillCatalogItemProjection struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type SkillCatalogExclusionProjection struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}
