// Package config provides configuration loading
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the main configuration
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	LLM        LLMConfig        `yaml:"llm"`
	Qdrant     QdrantConfig     `yaml:"qdrant"`
	MongoDB    MongoDBConfig    `yaml:"mongodb"`
	Redis      RedisConfig      `yaml:"redis"`
	Services   ServicesConfig   `yaml:"services"`
	Logging    LoggingConfig    `yaml:"logging"`
	Tools      ToolsConfig      `yaml:"tools"`
	Forgetting ForgettingConfig `yaml:"forgetting"`
	Engine     EngineConfig     `yaml:"engine"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	GRPCPort int    `yaml:"grpc_port"`
	HttpPort int    `yaml:"http_port"`
	Host     string `yaml:"host"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	SQLite SQLiteConfig `yaml:"sqlite"`
}

// SQLiteConfig holds SQLite configuration
type SQLiteConfig struct {
	Path string `yaml:"path"`
}

// LLMConfig holds LLM configuration
type LLMConfig struct {
	Provider       string            `yaml:"provider"`
	APIKey         string            `yaml:"api_key"`
	BaseURL        string            `yaml:"base_url"`
	Model          string            `yaml:"model"`
	EmbeddingModel string            `yaml:"embedding_model"`
	MaxTokens      int               `yaml:"max_tokens"`
	Models         map[string]string `yaml:"models"`
	Compression    CompressionConfig `yaml:"compression"`
}

// CompressionConfig holds context-compression settings for the LLM client.
// Compression is enabled by default (Disable=false); set disable: true to turn
// it off. Zero thresholds are filled with sane defaults at Load time, so an
// absent compression block in YAML yields the default behavior.
type CompressionConfig struct {
	Disable        bool `yaml:"disable"`
	MaxSystemChars int  `yaml:"max_system_chars"`
	MaxRecentChars int  `yaml:"max_recent_chars"`
	MaxOldChars    int  `yaml:"max_old_chars"`
	RecentCount    int  `yaml:"recent_count"`
}

// QdrantConfig holds Qdrant configuration
type QdrantConfig struct {
	URL        string `yaml:"url"`
	Collection string `yaml:"collection"`
	APIKey     string `yaml:"api_key"`
}

// MongoDBConfig holds MongoDB configuration
type MongoDBConfig struct {
	URL      string `yaml:"url"`
	Database string `yaml:"database"`
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	URL string `yaml:"url"`
}

// ServicesConfig holds services addresses
type ServicesConfig struct {
	Gateway   string `yaml:"gateway"`
	Chat      string `yaml:"chat"`
	Knowledge string `yaml:"knowledge"`
	Memory    string `yaml:"memory"`
	A2A       string `yaml:"a2a"`
	MCP       string `yaml:"mcp"`
	Harness   string `yaml:"harness"`
	Agent     string `yaml:"agent"` // Agent orchestration service
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// ToolsConfig holds external tool API configuration
type ToolsConfig struct {
	WebSearch WebSearchToolConfig `yaml:"web_search"`
	Weather   WeatherToolConfig   `yaml:"weather"`
	Browser   BrowserToolConfig   `yaml:"browser"`
}

// WebSearchToolConfig holds web search tool configuration
type WebSearchToolConfig struct {
	APIKey   string `yaml:"api_key"`
	Provider string `yaml:"provider"` // serpapi, bing
}

// WeatherToolConfig holds weather tool configuration
type WeatherToolConfig struct {
	APIKey   string `yaml:"api_key"`
	Provider string `yaml:"provider"` // openweathermap, qweather
}

// BrowserToolConfig holds browser automation tool configuration
type BrowserToolConfig struct {
	APIKey  string `yaml:"api_key"`  // LLM API Key (OpenAI/DashScope)
	BaseURL string `yaml:"base_url"` // LLM API Base URL
	Model   string `yaml:"model"`    // LLM Model name
}

// ForgettingConfig holds memory forgetting configuration
type ForgettingConfig struct {
	TimeDecayRate        float64 `yaml:"time_decay_rate"`
	ImportanceThreshold  float64 `yaml:"importance_threshold"`
	MaxAgeHours          int     `yaml:"max_age_hours"`
	CleanupIntervalHours int     `yaml:"cleanup_interval_hours"`
}

// EngineConfig holds agent engine configuration
type EngineConfig struct {
	MaxSteps         int `yaml:"max_steps"`
	MaxHistoryLength int `yaml:"max_history_length"`
}

// Load loads configuration from file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Fill compression defaults (enabled by default; zero thresholds -> sane values)
	cfg.applyCompressionDefaults()

	return &cfg, nil
}

// applyCompressionDefaults fills zero thresholds with sane defaults. Disable is
// left as-is so the feature is on by default (zero-value false = enabled) and
// users opt out with disable: true.
func (c *Config) applyCompressionDefaults() {
	comp := &c.LLM.Compression
	if comp.MaxSystemChars <= 0 {
		comp.MaxSystemChars = 12000
	}
	if comp.MaxRecentChars <= 0 {
		comp.MaxRecentChars = 6000
	}
	if comp.MaxOldChars <= 0 {
		comp.MaxOldChars = 1000
	}
	if comp.RecentCount <= 0 {
		comp.RecentCount = 8
	}
}

// LoadDefault loads default configuration
func LoadDefault() *Config {
	return &Config{
		Server: ServerConfig{
			GRPCPort: 50001,
			HttpPort: 8080,
			Host:     "localhost",
		},
		Database: DatabaseConfig{
			SQLite: SQLiteConfig{
				Path: "./data/app.db",
			},
		},
		LLM: LLMConfig{
			Provider:  "openai",
			Model:     "gpt-4",
			MaxTokens: 4096,
			Compression: CompressionConfig{
				MaxSystemChars: 12000,
				MaxRecentChars: 6000,
				MaxOldChars:    1000,
				RecentCount:    8,
			},
		},
		Qdrant: QdrantConfig{
			URL:        "http://localhost:6333",
			Collection: "documents",
		},
		MongoDB: MongoDBConfig{
			URL:      "mongodb://localhost:27017",
			Database: "agent_platform",
		},
		Redis: RedisConfig{
			URL: "redis://localhost:6379",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.GRPCPort == 0 {
		return fmt.Errorf("server.grpc_port is required")
	}
	if c.LLM.Provider == "" {
		return fmt.Errorf("llm.provider is required")
	}
	return nil
}
