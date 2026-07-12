package llm

import (
	"context"
	"log"
	"math"
)

// CompressionConfig holds runtime configuration for the compressing client.
// This is the runtime view; YAML-loaded defaults live in pkg/config and are
// mapped into this struct at service construction time.
type CompressionConfig struct {
	// Enable gates compression. When false, NewCompressionClient is a no-op.
	Enable bool
	// MaxSystemChars caps each system-role message (and the SystemPrompt field).
	// System prompts carry instructions and get the largest budget.
	MaxSystemChars int
	// MaxRecentChars caps each of the most recent RecentCount non-system messages.
	MaxRecentChars int
	// MaxOldChars caps each older non-system message. Old context is the prime
	// target for trimming since it is least likely to affect the next decision.
	MaxOldChars int
	// RecentCount is how many trailing messages are treated as "recent".
	RecentCount int
}

// DefaultCompressionConfig returns a sane enabled configuration. Thresholds are
// char-based (runes), tuned so normal short conversations stay untouched while
// bloated tool outputs and long histories get trimmed.
//
//   - MaxSystemChars 12000  (~6-8K tokens): system prompt rarely needs more
//   - MaxRecentChars 6000   (~3-4K tokens): recent turns kept fairly whole
//   - MaxOldChars   1000    (~500-700 tokens): older turns reduced to gist
//   - RecentCount   8       : keeps the active working set verbatim-ish
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		Enable:         true,
		MaxSystemChars: 12000,
		MaxRecentChars: 6000,
		MaxOldChars:    1000,
		RecentCount:    8,
	}
}

// compressingClient wraps a Client and truncates oversized prompt content
// before delegating to the inner client. It mirrors the metricsClient decorator
// pattern. Compression is lossless-in-spirit: no messages are deleted, so
// tool_call / tool_response pairing is preserved. Only message *content* is
// truncated when it exceeds the configured per-tier budget.
type compressingClient struct {
	inner Client
	cfg   CompressionConfig
}

// NewCompressionClient wraps inner with a compression layer.
// If inner is nil, nil is returned. If cfg.Enable is false, inner is returned
// unchanged (zero overhead). Any zero threshold is filled from defaults so a
// partially-specified config still behaves correctly.
func NewCompressionClient(inner Client, cfg CompressionConfig) Client {
	if inner == nil {
		return nil
	}
	if !cfg.Enable {
		return inner
	}
	if cfg.MaxSystemChars <= 0 {
		cfg.MaxSystemChars = 12000
	}
	if cfg.MaxRecentChars <= 0 {
		cfg.MaxRecentChars = 6000
	}
	if cfg.MaxOldChars <= 0 {
		cfg.MaxOldChars = 1000
	}
	if cfg.RecentCount <= 0 {
		cfg.RecentCount = 8
	}
	return &compressingClient{inner: inner, cfg: cfg}
}

func (c *compressingClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	c.compress(req)
	return c.inner.Chat(ctx, req)
}

func (c *compressingClient) ChatStream(ctx context.Context, req *ChatRequest) (<-chan ChatStreamChunk, error) {
	c.compress(req)
	return c.inner.ChatStream(ctx, req)
}

// Embed / EmbedBatch are pass-through: embeddings are not chat context.
func (c *compressingClient) Embed(ctx context.Context, text string) ([]float64, error) {
	return c.inner.Embed(ctx, text)
}

func (c *compressingClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	return c.inner.EmbedBatch(ctx, texts)
}

// compress mutates req in place, truncating oversized content per tier.
// It is safe to mutate: callers (e.g. llmAdapter) build a fresh ChatRequest
// per call, so there is no shared state to corrupt.
func (c *compressingClient) compress(req *ChatRequest) {
	if req == nil {
		return
	}

	before := 0
	changed := false

	// Cap the SystemPrompt field (used by direct llm.Client callers; the engine
	// path puts system content in Messages[0] instead).
	if req.SystemPrompt != "" {
		before += EstimateTokens(req.SystemPrompt)
		truncated := truncateWithMarker(req.SystemPrompt, c.cfg.MaxSystemChars)
		if truncated != req.SystemPrompt {
			changed = true
		}
		req.SystemPrompt = truncated
	}

	msgs := req.Messages
	if len(msgs) == 0 {
		return
	}

	// "Recent" = trailing RecentCount messages. System messages always get the
	// system budget regardless of position, so they are checked first.
	recentStart := len(msgs) - c.cfg.RecentCount
	if recentStart < 0 {
		recentStart = 0
	}

	for i := range msgs {
		if msgs[i].Content == "" {
			continue
		}
		before += EstimateTokens(msgs[i].Content)

		var max int
		switch {
		case msgs[i].Role == "system":
			max = c.cfg.MaxSystemChars
		case i >= recentStart:
			max = c.cfg.MaxRecentChars
		default:
			max = c.cfg.MaxOldChars
		}

		truncated := truncateWithMarker(msgs[i].Content, max)
		if truncated != msgs[i].Content {
			changed = true
		}
		msgs[i].Content = truncated
	}

	if changed {
		after := estimateRequestTokens(req)
		log.Printf("[compression] prompt truncated: ~%d -> ~%d tokens (saved ~%d)",
			before, after, before-after)
	}
}

// estimateRequestTokens sums a rough token estimate over SystemPrompt + Messages.
func estimateRequestTokens(req *ChatRequest) int {
	total := 0
	if req.SystemPrompt != "" {
		total += EstimateTokens(req.SystemPrompt)
	}
	for _, m := range req.Messages {
		total += EstimateTokens(m.Content)
	}
	return total
}

// truncateWithMarker truncates s to at most max runes. If s fits, it is
// returned unchanged. Otherwise the first max runes are kept and a visible
// truncation marker is appended so downstream consumers (LLM included) can
// tell the content was cut. Rune-based counting keeps CJK text fair: one
// Chinese character counts as one unit, not three bytes.
func truncateWithMarker(s string, max int) string {
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "\n…[truncated]"
}

// EstimateTokens returns a rough token estimate for s, CJK-aware.
// Heuristic: CJK characters ~1.5 tokens each, other characters ~0.25 tokens
// each (≈4 chars per token, the common ASCII rule of thumb). This is only for
// observability/logging, not for billing or hard budget enforcement.
func EstimateTokens(s string) int {
	cjk, other := 0, 0
	for _, r := range s {
		if isCJK(r) {
			cjk++
		} else {
			other++
		}
	}
	return int(math.Ceil(float64(cjk)*1.5 + float64(other)*0.25))
}

// isCJK reports whether r is in common CJK/Hangul/Kana ranges.
func isCJK(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF: // CJK Unified Ideographs
		return true
	case r >= 0x3400 && r <= 0x4DBF: // CJK Extension A
		return true
	case r >= 0x3040 && r <= 0x30FF: // Hiragana + Katakana
		return true
	case r >= 0xAC00 && r <= 0xD7AF: // Hangul Syllables
		return true
	case r >= 0xFF00 && r <= 0xFFEF: // Fullwidth Forms
		return true
	}
	return false
}
