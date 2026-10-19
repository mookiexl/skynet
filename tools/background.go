package tools

func init() {
	RegisterTool("background_check", BackgroundCheckDefinition, ExecuteBackgroundCheck)
	RegisterTool("background_terminate", BackgroundTerminateDefinition, ExecuteBackgroundTerminate)
}

// BackgroundCheckDefinition defines the tool schema.
var BackgroundCheckDefinition = map[string]any{
	"description": "Check the status and captured output of a background process",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":        "string",
				"description": "The background process id",
			},
		},
		"required": []string{"id"},
	},
}

// BackgroundTerminateDefinition defines the tool schema.
var BackgroundTerminateDefinition = map[string]any{
	"description": "Terminate a background process and its child processes",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{
				"type":        "string",
				"description": "The background process id",
			},
		},
		"required": []string{"id"},
	},
}
