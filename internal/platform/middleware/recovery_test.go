package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRecoveryReturnsParseableEnvelope 验证 panic 后仍返回可被前端解析的信封，
// 且业务码落在 500xx 分段——这是"前端统一 JSON.parse"能否成立的关键。
func TestRecoveryReturnsParseableEnvelope(t *testing.T) {
	log, buf := newLogBuffer()

	r := gin.New()
	r.Use(Recovery(log))
	r.GET("/api/boom", func(c *gin.Context) { panic("模拟的业务 panic") })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/boom", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("HTTP 状态码 = %d, 期望 200（信封契约下状态码恒为 200）", rec.Code)
	}

	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("panic 后的响应体不是合法信封: %v, body=%q", err, rec.Body.String())
	}
	if envelope.Code < 50000 || envelope.Code > 50099 {
		t.Errorf("业务码 = %d, 期望落在 500xx 分段", envelope.Code)
	}
	if envelope.Msg == "" {
		t.Error("panic 响应应带有提示文案")
	}
	if string(envelope.Data) != "null" {
		t.Errorf("panic 响应 data = %s, 期望 null", envelope.Data)
	}
	if buf.Len() == 0 {
		t.Error("panic 应被记录为日志")
	}
}

// TestRecoveryLogsPanicAndPath 验证日志中包含 panic 信息与请求路径。
func TestRecoveryLogsPanicAndPath(t *testing.T) {
	log, buf := newLogBuffer()

	r := gin.New()
	r.Use(Recovery(log))
	r.GET("/api/destinations", func(c *gin.Context) { panic("仓储炸了") })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/destinations", nil))

	out := buf.String()
	for _, want := range []string{RecoveryMsg, FieldPath, "/api/destinations", "仓储炸了"} {
		if !strings.Contains(out, want) {
			t.Errorf("panic 日志缺少 %q, 实际输出: %s", want, out)
		}
	}
}

// TestRecoveryDoesNotAffectNormalRequests 验证没有 panic 时不写任何响应、
// 不打断正常流程，避免恢复中间件"顺手"改掉正常响应。
func TestRecoveryDoesNotAffectNormalRequests(t *testing.T) {
	log, buf := newLogBuffer()

	r := gin.New()
	r.Use(Recovery(log))
	r.GET("/api/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Body.String() != "ok" {
		t.Errorf("正常响应被恢复中间件改动: %q", rec.Body.String())
	}
	if buf.Len() != 0 {
		t.Errorf("正常请求不应产生 panic 日志: %s", buf.String())
	}
}
