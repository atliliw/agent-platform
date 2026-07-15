// Package gateway provides LLM Gateway functionality
package gateway

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GatewayConfigDB is the database model for gateway configurations
type GatewayConfigDB struct {
	ID          string    `gorm:"primaryKey"`
	Name        string
	Description string
	Provider    string    `gorm:"index"`
	APIKey      string
	BaseURL     string
	Models      string    `gorm:"type:text"`
	RateLimit   int
	Timeout     int
	RetryCount  int
	Priority    int
	Enabled     bool      `gorm:"index"`
	TenantID    string    `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// GatewayRouteDB is the database model for gateway routes
type GatewayRouteDB struct {
	ID        string    `gorm:"primaryKey"`
	Name      string
	Pattern   string
	ModelID   string
	Fallbacks string    `gorm:"type:text"`
	TenantID  string    `gorm:"index"`
	Enabled   bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GatewayStatsDB is the database model for gateway statistics
type GatewayStatsDB struct {
	ID             string    `gorm:"primaryKey"`
	Provider       string    `gorm:"uniqueIndex"`
	TotalRequests  int64
	SuccessCount   int64
	ErrorCount     int64
	AvgLatency     float64
	TotalTokens    int64
	TotalCost      float64
	LastActiveTime time.Time
}

// Repository provides data access for gateway configurations, routes, and stats
type Repository struct {
	db *gorm.DB
	mu sync.RWMutex
	// In-memory storage for memory mode
	configDBs map[string]*GatewayConfigDB
	routeDBs  map[string]*GatewayRouteDB
	statsDBs  map[string]*GatewayStatsDB
}

// NewRepository creates a new gateway repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db:        db,
		configDBs: make(map[string]*GatewayConfigDB),
		routeDBs:  make(map[string]*GatewayRouteDB),
		statsDBs:  make(map[string]*GatewayStatsDB),
	}
}

// AutoMigrate runs database migrations for gateway tables
func (r *Repository) AutoMigrate() error {
	if r.db == nil {
		return nil
	}
	return r.db.AutoMigrate(&GatewayConfigDB{}, &GatewayRouteDB{}, &GatewayStatsDB{})
}
// SaveConfig saves a gateway configuration
func (r *Repository) SaveConfig(ctx context.Context, cfg *GatewayConfig) error {
	if cfg.ID == "" {
		cfg.ID = uuid.New().String()
	}
	if cfg.CreatedAt.IsZero() {
		cfg.CreatedAt = time.Now()
	}
	cfg.UpdatedAt = time.Now()

	record := configDomainToDB(cfg)

	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.configDBs[record.ID] = record
		return nil
	}

	return r.db.WithContext(ctx).Create(record).Error
}

// GetConfig retrieves a gateway configuration by ID
func (r *Repository) GetConfig(ctx context.Context, id string) (*GatewayConfig, error) {
	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		record, ok := r.configDBs[id]
		if !ok {
			return nil, fmt.Errorf("config not found: %s", id)
		}
		return configDBToDomain(record), nil
	}

	var record GatewayConfigDB
	if err := r.db.WithContext(ctx).First(&record, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return configDBToDomain(&record), nil
}

// ListConfigs lists gateway configurations by tenant
func (r *Repository) ListConfigs(ctx context.Context, tenantID string) ([]*GatewayConfig, error) {
	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		var results []*GatewayConfig
		for _, record := range r.configDBs {
			if tenantID == "" || record.TenantID == tenantID {
				results = append(results, configDBToDomain(record))
			}
		}
		return results, nil
	}

	var records []GatewayConfigDB
	query := r.db.WithContext(ctx)
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if err := query.Find(&records).Error; err != nil {
		return nil, err
	}

	results := make([]*GatewayConfig, len(records))
	for i, record := range records {
		results[i] = configDBToDomain(&record)
	}
	return results, nil
}

// UpdateConfig updates a gateway configuration
func (r *Repository) UpdateConfig(ctx context.Context, cfg *GatewayConfig) error {
	cfg.UpdatedAt = time.Now()
	record := configDomainToDB(cfg)

	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.configDBs[record.ID] = record
		return nil
	}

	return r.db.WithContext(ctx).Save(record).Error
}

// DeleteConfig deletes a gateway configuration by ID
func (r *Repository) DeleteConfig(ctx context.Context, id string) error {
	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	delete(r.configDBs, id)
		return nil
	}

	return r.db.WithContext(ctx).Delete(&GatewayConfigDB{}, "id = ?", id).Error
}
// SaveRoute saves a gateway route
func (r *Repository) SaveRoute(ctx context.Context, route *GatewayRoute) error {
	if route.ID == "" {
		route.ID = uuid.New().String()
	}
	if route.CreatedAt.IsZero() {
		route.CreatedAt = time.Now()
	}
	route.UpdatedAt = time.Now()

	record := routeDomainToDB(route)

	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.routeDBs[record.ID] = record
		return nil
	}

	return r.db.WithContext(ctx).Create(record).Error
}

// ListRoutes lists gateway routes by tenant
func (r *Repository) ListRoutes(ctx context.Context, tenantID string) ([]*GatewayRoute, error) {
	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		var results []*GatewayRoute
		for _, record := range r.routeDBs {
			if tenantID == "" || record.TenantID == tenantID {
				results = append(results, routeDBToDomain(record))
			}
		}
		return results, nil
	}

	var records []GatewayRouteDB
	query := r.db.WithContext(ctx)
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	if err := query.Find(&records).Error; err != nil {
		return nil, err
	}

	results := make([]*GatewayRoute, len(records))
	for i, record := range records {
		results[i] = routeDBToDomain(&record)
	}
	return results, nil
}

// DeleteRoute deletes a gateway route by ID
func (r *Repository) DeleteRoute(ctx context.Context, id string) error {
	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
	delete(r.routeDBs, id)
		return nil
	}

	return r.db.WithContext(ctx).Delete(&GatewayRouteDB{}, "id = ?", id).Error
}

// SaveStats saves gateway statistics for a provider
func (r *Repository) SaveStats(ctx context.Context, provider string, stats *GatewayStats) error {
	record := statsDomainToDB(stats)

	if r.db == nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.statsDBs[record.Provider] = record
		return nil
	}

	return r.db.WithContext(ctx).Save(record).Error
}

// GetStats retrieves all gateway statistics
func (r *Repository) GetStats(ctx context.Context) (map[string]*GatewayStats, error) {
	if r.db == nil {
		r.mu.RLock()
		defer r.mu.RUnlock()
		results := make(map[string]*GatewayStats)
		for k, record := range r.statsDBs {
			results[k] = statsDBToDomain(record)
		}
		return results, nil
	}

	var records []GatewayStatsDB
	if err := r.db.WithContext(ctx).Find(&records).Error; err != nil {
		return nil, err
	}

	results := make(map[string]*GatewayStats)
	for i := range records {
		results[records[i].Provider] = statsDBToDomain(&records[i])
	}
	return results, nil
}
// Conversion helpers

func configDBToDomain(r *GatewayConfigDB) *GatewayConfig {
	return &GatewayConfig{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		Provider:    r.Provider,
		APIKey:      r.APIKey,
		BaseURL:     r.BaseURL,
		Models:      r.Models,
		RateLimit:   r.RateLimit,
		Timeout:     r.Timeout,
		RetryCount:  r.RetryCount,
		Priority:    r.Priority,
		Enabled:     r.Enabled,
		TenantID:    r.TenantID,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func configDomainToDB(c *GatewayConfig) *GatewayConfigDB {
	return &GatewayConfigDB{
		ID:          c.ID,
		Name:        c.Name,
		Description: c.Description,
		Provider:    c.Provider,
		APIKey:      c.APIKey,
		BaseURL:     c.BaseURL,
		Models:      c.Models,
		RateLimit:   c.RateLimit,
		Timeout:     c.Timeout,
		RetryCount:  c.RetryCount,
		Priority:    c.Priority,
		Enabled:     c.Enabled,
		TenantID:    c.TenantID,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func routeDBToDomain(r *GatewayRouteDB) *GatewayRoute {
	return &GatewayRoute{
		ID:        r.ID,
		Name:      r.Name,
		Pattern:   r.Pattern,
		ModelID:   r.ModelID,
		Fallbacks: r.Fallbacks,
		TenantID:  r.TenantID,
		Enabled:   r.Enabled,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func routeDomainToDB(r *GatewayRoute) *GatewayRouteDB {
	return &GatewayRouteDB{
		ID:        r.ID,
		Name:      r.Name,
		Pattern:   r.Pattern,
		ModelID:   r.ModelID,
		Fallbacks: r.Fallbacks,
		TenantID:  r.TenantID,
		Enabled:   r.Enabled,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

func statsDBToDomain(r *GatewayStatsDB) *GatewayStats {
	return &GatewayStats{
		Provider:       r.Provider,
		TotalRequests:  r.TotalRequests,
		SuccessCount:   r.SuccessCount,
		ErrorCount:     r.ErrorCount,
		AvgLatency:     r.AvgLatency,
		TotalTokens:    r.TotalTokens,
		TotalCost:      r.TotalCost,
		LastActiveTime: r.LastActiveTime,
	}
}

func statsDomainToDB(s *GatewayStats) *GatewayStatsDB {
	id := s.Provider // Use provider as ID for upsert simplicity
	return &GatewayStatsDB{
		ID:             id,
		Provider:       s.Provider,
		TotalRequests:  s.TotalRequests,
		SuccessCount:   s.SuccessCount,
		ErrorCount:     s.ErrorCount,
		AvgLatency:     s.AvgLatency,
		TotalTokens:    s.TotalTokens,
		TotalCost:      s.TotalCost,
		LastActiveTime: s.LastActiveTime,
	}
}
