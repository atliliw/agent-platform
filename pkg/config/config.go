// Package config provides configuration loading
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the main configuration
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	LLM       LLMConfig       `yaml:"llm"`
	Qdrant    QdrantConfig    `yaml:"qdrant"`
	MongoDB   MongoDBConfig   `yaml:"mongodb"`
	Redis     RedisConfig     `yaml:"redis"`
	Services  ServicesConfig  `yaml:"services"`
	Logging   LoggingConfig   `yaml:"logging"`
	Tools     ToolsConfig     `yaml:"tools"`
	Forgetting ForgettingConfig `yaml:"forgetting"`
	Engine    EngineConfig    `yaml:"engine"`
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
	Provider      string            `yaml:"provider"`
	APIKey        string            `yaml:"api_key"`
	BaseURL       string            `yaml:"base_url"`
	Model         string            `yaml:"model"`
	EmbeddingModel string           `yaml:"embedding_model"`
	MaxTokens     int               `yaml:"max_tokens"`
	Models        map[string]string `yaml:"models"`
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

	// Apply environment variable overrides
	cfg.applyEnvOverrides()

	return &cfg, nil
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

func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		c.LLM.APIKey = v
	}
	if v := os.Getenv("LLM_PROVIDER"); v != "" {
		c.LLM.Provider = v
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		c.LLM.Model = v
	}
	if v := os.Getenv("QDRANT_URL"); v != "" {
		c.Qdrant.URL = v
	}
	if v := os.Getenv("MONGODB_URL"); v != "" {
		c.MongoDB.URL = v
	}
	if v := os.Getenv("REDIS_URL"); v != "" {
		c.Redis.URL = v
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