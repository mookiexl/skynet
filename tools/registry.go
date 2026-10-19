package tools

// RegisterTool registers a tool with the given name, definition, and execution function.
// This is the preferred registration method - no struct or interface required.
//
// Example usage:
//
//	func init() {
//	    RegisterTool("my_tool", MyToolDefinition, ExecuteMyTool)
//	}
func RegisterTool(name string, definition map[string]any, execute func(map[string]any) (any, error)) {
	Registry[name] = Tool{
		Definition: definition,
		Execute:    execute,
	}
}

// Registry of all tools - populated by init() functions
var Registry = make(map[string]Tool)

// Tool stores both the definition (map) and the execution function.
type Tool struct {
	Definition map[string]any
	Execute    func(map[string]any) (any, error)
}
