package subagent

import (
	"fmt"

	"skynet/tools"
)

// InspectTurnDefinition is the tool schema for subagent_inspect_turn.
var InspectTurnDefinition = map[string]any{
	"type":        "function",
	"name":        "subagent_inspect_turn",
	"description": "Returns the full content of one turn of a subagent run, identified by its index in the turn list from subagent_inspect.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"hash":  map[string]any{"type": "string", "description": "Hash ID of the subagent run."},
			"index": map[string]any{"type": "integer", "description": "Turn index as returned by subagent_inspect."},
		},
		"required": []string{"hash", "index"},
	},
}

// InspectTurn returns the full content of one turn of a subagent run.
func (m *Manager) InspectTurn(args map[string]any) (any, error) {
	hash, ok := tools.ArgString(args, "hash")
	if !ok || hash == "" {
		return tools.ErrorResult("hash is required"), nil
	}
	f, ok := args["index"].(float64)
	if !ok || f < 0 || f != float64(int(f)) {
		return tools.ErrorResult("index must be a non-negative integer"), nil
	}
	index := int(f)
	turns := m.getRunTurns(hash)
	if turns == nil {
		return tools.ErrorResult("subagent run not found"), nil
	}
	if index >= len(turns) {
		return tools.ErrorResult(fmt.Sprintf("no turn with index %d (the run has %d turns, indices 0-%d)", index, len(turns), len(turns)-1)), nil
	}
	t := turns[index]
	content, truncated := tools.TruncateOutput(t.content)
	result := map[string]any{ //nolint:exhaustruct_v5
		"hash":    hash,
		"index":   index,
		"type":    t.kind,
		"content": content,
	}
	if truncated {
		result["note"] = fmt.Sprintf("content truncated to %d characters", int(tools.MaxTextLimit.Load()))
	}
	return result, nil
}
