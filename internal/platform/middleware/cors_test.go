package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// whitelist 是各用例共用的白名单。
var whitelist = []string{"http://localhost:5174"}

// newCORSRouter 构造一个挂了 CORS 中间件与一个探针处理函数的路由，
// 处理函数会把请求真正走到业务层这一事实记录下来。
func newCORSRouter() (*gin.Engine, *bool) {
	handled := false

	r := gin.New()
	r.Use(CORS(whitelist))
	r.Any("/api/health", func(c *gin.Context) {
		handled = true
		c.String(http.StatusOK, "ok")
	})
	return r, &handled
}

// TestCORSWildcardIsNeverEmitted 验证任何场景下都不会输出通配符来源。
// 允许携带凭据时使用 `*` 会被浏览器直接拒绝，这条断言守住该约束。
func TestCORSWildcardIsNeverEmitted(t *testing.T) {
	r, _ := newCORSRouter()

	for _, origin := range []string{"http://localhost:5174", "http://evil.example.com"} {
		req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
		req.Header.Set("Origin", origin)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if got := rec.Header().Get(HeaderAllowOrigin); got == "*" {
			t.Errorf("来源 %s 的响应使用了通配符 Allow-Origin", origin)
		}
	}
}

// TestCORSAllowedOriginIsGranted 验证白名单内的来源被授予跨域访问权限。
func TestCORSAllowedOriginIsGranted(t *testing.T) {
	r, handled := newCORSRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "http://localhost:5174")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get(HeaderAllowOrigin); got != "http://localhost:5174" {
		t.Errorf("Allow-Origin = %q, 期望回显具体来源", got)
	}
	if got := rec.Header().Get(HeaderAllowCredentials); got != "true" {
		t.Errorf("Allow-Credentials = %q, 期望 true", got)
	}
	if !*handled {
		t.Error("白名单内的来源应正常进入业务处理")
	}
}

// TestCORSDisallowedOriginIsNotGranted 验证白名单外的来源拿不到跨域授权头。
// 服务端仍会处理请求，但浏览器不会把响应交给页面脚本。
func TestCORSDisallowedOriginIsNotGranted(t *testing.T) {
	r, _ := newCORSRouter()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "http://localhost:9999")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get(HeaderAllowOrigin); got != "" {
		t.Errorf("白名单外来源不应获得 Allow-Origin, 实际 %q", got)
	}
	if got := rec.Header().Get(HeaderAllowCredentials); got != "" {
		t.Errorf("白名单外来源不应获得 Allow-Credentials, 实际 %q", got)
	}
	if got := rec.Header().Get(HeaderVary); got != varyValue {
		t.Errorf("Vary = %q, 期望 %q（避免缓存串来源）", got, varyValue)
	}
}

// TestCORSPreflightIsAnswered 验证预检请求被正确响应并声明方法、请求头与有效期。
func TestCORSPreflightIsAnswered(t *testing.T) {
	r, handled := newCORSRouter()

	req := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	req.Header.Set("Origin", "http://localhost:5174")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code < 200 || rec.Code >= 300 {
		t.Errorf("预检响应状态码 = %d, 期望 2xx", rec.Code)
	}
	if got := rec.Header().Get(HeaderAllowOrigin); got != "http://localhost:5174" {
		t.Errorf("预检未授予来源: %q", got)
	}
	if got := rec.Header().Get(HeaderAllowMethods); got == "" {
		t.Error("预检未声明允许的请求方法")
	}
	if got := rec.Header().Get(HeaderAllowHeaders); got == "" {
		t.Error("预检未声明允许的请求头")
	}
	if got := rec.Header().Get(HeaderMaxAge); got == "" {
		t.Error("预检未声明有效期")
	}
	if *handled {
		t.Error("预检请求不应进入业务处理函数")
	}
	if body := rec.Body.String(); body != "" {
		t.Errorf("预检不应返回业务信封, 实际 body=%q", body)
	}
}

// TestCORSPreflightFromDisallowedOriginIsNotGranted 验证白名单之外的预检
// 同样不予授权，否则"白名单"形同虚设。
func TestCORSPreflightFromDisallowedOriginIsNotGranted(t *testing.T) {
	r, _ := newCORSRouter()

	req := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	req.Header.Set("Origin", "http://evil.example.com")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get(HeaderAllowOrigin); got != "" {
		t.Errorf("白名单外来源的预检不应获得 Allow-Origin, 实际 %q", got)
	}
}

// TestCORSSameOriginRequestUnaffected 验证不带 Origin 的同源请求被正常处理，
// 且响应上不出现任何跨域头。
func TestCORSSameOriginRequestUnaffected(t *testing.T) {
	r, handled := newCORSRouter()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("同源请求状态码 = %d, 期望 200", rec.Code)
	}
	if !*handled {
		t.Error("同源请求应正常进入业务处理")
	}
	for _, header := range []string{
		HeaderAllowOrigin, HeaderAllowCredentials, HeaderAllowMethods, HeaderAllowHeaders, HeaderVary,
	} {
		if got := rec.Header().Get(header); got != "" {
			t.Errorf("同源请求不应带有跨域头 %s=%q", header, got)
		}
	}
}

// TestCORSWhitelistEntryWithWhitespaceStillMatches 验证配置项中的多余空白
// 不会导致白名单静默失效（配置写法容错）。
func TestCORSWhitelistEntryWithWhitespaceStillMatches(t *testing.T) {
	r := gin.New()
	r.Use(CORS([]string{"  http://localhost:5174  "}))
	r.GET("/api/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "http://localhost:5174")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get(HeaderAllowOrigin); got != "http://localhost:5174" {
		t.Errorf("带空白的白名单项应被归一化后匹配, Allow-Origin = %q", got)
	}
}
