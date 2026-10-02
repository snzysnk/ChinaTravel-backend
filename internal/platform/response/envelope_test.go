package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// init 关闭 gin 的调试输出，避免测试日志污染标准输出，
// 也避免 gin 自身打印与 slog 出口混流。
func init() {
	gin.SetMode(gin.TestMode)
}

// decodeEnvelope 把响应体解析为「键到原始值」的映射。
// 刻意不用 Envelope 结构体来解析：那样只能验证"能解析"，无法发现多出的字段。
func decodeEnvelope(t *testing.T, body []byte) map[string]json.RawMessage {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("响应体不是合法 JSON 对象: %v, body=%s", err, body)
	}
	return raw
}

// TestOKCarriesData 验证成功信封的 code 为 0 且 data 非空。
func TestOKCarriesData(t *testing.T) {
	e := OK([]string{"a", "b"})

	if e.Code != CodeOK {
		t.Errorf("成功信封 code = %d, 期望 %d", e.Code, CodeOK)
	}
	if e.Data == nil {
		t.Error("成功信封 data 不应为 nil")
	}
	if e.Msg == "" {
		t.Error("成功信封 msg 不应为空")
	}
}

// TestOKWithMsgKeepsCodeZero 验证自定义提示不影响成功码。
func TestOKWithMsgKeepsCodeZero(t *testing.T) {
	e := OKWithMsg("自定义提示", 1)

	if e.Code != CodeOK {
		t.Errorf("code = %d, 期望 0", e.Code)
	}
	if e.Msg != "自定义提示" {
		t.Errorf("msg = %q, 期望自定义提示", e.Msg)
	}
}

// TestFailCarriesNilData 验证失败信封的 data 为 null、code 非零。
func TestFailCarriesNilData(t *testing.T) {
	e := Fail(CodeNotFound, "目的地不存在")

	if e.Code == CodeOK {
		t.Error("失败信封 code 不应为 0")
	}
	if e.Data != nil {
		t.Errorf("失败信封 data 应为 nil, 实际 %v", e.Data)
	}
	if e.Msg != "目的地不存在" {
		t.Errorf("msg = %q, 期望自定义文案", e.Msg)
	}
}

// TestFailFallsBackToDefaultMsg 验证未提供文案时按 code 回落默认文案。
func TestFailFallsBackToDefaultMsg(t *testing.T) {
	if msg := Fail(CodeNotFound, "").Msg; msg == "" {
		t.Error("未提供 msg 时应回落到默认文案，实际为空")
	}
	if msg := Fail(CodeInternalError, "").Msg; msg == "" {
		t.Error("未提供 msg 时应回落到默认文案，实际为空")
	}
}

// TestNotFoundCodeInSegment 验证资源不存在码落在 404xx 分段内，
// 守住所声明的分段编码规则。
func TestNotFoundCodeInSegment(t *testing.T) {
	if CodeNotFound < 40400 || CodeNotFound > 40499 {
		t.Errorf("CodeNotFound = %d, 不在 404xx 分段内", CodeNotFound)
	}
	if CodeInternalError < 50000 || CodeInternalError > 50099 {
		t.Errorf("CodeInternalError = %d, 不在 500xx 分段内", CodeInternalError)
	}
}

// TestResponseBodyHasExactlyThreeKeys 验证序列化后的字段不多不少。
func TestResponseBodyHasExactlyThreeKeys(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	WriteOK(c, map[string]string{"hello": "world"})

	raw := decodeEnvelope(t, rec.Body.Bytes())
	if len(raw) != 3 {
		t.Fatalf("信封字段数 = %d, 期望 3, body=%s", len(raw), rec.Body.String())
	}
	for _, key := range []string{"code", "msg", "data"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("信封缺少字段 %q, body=%s", key, rec.Body.String())
		}
	}
}

// TestWriteFailKeepsHTTP200 验证业务失败时 HTTP 状态码仍为 200，
// 这是信封契约中最容易被无意破坏的一条。
func TestWriteFailKeepsHTTP200(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	WriteFail(c, CodeNotFound, "")

	if rec.Code != http.StatusOK {
		t.Errorf("HTTP 状态码 = %d, 期望 200", rec.Code)
	}

	raw := decodeEnvelope(t, rec.Body.Bytes())
	if string(raw["data"]) != "null" {
		t.Errorf("失败响应 data = %s, 期望 null", raw["data"])
	}
}
