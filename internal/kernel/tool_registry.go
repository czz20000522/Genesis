package kernel

import "encoding/json"

const (
	ToolKindRead   = "read"
	ToolKindEffect = "effect"
)

type kernelToolDefinition struct {
	Descriptor ModelToolDescriptor
	Kind       string
	Prepare    func(*Kernel, string, string, json.RawMessage) (preparedModelToolCall, error)
}

func kernelToolDefinitions() []kernelToolDefinition {
	return []kernelToolDefinition{
		{
			Descriptor: ModelToolDescriptor{
				Name:        "shell.exec",
				Description: "Execute a small governed shell command. Permission mode and workspace root are controlled by the Genesis kernel.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"command": map[string]interface{}{
							"type":        "string",
							"description": "Command to execute through the governed shell tool.",
						},
						"cwd": map[string]interface{}{
							"type":        "string",
							"description": "Optional working directory. When omitted, the kernel uses the configured workspace root when available.",
						},
					},
					"required":             []string{"command"},
					"additionalProperties": false,
				},
			},
			Kind:    ToolKindEffect,
			Prepare: (*Kernel).prepareShellExecToolCall,
		},
		{
			Descriptor: ModelToolDescriptor{
				Name:        "skill.read",
				Description: "Read the bounded instructions for a configured user-space skill by skill name. This does not grant authority or bypass kernel tool permissions.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]interface{}{
							"type":        "string",
							"description": "Configured skill name from the available external skills catalog.",
						},
					},
					"required":             []string{"name"},
					"additionalProperties": false,
				},
			},
			Kind:    ToolKindRead,
			Prepare: (*Kernel).prepareSkillReadToolCall,
		},
	}
}

func lookupKernelTool(name string) (kernelToolDefinition, bool) {
	for _, definition := range kernelToolDefinitions() {
		if definition.Descriptor.Name == name {
			return definition, true
		}
	}
	return kernelToolDefinition{}, false
}

func (k *Kernel) modelToolDescriptors() []ModelToolDescriptor {
	definitions := kernelToolDefinitions()
	descriptors := make([]ModelToolDescriptor, 0, len(definitions))
	for _, definition := range definitions {
		descriptors = append(descriptors, definition.Descriptor)
	}
	return descriptors
}

func toolCapabilityKind(name string) string {
	definition, ok := lookupKernelTool(name)
	if !ok {
		return "unknown"
	}
	return definition.Kind
}
