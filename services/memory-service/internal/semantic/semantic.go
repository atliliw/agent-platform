// Package semantic provides semantic memory functionality
// Semantic memory stores knowledge facts, concepts, and relationships
package semantic

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// ConceptType defines the type of concept
type ConceptType string

const (
	ConceptTypeEntity    ConceptType = "entity"    // Named entities (people, places, things)
	ConceptTypeFact      ConceptType = "fact"      // Facts and statements
	ConceptTypeRule      ConceptType = "rule"      // Rules and patterns
	ConceptTypeProcedure ConceptType = "procedure" // Procedures and workflows
	ConceptTypeConcept   ConceptType = "concept"   // Abstract concepts
)

// RelationType defines the type of relation between concepts
type RelationType string

const (
	RelationTypeIsA        RelationType = "is_a"        // Inheritance/category
	RelationTypeHasA       RelationType = "has_a"       // Composition
	RelationTypePartOf     RelationType = "part_of"     // Part-whole
	RelationTypeRelatedTo  RelationType = "related_to"  // General relation
	RelationTypeCauses     RelationType = "causes"      // Causation
	RelationTypePrecedes   RelationType = "precedes"   // Temporal order
	RelationTypeFollows    RelationType = "follows"     // Temporal order
	RelationTypeInstanceOf RelationType = "instance_of" // Instance
	RelationTypeSameAs     RelationType = "same_as"     // Equivalence
)

// Concept represents a concept in semantic memory
type Concept struct {
	ID          string                 `json:"id"`
	Type        ConceptType            `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Properties  map[string]interface{} `json:"properties"`
	Importance  float64                `json:"importance"` // 0-1 importance
	Confidence  float64                `json:"confidence"` // 0-1 confidence in this knowledge
	Source      string                 `json:"source"`     // Where this came from
	Vector      []float64              `json:"vector"`     // Embedding for similarity
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	AccessCount int                    `json:"access_count"` // How many times accessed
}

// Relation represents a relation between concepts
type Relation struct {
	ID          string      `json:"id"`
	FromConcept string      `json:"from_concept"` // Source concept ID
	ToConcept   string      `json:"to_concept"`   // Target concept ID
	Type        RelationType `json:"type"`
	Weight      float64     `json:"weight"`       // Strength of relation (0-1)
	Confidence  float64     `json:"confidence"`   // Confidence in this relation
	Evidence    []string    `json:"evidence"`     // Supporting evidence
	CreatedAt   time.Time   `json:"created_at"`
}

// KnowledgeGraph represents a graph of concepts and relations
type KnowledgeGraph struct {
	Concepts   map[string]*Concept   `json:"concepts"`
	Relations  map[string]*Relation  `json:"relations"`
	Edges      map[string][]string   `json:"edges"`      // concept -> related concept IDs
	ReverseEdges map[string][]string `json:"reverse_edges"` // reverse edges
	mu         sync.RWMutex
}

// NewKnowledgeGraph creates a new knowledge graph
func NewKnowledgeGraph() *KnowledgeGraph {
	return &KnowledgeGraph{
		Concepts:     make(map[string]*Concept),
		Relations:    make(map[string]*Relation),
		Edges:        make(map[string][]string),
		ReverseEdges: make(map[string][]string),
	}
}

// AddConcept adds a concept to the graph
func (g *KnowledgeGraph) AddConcept(ctx context.Context, concept *Concept) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if concept.ID == "" {
		concept.ID = generateConceptID()
	}

	now := time.Now()
	if concept.CreatedAt.IsZero() {
		concept.CreatedAt = now
	}
	concept.UpdatedAt = now

	g.Concepts[concept.ID] = concept
	g.initEdges(concept.ID)

	return nil
}

// GetConcept retrieves a concept by ID
func (g *KnowledgeGraph) GetConcept(ctx context.Context, id string) (*Concept, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	concept, ok := g.Concepts[id]
	if !ok {
		return nil, fmt.Errorf("concept not found: %s", id)
	}

	// Increment access count
	g.mu.RUnlock()
	g.mu.Lock()
	concept.AccessCount++
	g.mu.Unlock()
	g.mu.RLock()

	return concept, nil
}

// AddRelation adds a relation between concepts
func (g *KnowledgeGraph) AddRelation(ctx context.Context, relation *Relation) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if relation.ID == "" {
		relation.ID = generateRelationID()
	}

	// Validate concepts exist
	if _, ok := g.Concepts[relation.FromConcept]; !ok {
		return fmt.Errorf("source concept not found: %s", relation.FromConcept)
	}
	if _, ok := g.Concepts[relation.ToConcept]; !ok {
		return fmt.Errorf("target concept not found: %s", relation.ToConcept)
	}

	relation.CreatedAt = time.Now()
	g.Relations[relation.ID] = relation

	// Add edges
	g.addEdge(relation.FromConcept, relation.ToConcept)
	g.addReverseEdge(relation.ToConcept, relation.FromConcept)

	return nil
}

// GetRelations retrieves relations for a concept
func (g *KnowledgeGraph) GetRelations(ctx context.Context, conceptID string, relationType RelationType) ([]*Relation, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var relations []*Relation
	for _, rel := range g.Relations {
		if rel.FromConcept == conceptID && (relationType == "" || rel.Type == relationType) {
			relations = append(relations, rel)
		}
	}

	return relations, nil
}

// GetRelatedConcepts retrieves concepts related to a concept
func (g *KnowledgeGraph) GetRelatedConcepts(ctx context.Context, conceptID string, relationType RelationType, depth int) ([]*Concept, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	visited := make(map[string]bool)
	result := make([]*Concept, 0)

	g.traverse(conceptID, relationType, depth, visited, &result)

	return result, nil
}

// traverse traverses the graph to find related concepts
func (g *KnowledgeGraph) traverse(conceptID string, relationType RelationType, depth int, visited map[string]bool, result *[]*Concept) {
	if depth <= 0 || visited[conceptID] {
		return
	}

	visited[conceptID] = true

	// Get related concepts
	edges := g.Edges[conceptID]
	for _, relatedID := range edges {
		// Check relation type if specified
		if relationType != "" {
			found := false
			for _, rel := range g.Relations {
				if rel.FromConcept == conceptID && rel.ToConcept == relatedID && rel.Type == relationType {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if concept, ok := g.Concepts[relatedID]; ok && !visited[relatedID] {
			*result = append(*result, concept)
			g.traverse(relatedID, relationType, depth-1, visited, result)
		}
	}
}

// SearchConcepts searches concepts by name or description
func (g *KnowledgeGraph) SearchConcepts(ctx context.Context, query string) ([]*Concept, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	queryLower := strings.ToLower(query)
	var results []*Concept

	for _, concept := range g.Concepts {
		if strings.Contains(strings.ToLower(concept.Name), queryLower) ||
			strings.Contains(strings.ToLower(concept.Description), queryLower) {
			results = append(results, concept)
		}
	}

	// Sort by importance and access count
	sortConceptsByImportance(results)

	return results, nil
}

// FindSimilarConcepts finds similar concepts by embedding
func (g *KnowledgeGraph) FindSimilarConcepts(ctx context.Context, embedding []float64, topK int) ([]*Concept, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var scored []scoredConcept
	for _, concept := range g.Concepts {
		if concept.Vector != nil && len(concept.Vector) == len(embedding) {
			score := cosineSimilarity(embedding, concept.Vector)
			scored = append(scored, scoredConcept{concept: concept, score: score})
		}
	}

	// Sort by score
	sortConceptsByScore(scored)

	// Return top K
	result := make([]*Concept, 0, topK)
	for i := 0; i < topK && i < len(scored); i++ {
		result = append(result, scored[i].concept)
	}

	return result, nil
}

// GetConceptPath finds a path between two concepts
func (g *KnowledgeGraph) GetConceptPath(ctx context.Context, fromID, toID string) ([]string, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	// BFS to find shortest path
	queue := [][]string{{fromID}}
	visited := make(map[string]bool)
	visited[fromID] = true

	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]

		current := path[len(path)-1]
		if current == toID {
			return path, nil
		}

		for _, next := range g.Edges[current] {
			if !visited[next] {
				visited[next] = true
				newPath := append([]string{}, path...)
				newPath = append(newPath, next)
				queue = append(queue, newPath)
			}
		}
	}

	return nil, fmt.Errorf("no path found between %s and %s", fromID, toID)
}

// ExtractKnowledge extracts knowledge from text
func (g *KnowledgeGraph) ExtractKnowledge(ctx context.Context, text string) ([]*Concept, []*Relation, error) {
	// This would use an LLM in production
	// For now, return basic extraction
	concepts := extractEntities(text)
	relations := extractRelations(text)

	return concepts, relations, nil
}

// GetStats returns knowledge graph statistics
func (g *KnowledgeGraph) GetStats() map[string]interface{} {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := map[string]interface{}{
		"total_concepts": len(g.Concepts),
		"total_relations": len(g.Relations),
		"by_concept_type": make(map[ConceptType]int),
		"by_relation_type": make(map[RelationType]int),
		"avg_connections": 0,
	}

	for _, c := range g.Concepts {
		stats["by_concept_type"].(map[ConceptType]int)[c.Type]++
	}
	for _, r := range g.Relations {
		stats["by_relation_type"].(map[RelationType]int)[r.Type]++
	}

	// Calculate average connections
	var totalConnections int
	for _, edges := range g.Edges {
		totalConnections += len(edges)
	}
	if len(g.Concepts) > 0 {
		stats["avg_connections"] = float64(totalConnections) / float64(len(g.Concepts))
	}

	return stats
}

// Helper methods
func (g *KnowledgeGraph) initEdges(conceptID string) {
	if g.Edges[conceptID] == nil {
		g.Edges[conceptID] = make([]string, 0)
	}
	if g.ReverseEdges[conceptID] == nil {
		g.ReverseEdges[conceptID] = make([]string, 0)
	}
}

func (g *KnowledgeGraph) addEdge(from, to string) {
	g.initEdges(from)
	g.Edges[from] = append(g.Edges[from], to)
}

func (g *KnowledgeGraph) addReverseEdge(to, from string) {
	g.initEdges(to)
	g.ReverseEdges[to] = append(g.ReverseEdges[to], from)
}

// SemanticMemory combines knowledge graph with memory operations
type SemanticMemory struct {
	graph       *KnowledgeGraph
	maxConcepts int
	mu          sync.RWMutex
}

// NewSemanticMemory creates a new semantic memory
func NewSemanticMemory(maxConcepts int) *SemanticMemory {
	if maxConcepts <= 0 {
		maxConcepts = 5000
	}
	return &SemanticMemory{
		graph:       NewKnowledgeGraph(),
		maxConcepts: maxConcepts,
	}
}

// Store stores a concept
func (m *SemanticMemory) Store(ctx context.Context, concept *Concept) error {
	return m.graph.AddConcept(ctx, concept)
}

// StoreRelation stores a relation
func (m *SemanticMemory) StoreRelation(ctx context.Context, relation *Relation) error {
	return m.graph.AddRelation(ctx, relation)
}

// Recall retrieves concepts matching a query
func (m *SemanticMemory) Recall(ctx context.Context, query string, topK int) ([]*Concept, error) {
	return m.graph.SearchConcepts(ctx, query)
}

// RecallByEmbedding retrieves concepts by embedding similarity
func (m *SemanticMemory) RecallByEmbedding(ctx context.Context, embedding []float64, topK int) ([]*Concept, error) {
	return m.graph.FindSimilarConcepts(ctx, embedding, topK)
}

// GetRelated retrieves concepts related to a concept
func (m *SemanticMemory) GetRelated(ctx context.Context, conceptID string, relationType RelationType) ([]*Concept, error) {
	return m.graph.GetRelatedConcepts(ctx, conceptID, relationType, 2)
}

// GetGraph returns the underlying knowledge graph
func (m *SemanticMemory) GetGraph() *KnowledgeGraph {
	return m.graph
}

// Helper functions
func generateConceptID() string {
	return fmt.Sprintf("concept-%d", time.Now().UnixNano())
}

func generateRelationID() string {
	return fmt.Sprintf("rel-%d", time.Now().UnixNano())
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func sortConceptsByImportance(concepts []*Concept) {
	// Simple sort by importance
	for i := 0; i < len(concepts)-1; i++ {
		for j := i + 1; j < len(concepts); j++ {
			if concepts[j].Importance > concepts[i].Importance {
				concepts[i], concepts[j] = concepts[j], concepts[i]
			}
		}
	}
}

func sortConceptsByScore(scored []scoredConcept) {
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}
}

type scoredConcept struct {
	concept *Concept
	score   float64
}

// Basic entity extraction (placeholder for LLM-based extraction)
func extractEntities(text string) []*Concept {
	// In production, this would use an LLM or NER model
	// For now, return empty
	return []*Concept{}
}

// Basic relation extraction (placeholder for LLM-based extraction)
func extractRelations(text string) []*Relation {
	// In production, this would use an LLM or relation extraction model
	// For now, return empty
	return []*Relation{}
}