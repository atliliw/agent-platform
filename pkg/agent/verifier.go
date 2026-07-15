package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Verifier checks whether a task's success criteria have actually been met
// before the engine declares completion. Without a verifier the engine treats
// "the LLM stopped calling tools" as success - which is not the same as the
// task being done correctly. A real agent verifies its own output.
type Verifier interface {
	// Verify returns whether the success criteria are met, plus a short
	// evidence string the agent sees when verification fails (so it can
	// re-plan). An error means the verifier itself failed; the engine treats
	// verifier errors as "inconclusive" and does not block completion.
	Verify(ctx context.Context, execCtx *ExecutionContext) (passed bool, evidence string, err error)
}

// LLMVerifier judges completion against the success criteria using an LLM call
// over the conversation history. It is the default Verifier.
type LLMVerifier struct {
	client LLMClient
	model  string
}

// NewLLMVerifier creates an LLM-backed verifier. model may be empty to use the
// client's default model.
func NewLLMVerifier(client LLMClient, model string) *LLMVerifier {
	return &LLMVerifier{client: client, model: model}
}

// Verify implements Verifier. It asks the LLM to judge the outcome against the
// success criteria. When criteria are empty or the client is nil, it passes
// (backward-compatible with tasks that have no checkable success condition).
func (v *LLMVerifier) Verify(ctx context.Context, execCtx *ExecutionContext) (bool, string, error) {
	if execCtx.SuccessCriteria == "" {
		return true, "", nil
	}
	if v.client == nil {
		return true, "", nil
	}

	// Summarize the conversation for the verifier (cap length to keep it cheap).
	var sb strings.Builder
	const maxChars = 4000
	for _, m := range execCtx.Messages {
		line := fmt.Sprintf("[%s] %s\n", m.Role, m.Content)
		if sb.Len()+len(line) > maxChars {
			sb.WriteString("...(truncated)...\n")
			break
		}
		sb.WriteString(line)
	}

	judgePrompt := fmt.Sprintf(`You are a strict task verifier. Decide whether the SUCCESS CRITERIA are met by the conversation below. Be rigorous: only return true if the criteria are demonstrably satisfied.

SUCCESS CRITERIA:
%s

CONVERSATION:
%s

Reply with JSON only, no prose: {"passed": <true|false>, "evidence": "<one sentence citing what was or was not achieved>"}`,
		execCtx.SuccessCriteria, sb.String())

	resp, err := v.client.Chat(ctx, &LLMRequest{
		Messages: []Message{
			{Role: "system", Content: "You are a strict task verifier. Reply with JSON only."},
			{Role: "user", Content: judgePrompt},
		},
		Model:       v.model,
		MaxTokens:   256,
		Temperature: 0,
	})
	if err != nil {
		// Fail open: a verifier error must not trap the agent in a loop.
		return true, "verification inconclusive (verifier call failed)", nil
	}

	var verdict struct {
		Passed   bool   `json:"passed"`
		Evidence string `json:"evidence"`
	}
	if err := json.Unmarshal([]byte(resp.Content), &verdict); err != nil {
		// LLM did not return clean JSON. Fail open to avoid trapping.
		return true, "verification inconclusive (could not parse verdict)", nil
	}
	if verdict.Evidence == "" {
		verdict.Evidence = "no evidence provided"
	}
	return verdict.Passed, verdict.Evidence, nil
}
