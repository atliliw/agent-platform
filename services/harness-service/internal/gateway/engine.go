// Package gateway provides LLM Gateway functionality
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"agent-platform/pkg/llm"
)

// GatewayEngine is the main gateway engine that manages multiple LLM providers
type GatewayEngine struct {
	configs     map[string]*GatewayConfig    // provider configs
	routes      []*GatewayRoute              // routing rules
	stats       map[string]*GatewayStats     // provider statistics
	llmClients  map[string]llm.Client        // LLM clients by provider
	mu          sync.RWMutex                 // mutex for thread safety
	lbCounter   int                          // counter for round-robin
	strategy    LoadBalanceStrategy          // load balance strategy
	repo        *Repository                  // persistence layer
	rateLimiter *RateLimiter                 // rate limiter
}

// NewGatewayEngine creates a new gateway engine with DB persistence
func NewGatewayEngine(repo *Repository) *GatewayEngine {
	e := &GatewayEngine{
		repo:        repo,
		rateLimiter: NewRateLimiter(),
		configs:     make(map[string]*GatewayConfig),
		routes:      []*GatewayRoute{},
		stats:       make(map[string]*GatewayStats),
		llmClients:  make(map[string]llm.Client),
		strategy:    LeastLatency,
	}
	if repo != nil {
		e.loadFromDB()
	}
	return e
}

// NewGatewayEngineMemory creates a new gateway engine in memory-only mode
func NewGatewayEngineMemory() *GatewayEngine {
	return NewGatewayEngine(nil)
}
// loadFromDB loads configs, routes, and stats from the repository
func (e *GatewayEngine) loadFromDB() {
	ctx := context.Background()

	// Load configs
	configs, err := e.repo.ListConfigs(ctx, "")
	if err == nil {
		for _, cfg := range configs {
			e.configs[cfg.ID] = cfg
			// Reconstruct LLM client
			llmConfig := llm.Config{
				Provider:  cfg.Provider,
				APIKey:    cfg.APIKey,
				BaseURL:   cfg.BaseURL,
				MaxTokens: cfg.Timeout * 1000,
			}
			client, err := llm.NewClient(llmConfig)
			if err == nil {
				e.llmClients[cfg.Provider] = client
			}
			// Configure rate limiter
			e.rateLimiter.Configure(cfg.Provider, cfg.RateLimit)
		}
	}

	// Load routes
	routes, err := e.repo.ListRoutes(ctx, "")
	if err == nil {
		e.routes = routes
	}

	// Load stats
	stats, err := e.repo.GetStats(ctx)
	if err == nil {
		for k, v := range stats {
			e.stats[k] = v
		}
	}
}

// Initialize initializes the gateway engine with default providers
func (e *GatewayEngine) Initialize(ctx context.Context, defaultConfigs []*GatewayConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, cfg := range defaultConfigs {
		if err := e.addProviderConfig(cfg); err != nil {
			return fmt.Errorf("add provider %s: %w", cfg.Provider, err)
		}
	}

	return nil
}
// addProviderConfig adds a provider configuration (must be called with lock held)
func (e *GatewayEngine) addProviderConfig(cfg *GatewayConfig) error {
	// Create LLM client for the provider
	llmConfig := llm.Config{
		Provider:  cfg.Provider,
		APIKey:    cfg.APIKey,
		BaseURL:   cfg.BaseURL,
		MaxTokens: cfg.Timeout * 1000, // Convert timeout seconds to max tokens approximation
	}

	client, err := llm.NewClient(llmConfig)
	if err != nil {
		return fmt.Errorf("create LLM client: %w", err)
	}

	e.configs[cfg.ID] = cfg
	e.llmClients[cfg.Provider] = client
	e.stats[cfg.Provider] = &GatewayStats{
		Provider:       cfg.Provider,
		LastActiveTime: time.Now(),
	}

	return nil
}

// AddConfig adds a new gateway configuration
func (e *GatewayEngine) AddConfig(ctx context.Context, cfg *GatewayConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cfg.ID = generateID()
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = time.Now()

	if err := e.addProviderConfig(cfg); err != nil {
		return err
	}

	// Persist to DB
	if e.repo != nil {
		if err := e.repo.SaveConfig(ctx, cfg); err != nil {
			return fmt.Errorf("persist config: %w", err)
		}
	}

	// Configure rate limiter
	e.rateLimiter.Configure(cfg.Provider, cfg.RateLimit)

	return nil
}
// ListConfigs lists all gateway configurations
func (e *GatewayEngine) ListConfigs(ctx context.Context, tenantID string) ([]*GatewayConfig, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var configs []*GatewayConfig
	for _, cfg := range e.configs {
		if tenantID == "" || cfg.TenantID == tenantID {
			configs = append(configs, cfg)
		}
	}

	return configs, nil
}

// GetConfig gets a gateway configuration by ID
func (e *GatewayEngine) GetConfig(ctx context.Context, id string) (*GatewayConfig, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	cfg, ok := e.configs[id]
	if !ok {
		return nil, fmt.Errorf("config not found: %s", id)
	}

	return cfg, nil
}

// UpdateConfig updates a gateway configuration
func (e *GatewayEngine) UpdateConfig(ctx context.Context, cfg *GatewayConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, ok := e.configs[cfg.ID]
	if !ok {
		return fmt.Errorf("config not found: %s", cfg.ID)
	}

	cfg.UpdatedAt = time.Now()
	cfg.CreatedAt = existing.CreatedAt
	e.configs[cfg.ID] = cfg

	// Recreate LLM client if provider or API key changed
	if cfg.Provider != existing.Provider || cfg.APIKey != existing.APIKey || cfg.BaseURL != existing.BaseURL {
		llmConfig := llm.Config{
			Provider:  cfg.Provider,
			APIKey:    cfg.APIKey,
			BaseURL:   cfg.BaseURL,
			MaxTokens: cfg.Timeout * 1000,
		}

		client, err := llm.NewClient(llmConfig)
		if err != nil {
			return fmt.Errorf("create LLM client: %w", err)
		}

		e.llmClients[cfg.Provider] = client
	}

	// Persist to DB
	if e.repo != nil {
		if err := e.repo.UpdateConfig(ctx, cfg); err != nil {
			return fmt.Errorf("persist config update: %w", err)
		}
	}

	// Update rate limiter
	e.rateLimiter.Configure(cfg.Provider, cfg.RateLimit)

	return nil
}
// DeleteConfig deletes a gateway configuration
func (e *GatewayEngine) DeleteConfig(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cfg, ok := e.configs[id]
	if !ok {
		return fmt.Errorf("config not found: %s", id)
	}

	delete(e.configs, id)
	delete(e.llmClients, cfg.Provider)
	delete(e.stats, cfg.Provider)

	// Remove from rate limiter
	e.rateLimiter.Remove(cfg.Provider)

	// Persist to DB
	if e.repo != nil {
		if err := e.repo.DeleteConfig(ctx, id); err != nil {
			return fmt.Errorf("persist config delete: %w", err)
		}
	}

	return nil
}

// Chat sends a chat request through the gateway
func (e *GatewayEngine) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	startTime := time.Now()

	// Route request to appropriate model
	provider, model, err := e.routeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("route request: %w", err)
	}

	// Check rate limit
	e.mu.RLock()
	cfg, cfgOk := e.configs[provider]
	e.mu.RUnlock()
	if cfgOk && !e.rateLimiter.AllowRequest(provider, cfg.RateLimit) {
		// Try fallback on rate limit
		fallbackModels := e.getFallbackModels(provider)
		for _, fallback := range fallbackModels {
			fallbackProvider, fallbackModelID := e.parseModelRef(fallback)
			e.mu.RLock()
			fallbackCfg, ok := e.configs[fallbackProvider]
			e.mu.RUnlock()
			if ok && e.rateLimiter.AllowRequest(fallbackProvider, fallbackCfg.RateLimit) {
				provider = fallbackProvider
				model = fallbackModelID
				break
			}
		}
		// If still rate limited after trying fallbacks, return error
		if !e.rateLimiter.AllowRequest(provider, cfg.RateLimit) {
			return nil, fmt.Errorf("rate limit exceeded for provider: %s", provider)
		}
	}

	// Execute with fallback support
	resp, usedFallback, _, err := e.executeWithFallback(ctx, provider, model, req)
	if err != nil {
		return nil, err
	}

	latency := time.Since(startTime).Milliseconds()

	// Update stats
	e.updateStats(provider, resp, latency, err == nil)

	return &ChatResponse{
		Content:       resp.Content,
		Model:         model,
		Provider:      provider,
		TotalTokens:   int64(resp.TotalTokens),
		Cost:          resp.Cost,
		Latency:       latency,
		UsedFallback:  usedFallback,
		OriginalModel: req.Model,
	}, nil
}
// ChatStream sends a streaming chat request through the gateway
func (e *GatewayEngine) ChatStream(ctx context.Context, req *ChatRequest) (<-chan ChatStreamChunk, error) {
	// Route request to appropriate model
	provider, model, err := e.routeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("route request: %w", err)
	}

	// Check rate limit
	e.mu.RLock()
	cfg, cfgOk := e.configs[provider]
	e.mu.RUnlock()
	if cfgOk && !e.rateLimiter.AllowRequest(provider, cfg.RateLimit) {
		// Try fallback on rate limit
		fallbackModels := e.getFallbackModels(provider)
		for _, fallback := range fallbackModels {
			fallbackProvider, fallbackModelID := e.parseModelRef(fallback)
			e.mu.RLock()
			fallbackCfg, ok := e.configs[fallbackProvider]
			e.mu.RUnlock()
			if ok && e.rateLimiter.AllowRequest(fallbackProvider, fallbackCfg.RateLimit) {
				provider = fallbackProvider
				model = fallbackModelID
				break
			}
		}
		// If still rate limited after trying fallbacks, return error
		if cfgOk && !e.rateLimiter.AllowRequest(provider, cfg.RateLimit) {
			return nil, fmt.Errorf("rate limit exceeded for provider: %s", provider)
		}
	}

	// Get LLM client
	e.mu.RLock()
	client, ok := e.llmClients[provider]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("provider not available: %s", provider)
	}

	// Build LLM request
	llmReq := &llm.ChatRequest{
		Messages:    convertMessages(req.Messages),
		Model:       model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	// Execute streaming request
	stream, err := client.ChatStream(ctx, llmReq)
	if err != nil {
		// Try fallback
		fallbackModels := e.getFallbackModels(provider)
		for _, fallback := range fallbackModels {
			fallbackProvider, fallbackModelID := e.parseModelRef(fallback)
			e.mu.RLock()
			fallbackClient, ok := e.llmClients[fallbackProvider]
			e.mu.RUnlock()

			if !ok {
				continue
			}

			llmReq.Model = fallbackModelID
			stream, err = fallbackClient.ChatStream(ctx, llmReq)
			if err == nil {
				// Return wrapped stream
				return e.wrapStream(stream, fallbackModelID, fallbackProvider, true, req.Model), nil
			}
		}
		return nil, fmt.Errorf("all providers failed: %w", err)
	}

	return e.wrapStream(stream, model, provider, false, req.Model), nil
}
// ChatStreamChunk represents a streaming chunk
type ChatStreamChunk struct {
	Content       string `json:"content"`
	Model         string `json:"model"`
	Provider      string `json:"provider"`
	UsedFallback  bool   `json:"used_fallback"`
	OriginalModel string `json:"original_model"`
	Done          bool   `json:"done"`
	Error         error  `json:"error,omitempty"`
}

// wrapStream wraps an LLM stream into a gateway stream
func (e *GatewayEngine) wrapStream(stream <-chan llm.ChatStreamChunk, model, provider string, usedFallback bool, originalModel string) <-chan ChatStreamChunk {
	ch := make(chan ChatStreamChunk, 100)

	go func() {
		defer close(ch)
		for chunk := range stream {
			ch <- ChatStreamChunk{
				Content:       chunk.Content,
				Model:         model,
				Provider:      provider,
				UsedFallback:  usedFallback,
				OriginalModel: originalModel,
				Done:          chunk.Done,
				Error:         chunk.Error,
			}
		}
	}()

	return ch
}
// routeRequest routes a request to the appropriate provider and model
func (e *GatewayEngine) routeRequest(req *ChatRequest) (provider, model string, err error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// If provider is specified, use it directly
	if req.Provider != "" {
		_, ok := e.llmClients[req.Provider]
		if !ok {
			return "", "", fmt.Errorf("provider not found: %s", req.Provider)
		}
		model = req.Model
		if model == "" {
			// Use default model for this provider
			for _, cfg := range e.configs {
				if cfg.Provider == req.Provider && cfg.Enabled {
					models, err := parseModels(cfg.Models)
					if err == nil && len(models) > 0 {
						model = models[0].ModelID
						break
					}
				}
			}
		}
		return req.Provider, model, nil
	}

	// Use routing rules if available
	for _, route := range e.routes {
		if route.Enabled && route.Pattern != "" {
			if matchesPattern(req, route.Pattern) {
				provider, model = e.parseModelRef(route.ModelID)
				return provider, model, nil
			}
		}
	}

	// Use load balancing to select best provider
	provider, model = e.selectBestModel(req)
	if provider == "" {
		return "", "", fmt.Errorf("no available providers")
	}

	return provider, model, nil
}
// executeWithFallback executes request with fallback support
func (e *GatewayEngine) executeWithFallback(ctx context.Context, provider, model string, req *ChatRequest) (resp *llm.ChatResponse, usedFallback bool, fallbackModel string, err error) {
	e.mu.RLock()
	client, ok := e.llmClients[provider]
	e.mu.RUnlock()

	if !ok {
		return nil, false, "", fmt.Errorf("provider not available: %s", provider)
	}

	// Build LLM request
	llmReq := &llm.ChatRequest{
		Messages:    convertMessages(req.Messages),
		Model:       model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	// Try primary provider
	resp, err = client.Chat(ctx, llmReq)
	if err == nil {
		return resp, false, "", nil
	}

	// Check if should fallback
	if !IsFallbackError(err) {
		return nil, false, "", err
	}

	// Get fallback models
	fallbackModels := e.getFallbackModels(provider)
	for _, fallback := range fallbackModels {
		fallbackProvider, fallbackModelID := e.parseModelRef(fallback)
		e.mu.RLock()
		fallbackClient, ok := e.llmClients[fallbackProvider]
		e.mu.RUnlock()

		if !ok {
			continue
		}

		llmReq.Model = fallbackModelID
		resp, err = fallbackClient.Chat(ctx, llmReq)
		if err == nil {
			return resp, true, fallbackModelID, nil
		}
	}

	return nil, false, "", fmt.Errorf("all providers failed: %w", err)
}
// selectBestModel selects the best model using load balancing strategy
func (e *GatewayEngine) selectBestModel(req *ChatRequest) (provider, model string) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Get all enabled providers with their models
	var candidates []struct {
		provider string
		model    string
		stats    *GatewayStats
		config   *GatewayConfig
	}

	for _, cfg := range e.configs {
		if !cfg.Enabled {
			continue
		}
		models, err := parseModels(cfg.Models)
		if err != nil || len(models) == 0 {
			continue
		}

		modelID := req.Model
		if modelID == "" {
			modelID = models[0].ModelID
		}

		stats := e.stats[cfg.Provider]
		candidates = append(candidates, struct {
			provider string
			model    string
			stats    *GatewayStats
			config   *GatewayConfig
		}{
			provider: cfg.Provider,
			model:    modelID,
			stats:    stats,
			config:   cfg,
		})
	}

	if len(candidates) == 0 {
		return "", ""
	}

	// Apply load balancing strategy
	switch e.strategy {
	case RoundRobin:
		idx := e.lbCounter % len(candidates)
		e.lbCounter++
		return candidates[idx].provider, candidates[idx].model

	case LeastLatency:
		best := candidates[0]
		for _, c := range candidates {
			if c.stats.AvgLatency < best.stats.AvgLatency {
				best = c
			}
		}
		return best.provider, best.model

	case LeastCost:
		best := candidates[0]
		bestCost := getBestCost(best.config, best.model)
		for _, c := range candidates {
			cost := getBestCost(c.config, c.model)
			if cost < bestCost {
				best = c
				bestCost = cost
			}
		}
		return best.provider, best.model

	default:
		// Default to first candidate
		return candidates[0].provider, candidates[0].model
	}
}
// getBestCost gets the cost for a model from config
func getBestCost(cfg *GatewayConfig, modelID string) float64 {
	models, err := parseModels(cfg.Models)
	if err != nil {
		return 999999 // high cost as fallback
	}

	for _, m := range models {
		if m.ModelID == modelID {
			return m.InputPrice + m.OutputPrice
		}
	}

	return 999999
}

// getFallbackModels gets fallback models for a provider
func (e *GatewayEngine) getFallbackModels(provider string) []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Check routes for fallback configuration
	for _, route := range e.routes {
		if route.ModelID != "" && route.Enabled {
			routeProvider, _ := e.parseModelRef(route.ModelID)
			if routeProvider == provider {
				var fallbacks []string
				if err := json.Unmarshal([]byte(route.Fallbacks), &fallbacks); err == nil {
					return fallbacks
				}
			}
		}
	}

	// Default fallback: try other providers in priority order
	var fallbacks []string
	for _, cfg := range e.configs {
		if cfg.Provider != provider && cfg.Enabled {
			models, err := parseModels(cfg.Models)
			if err == nil && len(models) > 0 {
				fallbacks = append(fallbacks, fmt.Sprintf("%s:%s", cfg.Provider, models[0].ModelID))
			}
		}
	}

	// Sort by priority
	sortByPriority(fallbacks, e.configs)
	return fallbacks
}
// updateStats updates provider statistics
func (e *GatewayEngine) updateStats(provider string, resp *llm.ChatResponse, latency int64, success bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	stats, ok := e.stats[provider]
	if !ok {
		stats = &GatewayStats{
			Provider:       provider,
			LastActiveTime: time.Now(),
		}
		e.stats[provider] = stats
	}

	stats.TotalRequests++
	if success {
		stats.SuccessCount++
	} else {
		stats.ErrorCount++
	}

	// Update average latency (rolling average)
	if stats.AvgLatency == 0 {
		stats.AvgLatency = float64(latency)
	} else {
		stats.AvgLatency = (stats.AvgLatency * 0.9) + (float64(latency) * 0.1)
	}

	stats.TotalTokens += int64(resp.TotalTokens)
	stats.TotalCost += resp.Cost
	stats.LastActiveTime = time.Now()

	// Persist stats to DB
	if e.repo != nil {
		if err := e.repo.SaveStats(context.Background(), provider, stats); err != nil {
			// Log but do not fail the request on stats persistence error
		fmt.Printf("Warning: failed to persist gateway stats for %s: %v\n", provider, err)
		}
	}
}

// GetStats returns statistics for all providers
func (e *GatewayEngine) GetStats(ctx context.Context) (map[string]*GatewayStats, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]*GatewayStats)
	for k, v := range e.stats {
		result[k] = v
	}

	return result, nil
}

// SetLoadBalanceStrategy sets the load balancing strategy
func (e *GatewayEngine) SetLoadBalanceStrategy(strategy LoadBalanceStrategy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.strategy = strategy
}
// AddRoute adds a routing rule
func (e *GatewayEngine) AddRoute(ctx context.Context, route *GatewayRoute) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	route.ID = generateID()
	route.CreatedAt = time.Now()
	route.UpdatedAt = time.Now()
	e.routes = append(e.routes, route)

	// Persist to DB
	if e.repo != nil {
		if err := e.repo.SaveRoute(ctx, route); err != nil {
			return fmt.Errorf("persist route: %w", err)
		}
	}

	return nil
}

// ListRoutes lists all routing rules
func (e *GatewayEngine) ListRoutes(ctx context.Context, tenantID string) ([]*GatewayRoute, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var routes []*GatewayRoute
	for _, route := range e.routes {
		if tenantID == "" || route.TenantID == tenantID {
			routes = append(routes, route)
		}
	}

	return routes, nil
}

// DeleteRoute deletes a gateway route by ID
func (e *GatewayEngine) DeleteRoute(ctx context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, route := range e.routes {
		if route.ID == id {
			e.routes = append(e.routes[:i], e.routes[i+1:]...)
			// Persist to DB
			if e.repo != nil {
				if err := e.repo.DeleteRoute(ctx, id); err != nil {
					return fmt.Errorf("persist route delete: %w", err)
				}
			}
			return nil
		}
	}

	return fmt.Errorf("route not found: %s", id)
}

// Helper functions

func generateID() string {
	return fmt.Sprintf("gw-%d", time.Now().UnixNano())
}

func convertMessages(messages []Message) []llm.Message {
	result := make([]llm.Message, len(messages))
	for i, m := range messages {
		result[i] = llm.Message{
			Role:    m.Role,
			Content: m.Content,
		}
	}
	return result
}

func parseModels(modelsJSON string) ([]ModelConfig, error) {
	var models []ModelConfig
	if modelsJSON == "" {
		return models, nil
	}
	err := json.Unmarshal([]byte(modelsJSON), &models)
	return models, err
}

func (e *GatewayEngine) parseModelRef(ref string) (provider, model string) {
	parts := splitModelRef(ref)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	// If only model, find provider that has it
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, cfg := range e.configs {
		models, _ := parseModels(cfg.Models)
		for _, m := range models {
			if m.ModelID == ref {
				return cfg.Provider, ref
			}
		}
	}
	return "", ref
}

func splitModelRef(ref string) []string {
	for i := 0; i < len(ref); i++ {
		if ref[i] == ":"[0] {
			return []string{ref[:i], ref[i+1:]}
		}
	}
	return []string{ref}
}

func matchesPattern(req *ChatRequest, pattern string) bool {
	switch pattern {
	case "high-latency":
		return req.Parameters["priority"] == "speed"
	case "cost-sensitive":
		return req.Parameters["priority"] == "cost"
	case "quality-first":
		return req.Parameters["priority"] == "quality"
	default:
		return false
	}
}

func sortByPriority(fallbacks []string, configs map[string]*GatewayConfig) {
	for i := 0; i < len(fallbacks)-1; i++ {
		for j := i + 1; j < len(fallbacks); j++ {
			providerI := splitModelRef(fallbacks[i])[0]
			providerJ := splitModelRef(fallbacks[j])[0]
			priorityI := getPriority(configs, providerI)
			priorityJ := getPriority(configs, providerJ)
			if priorityJ < priorityI {
				fallbacks[i], fallbacks[j] = fallbacks[j], fallbacks[i]
			}
		}
	}
}

func getPriority(configs map[string]*GatewayConfig, provider string) int {
	for _, cfg := range configs {
		if cfg.Provider == provider {
			return cfg.Priority
		}
	}
	return 999 // high priority as default
}