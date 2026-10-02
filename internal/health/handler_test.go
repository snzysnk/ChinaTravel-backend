package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newHealthRouter 构造只挂载健康检查路由的引擎，供各用例复用。
func newHealthRouter() *gin.Engine {
	r := gin.New()
	NewHandler().Register(r.Group("/api"))
	return r
}

// envelope 是测试侧使用的信封视图，字段与 api-response-envelope 规格一致。
// 各领域测试各自声明一份（而非共享），以免测试之间产生隐式耦合。
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// TestCheckReturnsHealthyEnvelope 验证健康检查返回 200、code 为 0，
// 且 data 中存在状态字段。
func TestCheckReturnsHealthyEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	newHealthRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))

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
	if env.Msg == "" {
		t.Error("成功响应也应有提示文案")
	}

	var data map[string]any
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("data 不是对象: %v, data=%s", err, env.Data)
	}
	if status, ok := data["status"]; !ok || status == "" {
		t.Errorf("data 中缺少非空的状态字段: %s", env.Data)
	}
}
