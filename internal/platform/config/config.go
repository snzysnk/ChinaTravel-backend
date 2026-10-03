// Package config 负责加载服务运行配置。
//
// 配置来源有三层，优先级由低到高：
//  1. 内置默认值（本文件中的 default* 常量）；
//  2. 配置文件（默认 configs/config.yaml），提供与环境无关的默认值；
//  3. APP_ 前缀的环境变量，用于在不同环境覆盖同名配置项。
//
// 约定：密钥一律走环境变量，不写进 config.yaml；本包不做热重载，
// 配置在进程启动时读取一次。
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// 内置默认值。当配置文件缺失、或某一项在配置文件中未提供时使用。
const (
	// defaultPath 是配置文件的默认相对路径（相对进程工作目录）。
	defaultPath = "configs/config.yaml"

	// defaultServerPort 是 HTTP 监听端口的默认值。
	defaultServerPort = 8080

	// defaultShutdownTimeoutSeconds 是优雅关闭等待上限的默认秒数。
	// 取值刻意宽裕，避免正常的长请求在重启时被切断。
	defaultShutdownTimeoutSeconds = 10

	// defaultLogLevel 是日志级别的默认值。
	defaultLogLevel = "info"
)

// envPrefix 是所有可覆盖配置项的环境变量前缀。
const envPrefix = "APP_"

// Config 是服务的完整运行配置，字段与 configs/config.yaml 的结构一一对应。
type Config struct {
	// Server 承载 HTTP 服务自身的参数。
	Server ServerConfig `yaml:"server"`
	// CORS 承载跨域访问相关的白名单配置。
	CORS CORSConfig `yaml:"cors"`
	// Log 承载日志相关的参数。
	Log LogConfig `yaml:"log"`
}

// ServerConfig 是 HTTP 服务的参数。
type ServerConfig struct {
	// Port 是 HTTP 监听端口。
	Port int `yaml:"port"`
	// ShutdownTimeoutSeconds 是优雅关闭的等待上限（秒）。
	ShutdownTimeoutSeconds int `yaml:"shutdown_timeout_seconds"`
}

// CORSConfig 是跨域访问的参数。
type CORSConfig struct {
	// AllowOrigins 是允许跨域访问的来源白名单，元素为完整来源
	// （scheme + host + port），不允许写通配符。
	AllowOrigins []string `yaml:"allow_origins"`
}

// LogConfig 是日志参数。
type LogConfig struct {
	// Level 是日志级别，取值 debug / info / warn / error。
	Level string `yaml:"level"`
}

// Default 返回一份全部由内置默认值填充的配置。
// 它保证调用方即使完全不提供配置也能得到一个可用的对象。
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Port:                   defaultServerPort,
			ShutdownTimeoutSeconds: defaultShutdownTimeoutSeconds,
		},
		CORS: CORSConfig{
			// 默认放行本地前端开发服务器（与 Makefile 的 serve-frontend 端口一致）。
			AllowOrigins: []string{"http://localhost:5174"},
		},
		Log: LogConfig{
			Level: defaultLogLevel,
		},
	}
}

// Load 按「内置默认值 < 配置文件 < 环境变量」的顺序装配配置。
//
// path 为配置文件路径，传空字符串时使用 defaultPath。
// 配置文件不存在不算错误：此时回落到内置默认值与环境变量，
// 以满足"配置来源全部缺失时服务仍可正常启动"的要求；
// 但配置文件存在却格式非法时会返回错误，避免把写错的配置静默忽略掉。
func Load(path string) (*Config, error) {
	if path == "" {
		path = defaultPath
	}

	cfg := Default()

	if err := applyFile(cfg, path); err != nil {
		return nil, err
	}
	if err := applyEnv(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// applyFile 读取 YAML 配置文件并覆盖 cfg 中对应字段。
// 文件不存在时直接返回 nil（视为"未提供该层配置"）。
func applyFile(cfg *Config, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
	}

	// 直接反序列化到已有对象上：未在文件中出现的字段保持内置默认值，
	// 这样"文件里写多少就覆盖多少"，不必为省略项再补一次默认值。
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
	}
	return nil
}

// applyEnv 用 APP_ 前缀的环境变量覆盖 cfg 中已存在的项。
//
// 变量名规则：APP_ + 配置路径的大写下划线形式，例如
//   - APP_SERVER_PORT              -> server.port
//   - APP_LOG_LEVEL                -> log.level
//   - APP_CORS_ALLOW_ORIGINS       -> cors.allow_origins
//
// 数组型配置以英文逗号分隔，例如 APP_CORS_ALLOW_ORIGINS=a,b。
// 未设置的环境变量不覆盖任何内容；设置但取值非法时返回错误而不是静默忽略，
// 避免"以为生效了其实没有"。
func applyEnv(cfg *Config) error {
	if v, ok := lookupEnv("SERVER_PORT"); ok {
		port, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s%s 不是合法端口: %q", envPrefix, "SERVER_PORT", v)
		}
		cfg.Server.Port = port
	}

	if v, ok := lookupEnv("SERVER_SHUTDOWN_TIMEOUT_SECONDS"); ok {
		seconds, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("%s%s 不是合法秒数: %q", envPrefix, "SERVER_SHUTDOWN_TIMEOUT_SECONDS", v)
		}
		cfg.Server.ShutdownTimeoutSeconds = seconds
	}

	if v, ok := lookupEnv("CORS_ALLOW_ORIGINS"); ok {
		cfg.CORS.AllowOrigins = splitAndTrim(v)
	}

	if v, ok := lookupEnv("LOG_LEVEL"); ok {
		cfg.Log.Level = strings.TrimSpace(v)
	}

	return nil
}

// lookupEnv 读取 APP_ 前缀的环境变量，返回去掉前缀后的取值。
// 变量未设置、或设置为空白字符串时返回 false（视为未覆盖）。
func lookupEnv(name string) (string, bool) {
	v, ok := os.LookupEnv(envPrefix + name)
	if !ok || strings.TrimSpace(v) == "" {
		return "", false
	}
	return v, true
}

// splitAndTrim 按英文逗号切分数组型配置取值，并去掉每一项的首尾空白。
// 全部为空白项时返回空切片（表示"显式清空白名单"），而非 nil。
func splitAndTrim(v string) []string {
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// SlogLevel 把配置中的日志级别字符串转换为 slog.Level。
// 取值非法时不报错，而是回落到 slog.LevelInfo 并留下一条告警日志——
// 日志级别配错不应该导致服务起不来。
func (c *Config) SlogLevel() slog.Level {
	switch strings.ToLower(strings.TrimSpace(c.Log.Level)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		slog.Warn("无法识别的日志级别，回落到 info", "level", c.Log.Level)
		return slog.LevelInfo
	}
}
