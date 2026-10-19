package subagent

import (
	"fmt"
	"strings"

	"skynet/tools"
)

// InspectDefinition is the tool schema for subagent_inspect.
var InspectDefinition = map[string]any{
	"type":        "function",
	"name":        "subagent_inspect",
	"description": "Lists the turns of a subagent run so far (think / content / tool / tool return) with short previews. Read-only observability for debugging.",
	"input_schema": map[string]any{
		"type": "object",
		"properties": map[string]any{
			"hash": map[string]any{"type": "string", "description": "Hash ID of the subagent run."},
		},
		"required": []string{"hash"},
	},
}

// Inspect lists the turns of a subagent run so far, with short previews.
func (m *Manager) Inspect(args map[string]any) (any, error) {
	hash, ok := tools.ArgString(args, "hash")
	if !ok || hash == "" {
		return tools.ErrorResult("hash is required"), nil
	}
	turns := m.getRunTurns(hash)
	if turns == nil {
		return tools.ErrorResult("subagent run not found"), nil
	}
	if len(turns) == 0 {
		return tools.ErrorResult("subagent run has no turns yet"), nil
	}
	start := max(len(turns)-inspectTurnListCap, 0)
	var sb strings.Builder
	for i := start; i < len(turns); i++ {
		t := turns[i]
		fmt.Fprintf(&sb, "%d: %s\n%s", t.index, t.kind, inspectPreview(t.content))
		if i+1 < len(turns) {
			sb.WriteString("\n\n")
		}
	}
	result := map[string]any{ //nolint:exhaustruct_v5
		"hash":        hash,
		"total_turns": len(turns),
		"turns":       sb.String(),
	}
	if start > 0 {
		result["note"] = fmt.Sprintf("showing the most recent %d of %d turns; earlier turns are available via subagent_inspect_turn", len(turns)-start, len(turns))
	}
	return result, nil
}
