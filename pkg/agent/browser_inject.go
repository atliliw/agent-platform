package agent

// fineGrainedBrowserTools are the six MCP-backed browser primitives that share
// a single browser page across calls within one session, so a navigate ->
// click -> extract sequence reuses the same Chrome tab and preserves cookies /
// login state. browser_execute is deliberately excluded: it is the autonomous
// mode that drives its own multi-step loop and manages its own browser
// lifecycle internally.
var fineGrainedBrowserTools = map[string]bool{
	"browser_navigate": true,
	"browser_click":    true,
	"browser_type":     true,
	"browser_extract":  true,
	"browser_scroll":   true,
	"browser_wait":     true,
}

// isFineGrainedBrowserTool reports whether name is one of the six session-aware
// browser primitives that need the current session ID injected into their tool
// config.
func isFineGrainedBrowserTool(name string) bool {
	return fineGrainedBrowserTools[name]
}

// injectSessionID returns a copy of toolCfg with sessionID merged into Extra so
// the MCP-side executor can bind the call to a shared browser session.
//
// It never mutates toolCfg or the agent's underlying ToolConfig map: the Extra
// map is rebuilt on a fresh copy. An empty sessionID is a no-op (toolCfg is
// returned unchanged) so callers without a session fall back to one-shot
// browsers at the MCP layer rather than poisoning the config.
func injectSessionID(toolCfg *ToolSpecificConfig, sessionID string) *ToolSpecificConfig {
	if sessionID == "" {
		return toolCfg
	}
	if toolCfg == nil {
		return &ToolSpecificConfig{Extra: map[string]any{"session_id": sessionID}}
	}
	clone := *toolCfg // struct copy: scalar fields + map pointer duplicated
	extra := make(map[string]any, len(toolCfg.Extra)+1)
	for k, v := range toolCfg.Extra {
		extra[k] = v
	}
	extra["session_id"] = sessionID
	clone.Extra = extra
	return &clone
}
