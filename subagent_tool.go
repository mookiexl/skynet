package main

import (
	"skynet/subagent"
	"skynet/tools"
)

// subagentMgr implements the subagent tools; the subagent module owns the
// logic, main only wires the host dependencies into it.
var subagentMgr *subagent.Manager

// registerSubagentTools registers the subagent tools with the tool
// registry. Must run after subagentMgr has been created in main().
func registerSubagentTools() {
	tools.RegisterTool("subagent_start", subagent.StartDefinition, subagentMgr.Start)
	tools.RegisterTool("subagent_resume", subagent.ResumeDefinition, subagentMgr.Resume)
	tools.RegisterTool("subagent_reprompt", subagent.RepromptDefinition, subagentMgr.Reprompt)
	tools.RegisterTool("subagent_inspect", subagent.InspectDefinition, subagentMgr.Inspect)
	tools.RegisterTool("subagent_inspect_turn", subagent.InspectTurnDefinition, subagentMgr.InspectTurn)
}
