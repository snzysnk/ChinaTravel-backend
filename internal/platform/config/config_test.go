package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeTempConfig 在临时目录写出一份配置文件，返回其路径。
// 用临时文件而非仓库内固定路径，避免测试之间互相污染。
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写入临时配置文件失败: %v", err)
	}
	return path
}

// TestLoadFromFileOnly 覆盖「仅 YAML」场景：
// 全部取值都应来自配置文件，而不是内置默认值。
func TestLoadFromFileOnly(t *testing.T) {
	path := writeTempConfig(t, `
server:
  port: 9091
  shutdown_timeout_seconds: 3
cors:
  allow_origins:
    - http://localhost:3001
log:
  level: warn
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load 返回错误: %v", err)
	}

	if cfg.Server.Port != 9091 {
		t.Errorf("server.port = %d, 期望 9091", cfg.Server.Port)
	}
	if cfg.Server.ShutdownTimeoutSeconds != 3 {
		t.Errorf("server.shutdown_timeout_seconds = %d, 期望 3", cfg.Server.ShutdownTimeoutSeconds)
	}
	if want := []string{"http://localhost:3001"}; !reflect.DeepEqual(cfg.CORS.AllowOrigins, want) {
		t.Errorf("cors.allow_origins = %v, 期望 %v", cfg.CORS.AllowOrigins, want)
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("log.level = %q, 期望 warn", cfg.Log.Level)
	}
}

// TestLoadFilePartialKeepsDefaults 覆盖「YAML 只写了一部分」场景：
// 文件中省略的项应保持内置默认值，而不是被零值覆盖。
func TestLoadFilePartialKeepsDefaults(t *testing.T) {
	path := writeTempConfig(t, `
server:
  port: 9092
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load 返回错误: %v", err)
	}

	if cfg.Server.Port != 9092 {
		t.Errorf("server.port = %d, 期望 9092", cfg.Server.Port)
	}
	if cfg.Server.ShutdownTimeoutSeconds != defaultShutdownTimeoutSeconds {
		t.Errorf("省略项未被默认值填充: shutdown_timeout_seconds = %d", cfg.Server.ShutdownTimeoutSeconds)
	}
	if cfg.Log.Level != defaultLogLevel {
		t.Errorf("省略项未被默认值填充: log.level = %q", cfg.Log.Level)
	}
}

// TestLoadEnvOverridesFile 覆盖「YAML 被环境变量覆盖」场景，
// 同时验证数组型配置的逗号分隔写法。
func TestLoadEnvOverridesFile(t *testing.T) {
	path := writeTempConfig(t, `
server:
  port: 8080
  shutdown_timeout_seconds: 10
cors:
  allow_origins:
    - http://localhost:5174
log:
  level: info
`)

	t.Setenv("APP_SERVER_PORT", "9090")
	t.Setenv("APP_SERVER_SHUTDOWN_TIMEOUT_SECONDS", "5")
	t.Setenv("APP_CORS_ALLOW_ORIGINS", "http://localhost:3001, https://example.com")
	t.Setenv("APP_LOG_LEVEL", "debug")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load 返回错误: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("环境变量未覆盖端口: %d, 期望 9090", cfg.Server.Port)
	}
	if cfg.Server.ShutdownTimeoutSeconds != 5 {
		t.Errorf("环境变量未覆盖关闭超时: %d, 期望 5", cfg.Server.ShutdownTimeoutSeconds)
	}
	if want := []string{"http://localhost:3001", "https://example.com"}; !reflect.DeepEqual(cfg.CORS.AllowOrigins, want) {
		t.Errorf("数组型配置未按逗号切分: %v, 期望 %v", cfg.CORS.AllowOrigins, want)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("环境变量未覆盖日志级别: %q, 期望 debug", cfg.Log.Level)
	}
}

// TestLoadBothMissingFallsBackToDefaults 覆盖「两者皆缺时回落到默认值」场景。
func TestLoadBothMissingFallsBackToDefaults(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "not-exists.yaml")

	cfg, err := Load(missingPath)
	if err != nil {
		t.Fatalf("配置文件缺失时不应报错，实际: %v", err)
	}

	if cfg.Server.Port != defaultServerPort {
		t.Errorf("server.port = %d, 期望默认值 %d", cfg.Server.Port, defaultServerPort)
	}
	if cfg.Server.ShutdownTimeoutSeconds != defaultShutdownTimeoutSeconds {
		t.Errorf("shutdown_timeout_seconds = %d, 期望默认值 %d",
			cfg.Server.ShutdownTimeoutSeconds, defaultShutdownTimeoutSeconds)
	}
	if cfg.Log.Level != defaultLogLevel {
		t.Errorf("log.level = %q, 期望默认值 %q", cfg.Log.Level, defaultLogLevel)
	}
	if len(cfg.CORS.AllowOrigins) == 0 {
		t.Error("cors.allow_origins 未回落到默认白名单")
	}
}

// TestLoadInvalidFileFails 验证配置文件格式非法时明确报错，
// 而不是静默忽略掉写错的配置。
func TestLoadInvalidFileFails(t *testing.T) {
	path := writeTempConfig(t, "server: [this is not valid")

	if _, err := Load(path); err == nil {
		t.Fatal("配置文件格式非法时应返回错误，实际返回 nil")
	}
}

// TestLoadInvalidEnvFails 验证环境变量取值非法时明确报错。
func TestLoadInvalidEnvFails(t *testing.T) {
	t.Setenv("APP_SERVER_PORT", "not-a-number")

	if _, err := Load(""); err == nil {
		t.Fatal("APP_SERVER_PORT 非数字时应返回错误，实际返回 nil")
	}
}

// TestLoadEmptyPathUsesDefaultPath 验证空路径时使用约定的默认配置文件。
// 该用例不依赖仓库内文件是否存在，只断言两种情形都不报错。
func TestLoadEmptyPathUsesDefaultPath(t *testing.T) {
	if _, err := Load(""); err != nil {
		t.Fatalf("使用默认路径加载不应报错: %v", err)
	}
}
