package main

// defaultSubagentDisabledTools lists the tools that are NOT enabled for
// subagents by default. If a tool is not explicitly set in .skynet.yml,
// subagent-enabled defaults to false for these tools and true for all
// others. The MAIN (client) state always defaults to enabled for every
// tool.
//
// Why these are disabled by default for subagents:
//   - the subagent tools (start/resume/reprompt/inspect) operate on the
//     main agent's run state and have no useful role from inside a
//     subagent.
var defaultSubagentDisabledTools = map[string]bool{
	"subagent_start":        true,
	"subagent_resume":       true,
	"subagent_reprompt":     true,
	"subagent_inspect":      true,
	"subagent_inspect_turn": true,
}

// isDefaultDisabledForSubagent reports whether a tool is disabled by
// default for subagents when not explicitly specified in .skynet.yml.
func isDefaultDisabledForSubagent(name string) bool {
	return defaultSubagentDisabledTools[name]
}
