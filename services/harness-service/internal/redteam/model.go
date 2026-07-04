// Package redteam provides red team security testing for AI agents
package redteam

import (
	"time"
)

// RedTeamTest represents a red team security test
type RedTeamTest struct {
	ID          string     `gorm:"primaryKey"`
	Name        string
	Description string
	AgentID     string     `gorm:"index"`
	Model       string
	Category    string     // prompt_injection, jailbreak, data_leak, harmful_content, all
	Status      string     `gorm:"index"` // pending, running, completed, failed
	Config      string     `gorm:"type:text"` // JSON configuration
	StartTime   *time.Time
	EndTime     *time.Time
	TenantID    string     `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// RedTeamAttack represents a single attack attempt
type RedTeamAttack struct {
	ID         string    `gorm:"primaryKey"`
	TestID     string    `gorm:"index"`
	AttackType string    `gorm:"index"` // prompt_injection, jailbreak, data_leak, harmful_content
	AttackName string
	Payload    string    `gorm:"type:text"`
	Expected   string    // Expected behavior description
	Actual     string    `gorm:"type:text"` // Actual response from agent
	Passed     bool      // true if attack was successfully blocked/mitigated
	Severity   string    // critical, high, medium, low
	Confidence float64   // 0-1 confidence score
	Duration   int64     // Duration in milliseconds
	Tokens     int64     // Tokens used
	Cost       float64   // Cost of the attack
	Timestamp  time.Time
}

// RedTeamReport represents the final security assessment report
type RedTeamReport struct {
	ID              string  `gorm:"primaryKey"`
	TestID          string  `gorm:"index"`
	TotalAttacks    int
	PassedAttacks   int     // Successfully blocked
	FailedAttacks   int     // Vulnerabilities found
	BlockedAttacks  int     // Blocked by guardrails
	CriticalCount   int
	HighCount       int
	MediumCount     int
	LowCount        int
	RiskScore       float64 // 0-100 risk score
	SecurityLevel   string  // excellent, good, moderate, poor, critical
	Vulnerabilities string  `gorm:"type:text"` // JSON array of vulnerabilities
	Recommendations string  `gorm:"type:text"` // JSON array of recommendations
	GeneratedAt     time.Time
}

// AttackPayload represents a single attack payload definition
type AttackPayload struct {
	ID          string
	Type        string   // prompt_injection, jailbreak, data_leak, harmful_content
	Name        string
	Description string
	Payload     string
	Expected    string   // Expected agent behavior
	Severity    string   // critical, high, medium, low
	Tags        []string
}

// TestConfig represents configuration for a red team test
type TestConfig struct {
	Categories     []string `json:"categories"`
	MaxAttacks     int      `json:"max_attacks"`
	Timeout        int      `json:"timeout_seconds"`
	Parallel       bool     `json:"parallel"`
	StopOnCritical bool     `json:"stop_on_critical"`
}

// Vulnerability represents a discovered vulnerability
type Vulnerability struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Severity    string   `json:"severity"`
	Description string   `json:"description"`
	Payload     string   `json:"payload"`
	Response    string   `json:"response"`
	Remediation string   `json:"remediation"`
	CVE         string   `json:"cve,omitempty"`
	References  []string `json:"references,omitempty"`
}

// Recommendation represents a security recommendation
type Recommendation struct {
	Priority    string   `json:"priority"`
	Category    string   `json:"category"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Actions     []string `json:"actions"`
}

// AttackResult represents the result of a single attack
type AttackResult struct {
	Attack    *RedTeamAttack
	Vuln      *Vulnerability
	Error     error
	Duration  int64
}
