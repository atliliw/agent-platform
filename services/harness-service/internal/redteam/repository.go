// Package redteam provides data access for red team tests, attacks, and reports
package redteam

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository provides database persistence for red team data
type Repository struct {
	db *gorm.DB
}

// NewRepository creates a new red team repository
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// AutoMigrate runs database migrations for red team tables
func (r *Repository) AutoMigrate() error {
	return r.db.AutoMigrate(&RedTeamTest{}, &RedTeamAttack{}, &RedTeamReport{})
}

// CreateTest persists a new red team test
func (r *Repository) CreateTest(ctx context.Context, test *RedTeamTest) error {
	if test.ID == "" {
		test.ID = uuid.New().String()
	}
	return r.db.WithContext(ctx).Create(test).Error
}

// GetTest retrieves a test by ID
func (r *Repository) GetTest(ctx context.Context, id string) (*RedTeamTest, error) {
	var test RedTeamTest
	if err := r.db.WithContext(ctx).First(&test, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("test not found: %s: %w", id, err)
	}
	return &test, nil
}

// ListTests lists tests with optional filters
func (r *Repository) ListTests(ctx context.Context, agentID, status, tenantID string) ([]*RedTeamTest, error) {
	query := r.db.WithContext(ctx).Order("created_at DESC")
	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}

	var tests []RedTeamTest
	if err := query.Find(&tests).Error; err != nil {
		return nil, err
	}

	results := make([]*RedTeamTest, len(tests))
	for i := range tests {
		results[i] = &tests[i]
	}
	return results, nil
}

// UpdateTest updates an existing test
func (r *Repository) UpdateTest(ctx context.Context, test *RedTeamTest) error {
	return r.db.WithContext(ctx).Save(test).Error
}

// DeleteTest deletes a test and its associated attacks and reports
func (r *Repository) DeleteTest(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("test_id = ?", id).Delete(&RedTeamAttack{}).Error; err != nil {
			return fmt.Errorf("delete attacks for test %s: %w", id, err)
		}
		if err := tx.Where("test_id = ?", id).Delete(&RedTeamReport{}).Error; err != nil {
			return fmt.Errorf("delete reports for test %s: %w", id, err)
		}
		if err := tx.Delete(&RedTeamTest{}, "id = ?", id).Error; err != nil {
			return fmt.Errorf("delete test %s: %w", id, err)
		}
		return nil
	})
}

// CreateAttack persists a new attack
func (r *Repository) CreateAttack(ctx context.Context, attack *RedTeamAttack) error {
	if attack.ID == "" {
		attack.ID = uuid.New().String()
	}
	return r.db.WithContext(ctx).Create(attack).Error
}

// GetAttacks retrieves all attacks for a test
func (r *Repository) GetAttacks(ctx context.Context, testID string) ([]*RedTeamAttack, error) {
	var attacks []RedTeamAttack
	if err := r.db.WithContext(ctx).Where("test_id = ?", testID).Order("timestamp ASC").Find(&attacks).Error; err != nil {
		return nil, err
	}

	results := make([]*RedTeamAttack, len(attacks))
	for i := range attacks {
		results[i] = &attacks[i]
	}
	return results, nil
}

// CreateReport persists a new report
func (r *Repository) CreateReport(ctx context.Context, report *RedTeamReport) error {
	if report.ID == "" {
		report.ID = uuid.New().String()
	}
	return r.db.WithContext(ctx).Create(report).Error
}

// GetReport retrieves a report by ID
func (r *Repository) GetReport(ctx context.Context, id string) (*RedTeamReport, error) {
	var report RedTeamReport
	if err := r.db.WithContext(ctx).First(&report, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("report not found: %s: %w", id, err)
	}
	return &report, nil
}

// GetReportByTest retrieves a report by test ID
func (r *Repository) GetReportByTest(ctx context.Context, testID string) (*RedTeamReport, error) {
	var report RedTeamReport
	if err := r.db.WithContext(ctx).Where("test_id = ?", testID).First(&report).Error; err != nil {
		return nil, fmt.Errorf("report not found for test: %s: %w", testID, err)
	}
	return &report, nil
}

// ListAllTests loads all tests from the database (used for cache warm-up)
func (r *Repository) ListAllTests(ctx context.Context) ([]*RedTeamTest, error) {
	var tests []RedTeamTest
	if err := r.db.WithContext(ctx).Find(&tests).Error; err != nil {
		return nil, err
	}
	results := make([]*RedTeamTest, len(tests))
	for i := range tests {
		results[i] = &tests[i]
	}
	return results, nil
}

// ListAllAttacks loads all attacks from the database (used for cache warm-up)
func (r *Repository) ListAllAttacks(ctx context.Context) ([]*RedTeamAttack, error) {
	var attacks []RedTeamAttack
	if err := r.db.WithContext(ctx).Find(&attacks).Error; err != nil {
		return nil, err
	}
	results := make([]*RedTeamAttack, len(attacks))
	for i := range attacks {
		results[i] = &attacks[i]
	}
	return results, nil
}

// ListAllReports loads all reports from the database (used for cache warm-up)
func (r *Repository) ListAllReports(ctx context.Context) ([]*RedTeamReport, error) {
	var reports []RedTeamReport
	if err := r.db.WithContext(ctx).Find(&reports).Error; err != nil {
		return nil, err
	}
	results := make([]*RedTeamReport, len(reports))
	for i := range reports {
		results[i] = &reports[i]
	}
	return results, nil
}
