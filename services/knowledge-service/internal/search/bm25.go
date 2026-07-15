// Package search provides search algorithms for knowledge retrieval
package search

import (
	"math"
	"regexp"
	"strings"
)

// BM25 implements the BM25 ranking algorithm
type BM25 struct {
	// K1 controls term saturation (typically 1.2-2.0)
	K1 float64
	// B controls length normalization (typically 0.75)
	B float64
	// Documents stored for search
	documents []BM25Document
	// Average document length
	avgDocLen float64
	// Document frequency for each term
	docFreq map[string]int
	// Total number of documents
	numDocs int
}

// BM25Document represents a document for BM25 indexing
type BM25Document struct {
	ID       string
	Content  string
	TermFreq map[string]int
	Length   int
}

// NewBM25 creates a new BM25 indexer
func NewBM25() *BM25 {
	return &BM25{
		K1:      1.5,
		B:       0.75,
		docFreq: make(map[string]int),
	}
}

// SetParams sets BM25 parameters
func (b *BM25) SetParams(k1, bParam float64) {
	b.K1 = k1
	b.B = bParam
}

// AddDocument adds a document to the index
func (b *BM25) AddDocument(id, content string) {
	terms := b.tokenize(content)
	termFreq := make(map[string]int)
	for _, term := range terms {
		termFreq[term]++
	}

	doc := BM25Document{
		ID:       id,
		Content:  content,
		TermFreq: termFreq,
		Length:   len(terms),
	}
	b.documents = append(b.documents, doc)
	b.numDocs++

	// Update document frequency
	seen := make(map[string]bool)
	for term := range termFreq {
		if !seen[term] {
			b.docFreq[term]++
			seen[term] = true
		}
	}

	// Update average document length
	totalLen := 0.0
	for _, d := range b.documents {
		totalLen += float64(d.Length)
	}
	b.avgDocLen = totalLen / float64(b.numDocs)
}

// Search performs BM25 search and returns scored results
func (b *BM25) Search(query string, topK int) []BM25Result {
	if b.numDocs == 0 {
		return nil
	}

	queryTerms := b.tokenize(query)
	scores := make(map[string]float64)

	// Calculate BM25 score for each document
	for _, doc := range b.documents {
		score := 0.0
		for _, term := range queryTerms {
			score += b.scoreTerm(term, doc)
		}
		if score > 0 {
			scores[doc.ID] = score
		}
	}

	// Convert to results and sort
	results := make([]BM25Result, 0, len(scores))
	for id, score := range scores {
		// Find document content
		var content string
		for _, doc := range b.documents {
			if doc.ID == id {
				content = doc.Content
				break
			}
		}
		results = append(results, BM25Result{
			ID:      id,
			Score:   score,
			Content: content,
		})
	}

	// Sort by score descending
	b.sortResults(results)

	// Return top K
	if len(results) > topK {
		results = results[:topK]
	}

	return results
}

// BM25Result represents a search result
type BM25Result struct {
	ID      string
	Score   float64
	Content string
}

// scoreTerm calculates BM25 score for a term in a document
func (b *BM25) scoreTerm(term string, doc BM25Document) float64 {
	// Get term frequency in document
	tf := float64(doc.TermFreq[term])
	if tf == 0 {
		return 0
	}

	// Get document frequency
	df := float64(b.docFreq[term])
	if df == 0 {
		return 0
	}

	// Calculate IDF: log((N - df + 0.5) / (df + 0.5))
	idf := math.Log((float64(b.numDocs) - df + 0.5) / (df + 0.5))

	// Calculate term saturation
	docLen := float64(doc.Length)
	tfNorm := (tf * (b.K1 + 1)) / (tf + b.K1*(1-b.B+b.B*docLen/b.avgDocLen))

	return idf * tfNorm
}

// tokenize splits text into tokens
func (b *BM25) tokenize(text string) []string {
	// Convert to lowercase
	text = strings.ToLower(text)

	// Remove punctuation and split by whitespace
	reg := regexp.MustCompile(`[^\w\s]`)
	text = reg.ReplaceAllString(text, " ")

	// Split by whitespace
	words := strings.Fields(text)

	// Filter out short words and stop words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"is": true, "are": true, "was": true, "were": true, "be": true,
		"been": true, "being": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true, "must": true,
		"shall": true, "can": true, "need": true, "dare": true, "ought": true,
		"used": true, "it": true, "its": true, "this": true, "that": true,
		"these": true, "those": true, "i": true, "you": true, "he": true,
		"she": true, "we": true, "they": true, "what": true, "which": true,
		"who": true, "whom": true, "whose": true, "where": true, "when": true,
		"why": true, "how": true, "as": true, "if": true, "then": true,
		"so": true, "no": true, "not": true, "only": true, "own": true,
		"same": true, "than": true, "too": true, "very": true, "just": true,
		// Chinese stop words
		"的": true, "了": true, "是": true, "在": true, "和": true,
		"有": true, "我": true, "他": true, "她": true, "它": true,
		"这": true, "那": true, "就": true, "也": true, "都": true,
		"而": true, "及": true, "与": true, "或": true, "但": true,
		"如": true, "若": true, "则": true, "因为": true, "所以": true,
	}

	var tokens []string
	for _, word := range words {
		// Skip short words
		if len(word) < 2 {
			continue
		}
		// Skip stop words
		if stopWords[word] {
			continue
		}
		tokens = append(tokens, word)
	}

	return tokens
}

// sortResults sorts results by score descending
func (b *BM25) sortResults(results []BM25Result) {
	// Simple bubble sort for clarity (can be optimized)
	n := len(results)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if results[j].Score < results[j+1].Score {
				results[j], results[j+1] = results[j+1], results[j]
			}
		}
	}
}

// Clear clears the index
func (b *BM25) Clear() {
	b.documents = nil
	b.docFreq = make(map[string]int)
	b.numDocs = 0
	b.avgDocLen = 0
}

// GetDocumentCount returns the number of indexed documents
func (b *BM25) GetDocumentCount() int {
	return b.numDocs
}

// GetVocabularySize returns the number of unique terms
func (b *BM25) GetVocabularySize() int {
	return len(b.docFreq)
}
