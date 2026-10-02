package destination

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// envelope 是测试侧使用的信封视图，与 api-response-envelope 规格一致。
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// newTestRouter 装配一条最小链路：内存仓储 -> 服务 -> 处理函数。
// 这里刻意走真实的三层，而不是给处理函数塞一个假服务——
// 本变更要验证的正是这条链路本身能跑通。
func newTestRouter() *gin.Engine {
	repo := NewMemoryRepository()
	handler := NewHandler(NewService(repo))

	r := gin.New()
	handler.Register(r.Group("/api"))
	return r
}

// doRequest 向测试路由发起一次请求并解析响应信封。
func doRequest(t *testing.T, r *gin.Engine, method, path string) (*httptest.ResponseRecorder, envelope) {
	t.Helper()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(method, path, nil))

	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("响应体不是合法信封: %v, body=%q", err, rec.Body.String())
	}
	return rec, env
}

// TestListReturnsAllItems 验证列表接口返回成功信封，data 为数组，
// 且长度与仓储条目数一致——这条断言同时守住了"处理函数不得硬编码数据"。
func TestListReturnsAllItems(t *testing.T) {
	rec, env := doRequest(t, newTestRouter(), http.MethodGet, "/api/destinations")

	if rec.Code != http.StatusOK {
		t.Errorf("HTTP 状态码 = %d, 期望 200", rec.Code)
	}
	if env.Code != 0 {
		t.Errorf("code = %d, 期望 0", env.Code)
	}

	var items []Destination
	if err := json.Unmarshal(env.Data, &items); err != nil {
		t.Fatalf("data 不是数组: %v, data=%s", err, env.Data)
	}
	if len(items) != len(sampleDestinations()) {
		t.Errorf("返回条目数 = %d, 期望 %d", len(items), len(sampleDestinations()))
	}
}

// TestListItemsCarryAllFields 验证列表项字段完整，避免前端渲染出空白条目。
func TestListItemsCarryAllFields(t *testing.T) {
	_, env := doRequest(t, newTestRouter(), http.MethodGet, "/api/destinations")

	var items []Destination
	if err := json.Unmarshal(env.Data, &items); err != nil {
		t.Fatalf("data 不是数组: %v", err)
	}
	for i, item := range items {
		if item.ID == "" || item.Name == "" || item.Province == "" || item.Summary == "" {
			t.Errorf("第 %d 项字段不完整: %+v", i, item)
		}
	}
}

// TestGetByIDReturnsItem 验证按标识查询命中时返回成功信封。
func TestGetByIDReturnsItem(t *testing.T) {
	want := sampleDestinations()[0]

	_, env := doRequest(t, newTestRouter(), http.MethodGet, "/api/destinations/"+want.ID)

	if env.Code != 0 {
		t.Fatalf("code = %d, 期望 0, msg=%s", env.Code, env.Msg)
	}

	var got Destination
	if err := json.Unmarshal(env.Data, &got); err != nil {
		t.Fatalf("data 不是对象: %v", err)
	}
	if got != want {
		t.Errorf("查询结果 = %+v, 期望 %+v", got, want)
	}
}

// TestGetByIDNotFoundKeepsHTTP200WithNonZeroCode 是信封契约最关键的一条用例：
// 业务失败时 HTTP 状态码仍为 200，错误只由 code 表达，且 data 为 null。
func TestGetByIDNotFoundKeepsHTTP200WithNonZeroCode(t *testing.T) {
	rec, env := doRequest(t, newTestRouter(), http.MethodGet, "/api/destinations/not-a-real-id")

	if rec.Code != http.StatusOK {
		t.Errorf("HTTP 状态码 = %d, 期望 200（业务失败不得改状态码）", rec.Code)
	}
	if env.Code == 0 {
		t.Error("资源不存在时 code 应为非零")
	}
	if env.Code < 40400 || env.Code > 40499 {
		t.Errorf("code = %d, 期望落在 404xx 分段", env.Code)
	}
	if string(env.Data) != "null" {
		t.Errorf("失败响应 data = %s, 期望 null", env.Data)
	}
	if env.Msg == "" {
		t.Error("失败响应应携带中文提示")
	}
}

// TestTrailingSlashRedirectsToListRoute 记录一条**框架行为**而非本项目契约：
// 请求 `/api/destinations/` 时，gin 的 RedirectTrailingSlash 会先回 301
// 重定向到 `/api/destinations`，因此该请求根本不会进入 :id 的处理函数，
// 也不存在"标识为空"的业务分支（见 GetByID 的注释）。
//
// 这里刻意用未走信封断言的原始请求来验证，因为重定向响应体是 gin 生成的
// HTML 链接而非业务信封——这正是"未知路由与重定向不属于信封契约"的体现。
func TestTrailingSlashRedirectsToListRoute(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/destinations/", nil))

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("HTTP 状态码 = %d, 期望 301（gin 的尾斜杠重定向）", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/api/destinations" {
		t.Errorf("Location = %q, 期望 /api/destinations", loc)
	}
}

// TestServiceReturnsNotFoundForUnknownID 验证服务层把仓储的"查不到"
// 暴露为可被 errors.Is 识别的 ErrNotFound，而不是一个无法分类的普通错误。
func TestServiceReturnsNotFoundForUnknownID(t *testing.T) {
	svc := NewService(NewMemoryRepository())

	_, err := svc.GetByID(context.Background(), "not-a-real-id")
	if err == nil {
		t.Fatal("查询不存在的标识应返回错误")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("错误应可用 errors.Is 匹配 ErrNotFound, 实际: %v", err)
	}
}

// TestServiceAcceptsAnyRepositoryImplementation 验证服务层只依赖仓储接口。
// 这里注入一个第三方实现（不嵌入内存仓储），若服务层偷偷依赖了具体类型，
// 本用例将无法编译。
func TestServiceAcceptsAnyRepositoryImplementation(t *testing.T) {
	svc := NewService(stubRepository{})

	items, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List 返回错误: %v", err)
	}
	if len(items) != 1 || items[0].ID != "stub" {
		t.Errorf("服务层未使用注入的仓储实现: %+v", items)
	}
}

// stubRepository 是一个最小化的 Repository 实现，仅用于验证依赖方向。
type stubRepository struct{}

// List 返回一条固定数据，用于证明服务层确实调用了被注入的实现。
func (stubRepository) List(context.Context) ([]Destination, error) {
	return []Destination{{ID: "stub", Name: "占位"}}, nil
}

// GetByID 总是返回"不存在"，本用例不涉及该路径。
func (stubRepository) GetByID(context.Context, string) (Destination, error) {
	return Destination{}, ErrNotFound
}
