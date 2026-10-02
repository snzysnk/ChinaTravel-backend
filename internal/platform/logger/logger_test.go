package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/snzysnk/ChinaTravel-backend/internal/platform/config"
)

// newTestLogger 构造一个写入内存缓冲的日志句柄，返回句柄与缓冲，
// 供各用例断言"哪些级别的日志真正被输出"。
func newTestLogger(level string) (*slog.Logger, *bytes.Buffer) {
	return newTestLoggerWithOptions(level, nil)
}

// newTestLoggerWithOptions 在 newTestLogger 基础上允许追加 handler 选项。
// 需要替换时钟一类不可直接断言的选项时使用。
func newTestLoggerWithOptions(level string, opts *slog.HandlerOptions) (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	cfg := config.Default()
	cfg.Log.Level = level

	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	// 级别始终以配置为准，避免调用方重复指定导致两处不一致。
	opts.Level = cfg.SlogLevel()

	return slog.New(slog.NewTextHandler(buf, opts)), buf
}

// TestLevelFiltersLowerSeverity 验证低于配置级别的日志不被输出。
// 这是"日志级别可配置"这条规格的最小验证：即使调用了 Debug，
// 在 info 级别下也不应出现在输出里。
func TestLevelFiltersLowerSeverity(t *testing.T) {
	log, buf := newTestLogger("info")

	log.Debug("调试信息不应出现")
	log.Info("普通信息应出现")

	out := buf.String()
	if strings.Contains(out, "调试信息不应出现") {
		t.Errorf("info 级别下 debug 日志被输出: %q", out)
	}
	if !strings.Contains(out, "普通信息应出现") {
		t.Errorf("info 级别下 info 日志未被输出: %q", out)
	}
}

// TestLevelAllowsHigherSeverity 验证达到或高于配置级别的日志被输出。
func TestLevelAllowsHigherSeverity(t *testing.T) {
	log, buf := newTestLogger("warn")

	log.Info("低于 warn 不应出现")
	log.Warn("warn 应出现")
	log.Error("error 应出现")

	out := buf.String()
	if strings.Contains(out, "低于 warn 不应出现") {
		t.Errorf("warn 级别下 info 日志被输出: %q", out)
	}
	if !strings.Contains(out, "warn 应出现") {
		t.Errorf("warn 级别下 warn 日志未被输出: %q", out)
	}
	if !strings.Contains(out, "error 应出现") {
		t.Errorf("warn 级别下 error 日志未被输出: %q", out)
	}
}

// TestDebugLevelOutputsEverything 验证 debug 级别下全部日志均被输出，
// 确认级别是"下限"而非"白名单"。
func TestDebugLevelOutputsEverything(t *testing.T) {
	log, buf := newTestLogger("debug")

	log.Debug("调试信息应出现")

	if !strings.Contains(buf.String(), "调试信息应出现") {
		t.Errorf("debug 级别下 debug 日志未被输出: %q", buf.String())
	}
}

// TestInvalidLevelFallsBackToInfo 验证非法级别不导致构造失败，
// 而是回落到 info——日志级别配错不应该让服务起不来。
func TestInvalidLevelFallsBackToInfo(t *testing.T) {
	log, buf := newTestLogger("verbose")

	log.Debug("不应出现")
	log.Info("应出现")

	out := buf.String()
	if strings.Contains(out, "不应出现") {
		t.Errorf("非法级别应回落到 info，但 debug 日志被输出了: %q", out)
	}
	if !strings.Contains(out, "应出现") {
		t.Errorf("非法级别应回落到 info，但 info 日志未被输出: %q", out)
	}
}

// TestOutputCarriesStructuredAttributes 验证日志以键值对携带上下文，
// 而非把上下文拼进消息字符串。
func TestOutputCarriesStructuredAttributes(t *testing.T) {
	log, buf := newTestLogger("info")

	log.Info("请求完成", "path", "/api/health", "status", 200)

	out := buf.String()
	for _, want := range []string{"path", "/api/health", "status", "200"} {
		if !strings.Contains(out, want) {
			t.Errorf("日志缺少结构化字段 %q: %q", want, out)
		}
	}
}

// TestEveryRecordCarriesTimestamp 验证每条日志都自带时间戳。
//
// 这里把 handler 的时钟替换成固定时间再断言，而不是搜 "time=" 字样——
// 后者只能证明"出现过一次时间"，无法证明每条记录都有。
func TestEveryRecordCarriesTimestamp(t *testing.T) {
	fixed := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	log, buf := newTestLoggerWithOptions("info", &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if attr.Key == slog.TimeKey && len(groups) == 0 {
				return slog.Time(slog.TimeKey, fixed)
			}
			return attr
		},
	})

	log.Info("第一条")
	log.Warn("第二条")

	lines := nonEmptyLines(buf.String())
	if len(lines) != 2 {
		t.Fatalf("日志行数 = %d, 期望 2: %q", len(lines), buf.String())
	}
	for i, line := range lines {
		if !strings.Contains(line, "2026-10-01T12:00:00") {
			t.Errorf("第 %d 条日志缺少时间戳: %q", i+1, line)
		}
	}
}

// nonEmptyLines 返回输出去掉空行后的行切片。
func nonEmptyLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
