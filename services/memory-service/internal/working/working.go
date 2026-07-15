// Package working provides working memory functionality
// Working memory manages current conversation context with intelligent compression
package working

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MessageType defines the type of message in working memory
type MessageType string

const (
	MessageTypeUser      MessageType = "user"
	MessageTypeAgent     MessageType = "agent"
	MessageTypeSystem    MessageType = "system"
	MessageTypeTool      MessageType = "tool"
	MessageTypeToolResult MessageType = "tool_result"
)

// Message represents a message in working memory
type Message struct {
	ID        string                 `json:"id"`
	Type      MessageType            `json:"type"`
	Content   string                 `json:"content"`
	Role      string                 `json:"role"`       // For LLM: user, assistant, system
	Summary   string                 `json:"summary"`    // Compressed summary
	Tokens    int                    `json:"tokens"`     // Estimated token count
	Importance float64               `json:"importance"` // 0-1 importance score
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata"`
	IsKey     bool                   `json:"is_key"`     // Key message to preserve
}

// WorkingMemoryContext represents the current working memory context
type WorkingMemoryContext struct {
	SessionID     string    `json:"session_id"`
	Messages      []Message `json:"messages"`
	TotalTokens   int       `json:"total_tokens"`
	MaxTokens     int       `json:"max_tokens"`      // Maximum tokens allowed
	KeyMessages   []string  `json:"key_messages"`    // IDs of key messages to preserve
	LastCompressed time.Time `json:"last_compressed"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CompressionStrategy defines how to compress memory
type CompressionStrategy string

const (
	CompressionStrategyNone      CompressionStrategy = "none"
	CompressionStrategySummary   CompressionStrategy = "summary"    // Summarize old messages
	CompressionStrategySelective CompressionStrategy = "selective" // Keep important messages
	CompressionStrategyHybrid    CompressionStrategy = "hybrid"    // Combine strategies
)

// WorkingMemory manages current conversation context
type WorkingMemory struct {
contexts     map[string]*WorkingMemoryContext
	llmClient   LLMCompressor    // Interface for LLM-based compression
	strategy    CompressionStrategy
	maxTokens   int              // Default max tokens
	maxMessages int              // Max messages per context
	mu          sync.RWMutex
}

// LLMCompressor interface for LLM-based compression
type LLMCompressor interface {
	Compress(ctx context.Context, messages []Message) (string, error)
	ScoreImportance(ctx context.Context, message Message) (float64, error)
}

// NewWorkingMemory creates a new working memory
func NewWorkingMemory(maxTokens, maxMessages int) *WorkingMemory {
	if maxTokens <= 0 {
		maxTokens = 8000
	}
	if maxMessages <= 0 {
		maxMessages = 100
	}
	return &WorkingMemory{
	contexts:     make(map[string]*WorkingMemoryContext),
		strategy:    CompressionStrategyHybrid,
		maxTokens:   maxTokens,
		maxMessages: maxMessages,
	}
}

// SetLLMCompressor sets the LLM compressor
func (w *WorkingMemory) SetLLMCompressor(compressor LLMCompressor) {
	w.llmClient = compressor
}

// SetCompressionStrategy sets the compression strategy
func (w *WorkingMemory) SetCompressionStrategy(strategy CompressionStrategy) {
	w.strategy = strategy
}

// Add adds a message to working memory
func (w *WorkingMemory) Add(ctx context.Context, sessionID string, message Message) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Get or create context
	context, ok := w.contexts[sessionID]
	if !ok {
		context = &WorkingMemoryContext{
			SessionID:   sessionID,
			Messages:    make([]Message, 0),
			MaxTokens:   w.maxTokens,
			KeyMessages: make([]string, 0),
			CreatedAt:   time.Now(),
		}
		w.contexts[sessionID] = context
	}

	// Set message metadata
	if message.ID == "" {
		message.ID = generateMessageID()
	}
	message.Timestamp = time.Now()
	message.Tokens = estimateTokens(message.Content)

	// Add message
	context.Messages = append(context.Messages, message)
	context.TotalTokens += message.Tokens
	context.UpdatedAt = time.Now()

	// Mark as key if important
	if message.IsKey {
		context.KeyMessages = append(context.KeyMessages, message.ID)
	}

	// Check if compression needed
	if context.TotalTokens > context.MaxTokens || len(context.Messages) > w.maxMessages {
		w.compress(context)
	}

	return nil
}

// Get retrieves the current context
func (w *WorkingMemory) Get(ctx context.Context, sessionID string) (*WorkingMemoryContext, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	context, ok := w.contexts[sessionID]
	if !ok {
		return nil, fmt.Errorf("context not found: %s", sessionID)
	}

	return context, nil
}

// GetMessages retrieves messages for LLM context
func (w *WorkingMemory) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	context, ok := w.contexts[sessionID]
	if !ok {
		return nil, fmt.Errorf("context not found: %s", sessionID)
	}

	return context.Messages, nil
}

// GetMessagesForLLM formats messages for LLM API
func (w *WorkingMemory) GetMessagesForLLM(ctx context.Context, sessionID string) []LLMMessage {
	w.mu.RLock()
	defer w.mu.RUnlock()

	context, ok := w.contexts[sessionID]
	if !ok {
		return []LLMMessage{}
	}

	var llmMessages []LLMMessage
	for _, msg := range context.Messages {
		role := msg.Role
		if role == "" {
			switch msg.Type {
			case MessageTypeUser:
				role = "user"
			case MessageTypeAgent:
				role = "assistant"
			case MessageTypeSystem:
				role = "system"
			default:
				role = "user"
			}
		}

		content := msg.Content
		if msg.Summary != "" && msg.Tokens > 200 {
			content = msg.Summary
		}

		llmMessages = append(llmMessages, LLMMessage{
			Role:    role,
			Content: content,
		})
	}

	return llmMessages
}

// Clear clears a session's context
func (w *WorkingMemory) Clear(ctx context.Context, sessionID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	delete(w.contexts, sessionID)
	return nil
}

// MarkKey marks a message as key (to be preserved during compression)
func (w *WorkingMemory) MarkKey(ctx context.Context, sessionID, messageID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	context, ok := w.contexts[sessionID]
	if !ok {
		return fmt.Errorf("context not found: %s", sessionID)
	}

	// Update message
	for i := range context.Messages {
		if context.Messages[i].ID == messageID {
			context.Messages[i].IsKey = true
			break
		}
	}

	// Add to key messages list
	context.KeyMessages = append(context.KeyMessages, messageID)

	return nil
}

// Update updates a message's content
func (w *WorkingMemory) Update(ctx context.Context, sessionID, messageID string, content string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	context, ok := w.contexts[sessionID]
	if !ok {
		return fmt.Errorf("context not found: %s", sessionID)
	}

	for i := range context.Messages {
		if context.Messages[i].ID == messageID {
			oldTokens := context.Messages[i].Tokens
			context.Messages[i].Content = content
			context.Messages[i].Tokens = estimateTokens(content)
			context.Messages[i].Summary = "" // Clear summary
			context.TotalTokens = context.TotalTokens - oldTokens + context.Messages[i].Tokens
			context.Messages[i].Timestamp = time.Now()
			break
		}
	}

	context.UpdatedAt = time.Now()

	return nil
}

// GetTokenUsage returns current token usage
func (w *WorkingMemory) GetTokenUsage(ctx context.Context, sessionID string) (int, int, float64, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	context, ok := w.contexts[sessionID]
	if !ok {
		return 0, 0, 0, fmt.Errorf("context not found: %s", sessionID)
	}

	usagePercent := float64(context.TotalTokens) / float64(context.MaxTokens) * 100

	return context.TotalTokens, context.MaxTokens, usagePercent, nil
}

// compress compresses the context based on strategy
func (w *WorkingMemory) compress(wmCtx *WorkingMemoryContext) {
	switch w.strategy {
	case CompressionStrategyNone:
		return

	case CompressionStrategySummary:
		w.compressBySummary(wmCtx)

	case CompressionStrategySelective:
		w.compressBySelective(wmCtx)

	case CompressionStrategyHybrid:
		w.compressByHybrid(wmCtx)
	}

	wmCtx.LastCompressed = time.Now()
}

// compressBySummary compresses by summarizing old messages
func (w *WorkingMemory) compressBySummary(wmCtx *WorkingMemoryContext) {
	// Keep last few messages uncompressed
	keepCount := 5
	if len(wmCtx.Messages) <= keepCount {
		return
	}

	// Get messages to compress
	toCompress := wmCtx.Messages[:len(wmCtx.Messages)-keepCount]

	// Create summary
	if w.llmClient != nil {
		ctx := context.Background()
		summary, err := w.llmClient.Compress(ctx, toCompress)
		if err == nil && summary != "" {
			// Replace compressed messages with summary
			summaryMsg := Message{
				ID:        generateMessageID(),
				Type:      MessageTypeSystem,
				Content:   summary,
				Summary:   summary,
				Tokens:    estimateTokens(summary),
				Importance: 0.8,
				Timestamp: time.Now(),
			}

			// Recalculate total tokens
			newTokens := summaryMsg.Tokens
			for _, m := range wmCtx.Messages[len(wmCtx.Messages)-keepCount:] {
				newTokens += m.Tokens
			}

			wmCtx.Messages = append([]Message{summaryMsg}, wmCtx.Messages[len(wmCtx.Messages)-keepCount:]...)
			wmCtx.TotalTokens = newTokens
		}
	} else {
		// Fallback: simple truncation
		w.simpleCompress(wmCtx)
	}
}

// compressBySelective compresses by removing low-importance messages
func (w *WorkingMemory) compressBySelective(wmCtx *WorkingMemoryContext) {
	// Sort messages by importance (keeping key messages)
	keySet := make(map[string]bool)
	for _, id := range wmCtx.KeyMessages {
		keySet[id] = true
	}

	// Keep key messages and high-importance messages
	var kept []Message
	var removed []Message
	var newTokens int

	for _, msg := range wmCtx.Messages {
		if msg.IsKey || keySet[msg.ID] || msg.Importance >= 0.7 {
			kept = append(kept, msg)
			newTokens += msg.Tokens
		} else {
			removed = append(removed, msg)
		}
	}

	// If still over limit, remove oldest low-importance messages
	for newTokens > wmCtx.MaxTokens && len(kept) > 5 {
		// Find lowest importance message (not key)
		lowestIdx := -1
		lowestImportance := 1.0
		for i, msg := range kept {
			if !msg.IsKey && !keySet[msg.ID] && msg.Importance < lowestImportance {
				lowestImportance = msg.Importance
				lowestIdx = i
			}
		}

		if lowestIdx >= 0 {
			newTokens -= kept[lowestIdx].Tokens
			kept = append(kept[:lowestIdx], kept[lowestIdx+1:]...)
		} else {
			break
		}
	}

	wmCtx.Messages = kept
	wmCtx.TotalTokens = newTokens
}

// compressByHybrid combines summary and selective strategies
func (w *WorkingMemory) compressByHybrid(wmCtx *WorkingMemoryContext) {
	// First, do selective compression to keep important messages
	keySet := make(map[string]bool)
	for _, id := range wmCtx.KeyMessages {
		keySet[id] = true
	}

	// Find messages that can be summarized (not key, older)
	var toSummarize []Message
	var toKeep []Message

	keepRecent := 10
	for i, msg := range wmCtx.Messages {
		if i >= len(wmCtx.Messages)-keepRecent || msg.IsKey || keySet[msg.ID] || msg.Importance >= 0.6 {
			toKeep = append(toKeep, msg)
		} else {
			toSummarize = append(toSummarize, msg)
		}
	}

	// Summarize old messages
	if len(toSummarize) > 3 && w.llmClient != nil {
		ctx := context.Background()
		summary, err := w.llmClient.Compress(ctx, toSummarize)
		if err == nil && summary != "" {
			summaryMsg := Message{
				ID:        generateMessageID(),
				Type:      MessageTypeSystem,
				Content:   summary,
				Summary:   summary,
				Tokens:    estimateTokens(summary),
				Importance: 0.7,
				Timestamp: time.Now(),
			}
			toKeep = append([]Message{summaryMsg}, toKeep...)
		}
	}

	// Calculate new tokens
	var newTokens int
	for _, m := range toKeep {
		newTokens += m.Tokens
	}

	wmCtx.Messages = toKeep
	wmCtx.TotalTokens = newTokens
}

// simpleCompress does simple truncation
func (w *WorkingMemory) simpleCompress(wmCtx *WorkingMemoryContext) {
	// Keep last 20 messages
	if len(wmCtx.Messages) > 20 {
		wmCtx.Messages = wmCtx.Messages[len(wmCtx.Messages)-20:]
	}

	// Recalculate tokens
	var newTokens int
	for _, m := range wmCtx.Messages {
		newTokens += m.Tokens
	}
	wmCtx.TotalTokens = newTokens
}

// GetStats returns working memory statistics
func (w *WorkingMemory) GetStats() map[string]interface{} {
	w.mu.RLock()
	defer w.mu.RUnlock()

	stats := map[string]interface{}{
		"active_contexts": len(w.contexts),
		"strategy":        w.strategy,
		"max_tokens":      w.maxTokens,
		"max_messages":    w.maxMessages,
		"contexts":        make(map[string]interface{}),
	}

	for sessionID, ctx := range w.contexts {
		stats["contexts"].(map[string]interface{})[sessionID] = map[string]interface{}{
			"message_count":  len(ctx.Messages),
			"total_tokens":   ctx.TotalTokens,
			"usage_percent":  float64(ctx.TotalTokens) / float64(ctx.MaxTokens) * 100,
			"key_messages":   len(ctx.KeyMessages),
			"last_compressed": ctx.LastCompressed,
		}
	}

	return stats
}

// LLMMessage represents a message for LLM API
type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Helper functions
func generateMessageID() string {
	return fmt.Sprintf("msg-%d", time.Now().UnixNano())
}

// estimateTokens estimates token count for content
func estimateTokens(content string) int {
	// Rough estimation: ~4 characters per token for English, ~1.5 for Chinese
	// This is a simple heuristic
	englishChars := 0
	chineseChars := 0

	for _, char := range content {
		if char >= 0x4E00 && char <= 0x9FFF {
			chineseChars++
		} else {
			englishChars++
		}
	}

	englishTokens := englishChars / 4
	if englishChars % 4 > 0 {
		englishTokens++
	}

	chineseTokens := chineseChars / 2
	if chineseChars % 2 > 0 {
		chineseTokens++
	}

	return englishTokens + chineseTokens + 1
}

// while loop replaced with proper Go loop
func (w *WorkingMemory) compressBySelectiveLoop(wmCtx *WorkingMemoryContext, kept []Message, newTokens int) []Message {
	for newTokens > wmCtx.MaxTokens && len(kept) > 5 {
		// Find lowest importance message (not key)
		lowestIdx := -1
		lowestImportance := 1.0
		for i, msg := range kept {
			if !msg.IsKey && msg.Importance < lowestImportance {
				lowestImportance = msg.Importance
				lowestIdx = i
			}
		}

		if lowestIdx >= 0 {
			newTokens -= kept[lowestIdx].Tokens
			kept = append(kept[:lowestIdx], kept[lowestIdx+1:]...)
		} else {
			break
		}
	}
	return kept
}