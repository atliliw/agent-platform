// Package chunker provides text chunking functionality
package chunker

import (
	"strings"
)

// TokenChunker chunks text by token count
type TokenChunker struct {
	chunkTokens  int
	overlapChars int
}

// NewTokenChunker creates a new token chunker
func NewTokenChunker(chunkTokens, overlapChars int) *TokenChunker {
	return &TokenChunker{
		chunkTokens:  chunkTokens,
		overlapChars: overlapChars,
	}
}

// Chunk splits text into chunks
func (c *TokenChunker) Chunk(text string) []string {
	if text == "" {
		return nil
	}

	// Estimate chars per token (rough approximation)
	// Chinese: ~1 char per token, English: ~4 chars per token
	maxChars := c.chunkTokens * 2

	chars := []rune(text)
	var chunks []string

	start := 0
	for start < len(chars) {
		end := c.findSplitEnd(chars, start, maxChars)
		chunk := string(chars[start:end])
		chunks = append(chunks, chunk)

		// Move start with overlap
		nextStart := end - c.overlapChars
		if nextStart <= start {
			nextStart = end
		}
		start = nextStart
	}

	return chunks
}

func (c *TokenChunker) findSplitEnd(chars []rune, start, maxChars int) int {
	end := start + maxChars
	if end >= len(chars) {
		return len(chars)
	}

	// Try to find paragraph break
	for i := end - 1; i >= start; i-- {
		if chars[i] == '\n' && i+1 < len(chars) && chars[i+1] == '\n' {
			return i + 2
		}
	}

	// Try to find sentence end (Chinese)
	for i := end - 1; i >= start; i-- {
		if chars[i] == '。' || chars[i] == '！' || chars[i] == '？' {
			return i + 1
		}
	}

	// Try to find sentence end (English)
	for i := end - 1; i >= start; i-- {
		if chars[i] == '.' || chars[i] == '!' || chars[i] == '?' {
			return i + 1
		}
	}

	// Try to find newline
	for i := end - 1; i >= start; i-- {
		if chars[i] == '\n' {
			return i + 1
		}
	}

	// Try to find word break
	for i := end - 1; i >= start; i-- {
		if chars[i] == ' ' || chars[i] == ',' || chars[i] == '，' {
			return i + 1
		}
	}

	return end
}

// SemanticChunker chunks text by semantic similarity
type SemanticChunker struct {
	minChunkChars int
	maxChunkChars int
	embedFunc     func(string) ([]float64, error)
}

// NewSemanticChunker creates a new semantic chunker
func NewSemanticChunker(minChars, maxChars int, embedFunc func(string) ([]float64, error)) *SemanticChunker {
	return &SemanticChunker{
		minChunkChars: minChars,
		maxChunkChars: maxChars,
		embedFunc:     embedFunc,
	}
}

// Chunk splits text into semantically coherent chunks
func (c *SemanticChunker) Chunk(text string) ([]string, error) {
	sentences := c.splitSentences(text)
	if len(sentences) <= 1 {
		return []string{text}, nil
	}

	// Get embeddings for each sentence
	var embeddings [][]float64
	for _, s := range sentences {
		emb, err := c.embedFunc(s)
		if err != nil {
			return nil, err
		}
		embeddings = append(embeddings, emb)
	}

	// Find boundaries based on similarity
	var boundaries []int
	boundaries = append(boundaries, 0)
	currentLen := 0

	for i := 1; i < len(sentences); i++ {
		sim := cosineSimilarity(embeddings[i-1], embeddings[i])
		sentLen := len([]rune(sentences[i]))

		// Low similarity = topic boundary
		if sim < 0.45 && currentLen >= c.minChunkChars {
			boundaries = append(boundaries, i)
			currentLen = 0
		} else if currentLen+sentLen > c.maxChunkChars {
			boundaries = append(boundaries, i)
			currentLen = 0
		} else {
			currentLen += sentLen
		}
	}
	boundaries = append(boundaries, len(sentences))

	// Build chunks
	var chunks []string
	for i := 0; i < len(boundaries)-1; i++ {
		start := boundaries[i]
		end := boundaries[i+1]
		chunk := strings.Join(sentences[start:end], "")
		if strings.TrimSpace(chunk) != "" {
			chunks = append(chunks, chunk)
		}
	}

	return chunks, nil
}

func (c *SemanticChunker) splitSentences(text string) []string {
	separators := []rune{'。', '！', '？', '!', '?', '\n'}
	var sentences []string
	var current strings.Builder

	for _, r := range text {
		current.WriteRune(r)
		for _, sep := range separators {
			if r == sep {
				s := strings.TrimSpace(current.String())
				if s != "" {
					sentences = append(sentences, s)
				}
				current.Reset()
				break
			}
		}
	}

	// Add remaining text
	if s := strings.TrimSpace(current.String()); s != "" {
		sentences = append(sentences, s)
	}

	return sentences
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}

	if na == 0 || nb == 0 {
		return 0
	}

	return dot / (sqrt(na) * sqrt(nb))
}

func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}