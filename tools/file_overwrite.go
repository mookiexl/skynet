package tools

func init() {
	RegisterTool("file_overwrite", OverwriteFileDefinition, OverwriteFile)
}

// OverwriteFile replaces the content of an existing file.
func OverwriteFile(args map[string]any) (any, error) {
	path, ok := ArgString(args, "path")
	content, okC := ArgString(args, "content")
	if !ok || !okC {
		return ErrorResult("Path and content are required"), nil
	}

	absPath, err := ValidatePath(path)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	relPath, err := relToWorkDir(absPath)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}

	hash, reason := CreateRollbackEntry(absPath, relPath)

	if err := writeFile(absPath, []byte(content)); err != nil {
		return ErrorResult(err.Error()), nil
	}

	resp := map[string]any{
		"success": true,
	}
	switch reason {
	case RollbackCreated:
		resp["rollback"] = hash
	case RollbackUnavailable:
		resp["rollback"] = RollbackUnavailable
	}
	return resp, nil
}

// OverwriteFileDefinition defines the tool schema.
var OverwriteFileDefinition = map[string]any{
	"description": "Overwrite an existing file with new content. Creates the file if it doesn't exist.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path":    map[string]any{"type": "string", "description": "File path relative to current directory"},
			"content": map[string]any{"type": "string", "description": "The text content to write"},
		},
		"required": []string{"path", "content"},
	},
}
