package llm

import (
	"context"
	"strings"
	"testing"
)

func TestTruncateWithMarker(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{
			name: "under limit unchanged",
			s:    "hello world",
			max:  100,
			want: "hello world",
		},
		{
			name: "over limit truncated with marker",
			s:    "abcdefghij",
			max:  4,
			want: "abcd\n…[truncated]",
		},
		{
			name: "exact length unchanged",
			s:    "abcd",
			max:  4,
			want: "abcd",
		},
		{
			name: "CJK counted by rune not byte",
			s:    "你好世界测试文本",
			max:  4,
			want: "你好世界\n…[truncated]",
		},
		{
			name: "zero max returns unchanged",
			s:    "hello",
			max:  0,
			want: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateWithMarker(tt.s, tt.max)
			if got != tt.want {
				t.Errorf("truncateWithMarker(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	// Pure ASCII: ~4 chars per token
	asciiTokens := EstimateTokens("hello world!") // 12 chars -> ~3 tokens
	if asciiTokens < 2 || asciiTokens > 4 {
		t.Errorf("ASCII estimate %d out of expected [2,4] range", asciiTokens)
	}

	// Pure CJK: 6 chars * 1.5 = 9 tokens
	cjkTokens := EstimateTokens("你好世界测试") // 6 chars
	if cjkTokens != 9 {
		t.Errorf("CJK estimate = %d, want 9", cjkTokens)
	}

	// Empty
	if EstimateTokens("") != 0 {
		t.Errorf("empty estimate = %d, want 0", EstimateTokens(""))
	}
}

func TestNewCompressionClient_NoOpWhenDisabled(t *testing.T) {
	inner := &OpenAIClient{} // concrete, comparable identity via pointer
	got := NewCompressionClient(inner, CompressionConfig{Enable: false})
	if got != inner {
		t.Errorf("disabled compression should return inner unchanged")
	}
}

func TestNewCompressionClient_NilInner(t *testing.T) {
	if got := NewCompressionClient(nil, CompressionConfig{Enable: true}); got != nil {
		t.Errorf("nil inner should return nil, got %v", got)
	}
}

func TestNewCompressionClient_FillsZeroThresholds(t *testing.T) {
	inner := &OpenAIClient{}
	c, ok := NewCompressionClient(inner, CompressionConfig{Enable: true}).(*compressingClient)
	if !ok {
		t.Fatal("expected *compressingClient when enabled")
	}
	if c.cfg.MaxSystemChars != 12000 {
		t.Errorf("MaxSystemChars = %d, want 12000", c.cfg.MaxSystemChars)
	}
	if c.cfg.MaxRecentChars != 6000 {
		t.Errorf("MaxRecentChars = %d, want 6000", c.cfg.MaxRecentChars)
	}
	if c.cfg.MaxOldChars != 1000 {
		t.Errorf("MaxOldChars = %d, want 1000", c.cfg.MaxOldChars)
	}
	if c.cfg.RecentCount != 8 {
		t.Errorf("RecentCount = %d, want 8", c.cfg.RecentCount)
	}
}

// fakeClient records the request it receives so tests can assert post-compression state.
type fakeClient struct {
	lastReq *ChatRequest
}

func (f *fakeClient) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	f.lastReq = req
	return &ChatResponse{}, nil
}
func (f *fakeClient) ChatStream(ctx context.Context, req *ChatRequest) (<-chan ChatStreamChunk, error) {
	f.lastReq = req
	return nil, nil
}
func (f *fakeClient) Embed(ctx context.Context, text string) ([]float64, error) {
	return nil, nil
}
func (f *fakeClient) EmbedBatch(ctx context.Context, texts []string) ([][]float64, error) {
	return nil, nil
}

func TestCompress_TiersAndNoDeletion(t *testing.T) {
	inner := &fakeClient{}
	c := NewCompressionClient(inner, CompressionConfig{
		Enable:         true,
		MaxSystemChars: 20,
		MaxRecentChars: 15,
		MaxOldChars:    5,
		RecentCount:    2,
	})

	// Build a request: 1 system + 3 user/assistant (so oldest 1 is "old", recent 2).
	long := strings.Repeat("x", 50)
	req := &ChatRequest{
		Messages: []Message{
			{Role: "system", Content: long},                                  // -> capped to 20
			{Role: "user", Content: long},                                    // old -> capped to 5
			{Role: "assistant", Content: long},                               // recent -> capped to 15
			{Role: "user", Content: long},                                    // recent -> capped to 15
		},
	}

	_, _ = c.Chat(context.Background(), req)

	got := inner.lastReq.Messages
	if len(got) != 4 {
		t.Fatalf("expected 4 messages preserved, got %d (compression must not delete)", len(got))
	}

	// system capped to 20 runes + marker
	if !strings.HasSuffix(got[0].Content, "[truncated]") || len([]rune(got[0].Content)) > 40 {
		t.Errorf("system not capped correctly: len=%d", len([]rune(got[0].Content)))
	}
	// old capped to 5
	if !strings.HasSuffix(got[1].Content, "[truncated]") || len([]rune(got[1].Content)) > 30 {
		t.Errorf("old not capped correctly: len=%d", len([]rune(got[1].Content)))
	}
	// recent capped to 15
	if !strings.HasSuffix(got[2].Content, "[truncated]") || len([]rune(got[2].Content)) > 40 {
		t.Errorf("recent[0] not capped correctly: len=%d", len([]rune(got[2].Content)))
	}
	if !strings.HasSuffix(got[3].Content, "[truncated]") || len([]rune(got[3].Content)) > 40 {
		t.Errorf("recent[1] not capped correctly: len=%d", len([]rune(got[3].Content)))
	}
}

func TestCompress_ShortContentUntouched(t *testing.T) {
	inner := &fakeClient{}
	c := NewCompressionClient(inner, CompressionConfig{
		Enable:         true,
		MaxSystemChars: 1000,
		MaxRecentChars: 1000,
		MaxOldChars:    1000,
		RecentCount:    8,
	})

	req := &ChatRequest{
		Messages: []Message{
			{Role: "system", Content: "short system"},
			{Role: "user", Content: "short user"},
		},
	}
	_, _ = c.Chat(context.Background(), req)

	if inner.lastReq.Messages[0].Content != "short system" {
		t.Errorf("short system content should be unchanged, got %q", inner.lastReq.Messages[0].Content)
	}
	if inner.lastReq.Messages[1].Content != "short user" {
		t.Errorf("short user content should be unchanged, got %q", inner.lastReq.Messages[1].Content)
	}
}

func TestCompress_SystemPromptField(t *testing.T) {
	inner := &fakeClient{}
	c := NewCompressionClient(inner, CompressionConfig{
		Enable:         true,
		MaxSystemChars: 10,
		MaxRecentChars: 1000,
		MaxOldChars:    1000,
		RecentCount:    8,
	})

	req := &ChatRequest{
		SystemPrompt: strings.Repeat("y", 50),
		Messages:     []Message{{Role: "user", Content: "hi"}},
	}
	_, _ = c.Chat(context.Background(), req)

	if !strings.HasSuffix(inner.lastReq.SystemPrompt, "[truncated]") {
		t.Errorf("SystemPrompt field should be truncated, got %q", inner.lastReq.SystemPrompt)
	}
}

// Verify tool-call pairing survives: even tool messages get truncated, not dropped.
func TestCompress_ToolMessagesPreserved(t *testing.T) {
	inner := &fakeClient{}
	c := NewCompressionClient(inner, CompressionConfig{
		Enable:         true,
		MaxSystemChars: 1000,
		MaxRecentChars: 10,
		MaxOldChars:    5,
		RecentCount:    1,
	})

	long := strings.Repeat("z", 80)
	req := &ChatRequest{
		Messages: []Message{
			{Role: "assistant", Content: "calling tool"}, // old
			{Role: "tool", Content: long},                // old, logically paired with the call
		},
	}
	// Message struct only carries Role/Content at this layer; tool-call pairing
	// is logical. This test asserts both messages survive compression (none deleted).
	_, _ = c.Chat(context.Background(), req)

	if len(inner.lastReq.Messages) != 2 {
		t.Fatalf("tool pairing broken: expected 2 messages, got %d", len(inner.lastReq.Messages))
	}
}
