package agent

import "testing"

func TestIsFineGrainedBrowserTool(t *testing.T) {
	for _, name := range []string{
		"browser_navigate", "browser_click", "browser_type",
		"browser_extract", "browser_scroll", "browser_wait",
	} {
		if !isFineGrainedBrowserTool(name) {
			t.Errorf("isFineGrainedBrowserTool(%q) = false, want true", name)
		}
	}
	// browser_execute is the autonomous mode and must NOT be treated as
	// session-aware; it manages its own browser lifecycle.
	for _, name := range []string{"browser_execute", "web_search", "load_skill", "calculator", ""} {
		if isFineGrainedBrowserTool(name) {
			t.Errorf("isFineGrainedBrowserTool(%q) = true, want false", name)
		}
	}
}

func TestInjectSessionID_NilToolCfg(t *testing.T) {
	got := injectSessionID(nil, "sess-1")
	if got == nil {
		t.Fatal("expected non-nil config for nil input + non-empty session")
	}
	if got.Extra["session_id"] != "sess-1" {
		t.Errorf("session_id = %v, want sess-1", got.Extra["session_id"])
	}
	if len(got.Extra) != 1 {
		t.Errorf("expected exactly one extra key, got %d", len(got.Extra))
	}
}

func TestInjectSessionID_PreservesExistingFields(t *testing.T) {
	orig := &ToolSpecificConfig{
		APIKey:  "k",
		BaseURL: "http://x",
		Model:   "m",
		Extra:   map[string]any{"foo": "bar", "count": 3},
	}
	got := injectSessionID(orig, "sess-1")

	if got.APIKey != "k" || got.BaseURL != "http://x" || got.Model != "m" {
		t.Error("scalar fields not preserved in clone")
	}
	if got.Extra["session_id"] != "sess-1" {
		t.Errorf("session_id = %v, want sess-1", got.Extra["session_id"])
	}
	if got.Extra["foo"] != "bar" || got.Extra["count"] != 3 {
		t.Error("existing extra entries not preserved")
	}
	if len(got.Extra) != 3 {
		t.Errorf("expected 3 extra keys, got %d", len(got.Extra))
	}
}

func TestInjectSessionID_DoesNotMutateOriginal(t *testing.T) {
	orig := &ToolSpecificConfig{Extra: map[string]any{"foo": "bar"}}
	clone := injectSessionID(orig, "sess-1")

	// The original config and its Extra map must be untouched.
	if _, has := orig.Extra["session_id"]; has {
		t.Error("original Extra was mutated with session_id")
	}
	if len(orig.Extra) != 1 {
		t.Errorf("original Extra size = %d, want 1", len(orig.Extra))
	}
	// Mutating the clone's Extra must not bleed into the original.
	clone.Extra["leaked"] = true
	if _, has := orig.Extra["leaked"]; has {
		t.Error("clone Extra shares storage with original Extra")
	}
}

func TestInjectSessionID_EmptySessionIDIsNoOp(t *testing.T) {
	orig := &ToolSpecificConfig{APIKey: "k"}
	if got := injectSessionID(orig, ""); got != orig {
		t.Error("non-nil config + empty session should return the same pointer unchanged")
	}
	if got := injectSessionID(nil, ""); got != nil {
		t.Error("nil config + empty session should return nil")
	}
}
