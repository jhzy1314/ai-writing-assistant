package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config 后端全局配置，来自 configs/config.yaml
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Quotas     QuotasConfig     `mapstructure:"quotas"`
	Models     []ModelSeed      `mapstructure:"models"`
	RoleModels map[string][]string `mapstructure:"role_models"`
}

// ServerConfig 服务与数据库配置
type ServerConfig struct {
	ListenAddr string `mapstructure:"listen_addr"`
	Port       int    `mapstructure:"port"`
	SQLitePath string `mapstructure:"sqlite_path"`
}

// QuotasConfig 调用限制与成本控制参数（同时种子入库 configs 表）
type QuotasConfig struct {
	DailyCallLimit       int     `mapstructure:"daily_call_limit"`
	DailyTokenLimit      int     `mapstructure:"daily_token_limit"`
	PerRequestTokenLimit int     `mapstructure:"per_request_token_limit"`
	LightInputCharLimit  int     `mapstructure:"light_input_char_limit"`
	MaxIterations        int     `mapstructure:"max_iterations"`
	RateLimitPerMinute   int     `mapstructure:"rate_limit_per_minute"`
	MaxConcurrent        int     `mapstructure:"max_concurrent"`
	WarnRatio            float64 `mapstructure:"warn_ratio"`
}

// ModelSeed 模型种子配置（启动时写入 models 表）
type ModelSeed struct {
	Name        string `mapstructure:"name"`
	Vendor      string `mapstructure:"vendor"`
	APIEndpoint string `mapstructure:"api_endpoint"`
	APIKey      string `mapstructure:"api_key"`
	Status      string `mapstructure:"status"`
	DailyLimit  int    `mapstructure:"daily_limit"`
}

// LoadConfig 从给定目录读取 config.yaml，支持环境变量覆盖 API 密钥
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(configPath)
	v.AddConfigPath(".")
	v.SetDefault("server.port", 8081)
	v.SetDefault("server.listen_addr", "0.0.0.0")
	v.SetDefault("server.sqlite_path", "data/ai-novel.db")
	v.SetDefault("quotas.daily_call_limit", 500)
	v.SetDefault("quotas.daily_token_limit", 2000000)
	v.SetDefault("quotas.per_request_token_limit", 8000)
	v.SetDefault("quotas.light_input_char_limit", 500)
	v.SetDefault("quotas.max_iterations", 3)
	v.SetDefault("quotas.rate_limit_per_minute", 20)
	v.SetDefault("quotas.max_concurrent", 5)
	v.SetDefault("quotas.warn_ratio", 0.8)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	// 环境变量覆盖：AI_NOVEL_MODEL_X_APIKEY 优先级高于 config.yaml
	for i, m := range cfg.Models {
		envKey := fmt.Sprintf("AI_NOVEL_MODEL_%s_APIKEY", strings.ToUpper(strings.ReplaceAll(m.Name, "-", "_")))
		if v := os.Getenv(envKey); v != "" { cfg.Models[i].APIKey = v }
	}
	if cfg.RoleModels == nil {
		cfg.RoleModels = map[string][]string{}
	}
	return &cfg, nil
}
