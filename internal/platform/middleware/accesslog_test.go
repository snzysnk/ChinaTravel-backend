package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newLogBuffer 构造一个写入内存缓冲的日志句柄与缓冲本身。
func newLogBuffer() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	handler := slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(handler), buf
}

// TestAccessLogEmitsExactlyOneLinePerRequest 验证一个请求恰好产生一条访问日志。
// "恰好一条"是规格里的硬要求：多一条就意味着框架默认日志混了进来。
func TestAccessLogEmitsExactlyOneLinePerRequest(t *testing.T) {
	log, buf := newLogBuffer()

	r := gin.New()
	r.Use(AccessLog(log))
	r.GET("/api/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	lines := nonEmptyLines(buf.String())
	if len(lines) != 1 {
		t.Fatalf("访问日志条数 = %d, 期望 1, 实际输出:\n%s", len(lines), buf.String())
	}
}

// TestAccessLogCarriesRequiredFields 验证访问日志包含规格要求的五个字段。
func TestAccessLogCarriesRequiredFields(t *testing.T) {
	log, buf := newLogBuffer()

	r := gin.New()
	r.Use(AccessLog(log))
	r.POST("/api/destinations/1", func(c *gin.Context) {
		c.String(http.StatusCreated, "ok")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/destinations/1", nil)
	req.RemoteAddr = "203.0.113.7:54321"
	r.ServeHTTP(rec, req)

	out := buf.String()
	for _, want := range []string{
		FieldMethod, http.MethodPost,
		FieldPath, "/api/destinations/1",
		FieldStatus, "201",
		FieldDurationMS,
		FieldClientIP, "203.0.113.7",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("访问日志缺少 %q, 实际输出: %s", want, out)
		}
	}
}

// TestAccessLogRecordsFailedRequestOnce 验证处理函数返回失败状态时，
// 依然只有一条日志——耗时与状态码是合并记录的，不因分支而分裂。
func TestAccessLogRecordsFailedRequestOnce(t *testing.T) {
	log, buf := newLogBuffer()

	r := gin.New()
	r.Use(AccessLog(log))
	r.GET("/api/missing", func(c *gin.Context) {
		c.String(http.StatusNotFound, "nope")
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/missing", nil))

	lines := nonEmptyLines(buf.String())
	if len(lines) != 1 {
		t.Fatalf("访问日志条数 = %d, 期望 1, 实际输出:\n%s", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], "404") {
		t.Errorf("访问日志未记录真实状态码: %s", lines[0])
	}
}

// TestAccessLogIsAppliedAcrossRequests 验证中间件对每个请求都生效，
// 而不是只在第一个请求上记录。
func TestAccessLogIsAppliedAcrossRequests(t *testing.T) {
	log, buf := newLogBuffer()

	r := gin.New()
	r.Use(AccessLog(log))
	r.GET("/api/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	for i := 0; i < 3; i++ {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/health", nil))
	}

	if lines := nonEmptyLines(buf.String()); len(lines) != 3 {
		t.Fatalf("三次请求的访问日志条数 = %d, 期望 3, 实际输出:\n%s", len(lines), buf.String())
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
