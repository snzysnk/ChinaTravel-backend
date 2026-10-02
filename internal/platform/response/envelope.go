// Package response 定义全项目统一的 HTTP 响应信封。
//
// 契约（见 `api-response-envelope` 规格）：
//  1. 每个响应体都是一个 `{code, msg, data}` 结构的 JSON 对象，字段不多不少；
//  2. HTTP 状态码恒为 200，业务成败只由 `code` 表达；
//  3. 处理函数不得手写 JSON 字面量，一律经由本包的构造函数产出，
//     这样信封格式若要变更只需改动这一处。
package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Code 是响应信封中的业务码。
//
// 编码规则为五位分段：前 3 位表示 HTTP 语义类别，后 2 位表示该类别下的业务序号。
// 已声明的分段：
//
//	0      — 成功
//	400xx  — 客户端请求错误（参数缺失、格式非法等）
//	401xx  — 未认证
//	403xx  — 无权限
//	404xx  — 资源不存在
//	500xx  — 服务端内部错误
//
// 约定：只定义当前用得到的常量（YAGNI）。新增错误时，归入上表中已有分段中
// 最贴近的一类并在此处补一个常量，不得使用未声明的前缀。
type Code int

const (
	// CodeOK 表示请求被正常处理完成。
	CodeOK Code = 0

	// CodeNotFound 表示请求的资源不存在。占 404xx 分段中的 01 号。
	CodeNotFound Code = 40401

	// CodeInternalError 表示服务端内部错误（含 panic 恢复）。
	// 占 500xx 分段中的 01 号。
	CodeInternalError Code = 50001
)

// 成功与失败提示的默认文案。集中在此处便于统一措辞。
const (
	msgOK       = "成功"
	msgNotFound = "资源不存在"
	msgInternal = "服务内部错误"
)

// Envelope 是所有 HTTP 响应体的结构，字段与 JSON 键名一一对应。
type Envelope struct {
	// Code 是业务码，0 表示成功，非零表示失败，取值见 Code 类型的分段说明。
	Code Code `json:"code"`
	// Msg 是面向使用者的提示文案，成功与失败都提供服务端给定的中文提示。
	Msg string `json:"msg"`
	// Data 承载业务数据。成功时为对象或数组；失败时固定为 null，
	// 以免前端拿到半截数据后继续渲染。
	Data any `json:"data"`
}

// OK 构造「成功且携带数据」的信封。
func OK(data any) Envelope {
	return Envelope{Code: CodeOK, Msg: msgOK, Data: data}
}

// OKWithMsg 构造「成功但自定义提示」的信封。
// 适用于"处理成功但需要告知使用者额外信息"的场景。
func OKWithMsg(msg string, data any) Envelope {
	return Envelope{Code: CodeOK, Msg: msg, Data: data}
}

// Fail 构造「失败」信封，Data 固定为 null。
// msg 传空字符串时使用与 code 对应的默认文案。
func Fail(code Code, msg string) Envelope {
	return Envelope{Code: code, Msg: resolveMsg(code, msg), Data: nil}
}

// resolveMsg 在调用方未提供提示文案时，按业务码回落到默认文案。
// 这样调用方只需关心"哪种错误"，不必每次都重复写一遍提示语。
func resolveMsg(code Code, msg string) string {
	if msg != "" {
		return msg
	}

	switch code {
	case CodeNotFound:
		return msgNotFound
	case CodeInternalError:
		return msgInternal
	default:
		return "请求处理失败"
	}
}

// WriteOK 是「成功且携带数据」的便捷出口，直接写入 gin 上下文。
func WriteOK(c *gin.Context, data any) {
	write(c, OK(data))
}

// WriteFail 是「失败」的便捷出口，直接写入 gin 上下文。
// 注意：即使 code 表示失败，HTTP 状态码依然恒为 200。
func WriteFail(c *gin.Context, code Code, msg string) {
	write(c, Fail(code, msg))
}

// write 是唯一真正调用 gin 序列化的地方，保证 HTTP 状态码恒为 200，
// 且信封结构不会被各处理函数各自改写。
func write(c *gin.Context, e Envelope) {
	c.JSON(http.StatusOK, e)
}
