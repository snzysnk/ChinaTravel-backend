package bootstrap

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/snzysnk/ChinaTravel-backend/internal/platform/config"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// envelope 是测试侧使用的信封视图。
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// buildForTest 装配一个写入内存缓冲的应用，返回处理器与日志缓冲。
func buildForTest(t *testing.T) (*gin.Engine, *bytes.Buffer) {
	t.Helper()

	buf := &bytes.Buffer{}
	cfg := config.Default()
	cfg.CORS.AllowOrigins = []string{"http://localhost:5173"}

	return Build(cfg, buf), buf
}

// TestBuildServesHealthWithoutRealPort 验证装配结果可以直接被 httptest 调用，
// 全程不占用任何 TCP 端口——这是"装配模块 SHALL 返回可测试的 HTTP 处理器"的
// 直接体现。
func TestBuildServesHealthWithoutRealPort(t *testing.T) {
	r, _ := buildForTest(t)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP 状态码 = %d, 期望 200", rec.Code)
	}

	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("响应体不是合法信封: %v, body=%q", err, rec.Body.String())
	}
	if env.Code != 0 {
		t.Errorf("code = %d, 期望 0", env.Code)
	}
}

// TestBuildServesDestinationList 验证装配后示例领域链路可用。
func TestBuildServesDestinationList(t *testing.T) {
	r, _ := buildForTest(t)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/destinations", nil))

	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("响应体不是合法信封: %v, body=%q", err, rec.Body.String())
	}

	var items []map[string]any
	if err := json.Unmarshal(env.Data, &items); err != nil {
		t.Fatalf("data 不是数组: %v", err)
	}
	if len(items) == 0 {
		t.Error("列表接口返回值不应为空")
	}
}

// TestBuildProducesOneAccessLogPerRequest 验证经由完整装配后，
// 每个请求产生恰好一条统一格式的访问日志（不含框架默认日志）。
func TestBuildProducesOneAccessLogPerRequest(t *testing.T) {
	r, buf := buildForTest(t)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	lines := 0
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.TrimSpace(line) != "" {
			lines++
		}
	}
	if lines != 1 {
		t.Fatalf("日志行数 = %d, 期望 1（不应混入框架默认日志）:\n%s", lines, buf.String())
	}
}

// TestBuildUnknownRouteReturnsEnvelope 验证未注册路径返回信封化 404，
// 而不是框架默认的空响应体——前端对每个响应都做 JSON.parse。
func TestBuildUnknownRouteReturnsEnvelope(t *testing.T) {
	r, _ := buildForTest(t)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/not-registered", nil))

	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("404 响应体不是合法信封: %v, body=%q", err, rec.Body.String())
	}
	if env.Code < 40400 || env.Code > 40499 {
		t.Errorf("404 的 code = %d, 期望落在 404xx 分段", env.Code)
	}
	if string(env.Data) != "null" {
		t.Errorf("404 响应 data = %s, 期望 null", env.Data)
	}
}

// TestBuildAppliesCORSWhitelistFromConfig 验证跨域白名单确实取自配置，
// 而不是硬编码在中间件里。
func TestBuildAppliesCORSWhitelistFromConfig(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := config.Default()
	cfg.CORS.AllowOrigins = []string{"http://localhost:4321"}
	r := Build(cfg, buf)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "http://localhost:4321")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:4321" {
		t.Errorf("Allow-Origin = %q, 期望配置中的白名单来源", got)
	}
}

// TestBuildUsesFactoryLoggerWithoutLeakingDefaultLogs 验证装配过程本身
// 不产生任何日志，且日志出口就是外部传入的目标——框架默认日志一旦混入，
// 这个缓冲里就会出现它的行。
func TestBuildUsesFactoryLoggerWithoutLeakingDefaultLogs(t *testing.T) {
	_, buf := buildForTest(t)

	if buf.Len() != 0 {
		t.Fatalf("装配过程不应产生日志: %s", buf.String())
	}
}

// TestBuildRecoversFromPanicWithEnvelope 验证整条中间件链上 panic 被兜住，
// 且返回的仍是可解析的信封。这里直接复用 platform 中间件的恢复能力，
// 借助一个额外的 panic 路由做端到端确认。
func TestBuildRecoversFromPanicWithEnvelope(t *testing.T) {
	buf := &bytes.Buffer{}
	cfg := config.Default()
	r := Build(cfg, buf)

	// Build 的中间件链已挂恢复；通过一个临时注册的 panic 路由验证它确实生效。
	r.GET("/api/panic-for-test", func(c *gin.Context) { panic("装配层恢复验证") })

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/panic-for-test", nil))

	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("panic 后响应体不是合法信封: %v, body=%q", err, rec.Body.String())
	}
	if env.Code < 50000 || env.Code > 50099 {
		t.Errorf("panic 的 code = %d, 期望落在 500xx 分段", env.Code)
	}
	if !strings.Contains(buf.String(), "装配层恢复验证") {
		t.Errorf("panic 未被记录到统一日志出口: %s", buf.String())
	}
}
