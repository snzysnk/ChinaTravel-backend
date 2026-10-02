package middleware

import (
	"log/slog"
	"runtime"

	"github.com/gin-gonic/gin"

	"github.com/snzysnk/ChinaTravel-backend/internal/platform/response"
)

// RecoveryMsg 是 panic 日志的固定消息文本，便于按关键词检索。
const RecoveryMsg = "http_panic_recovered"

// 日志字段名。
const (
	// FieldPanic 记录 panic 的值。
	FieldPanic = "panic"
	// FieldStack 记录 panic 时的调用栈。
	FieldStack = "stack"
)

// Recovery 返回 panic 恢复中间件。
//
// 为什么必须自写而不能用 gin 的默认 Recovery：
// 默认实现返回 HTTP 500 与空响应体。在统一信封契约下，前端对每个响应都执行
// `JSON.parse`，空 body 会直接抛出异常，把一个后端 panic 放大成前端页面崩溃。
// 因此这里必须在恢复后写回一个合法的信封，且 HTTP 状态码保持 200，
// 使前端能走到"code 非零则展示 msg"的既有分支。
func Recovery(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}

			log.Error(RecoveryMsg,
				FieldPath, c.Request.URL.Path,
				FieldMethod, c.Request.Method,
				FieldPanic, r,
				FieldStack, stackTrace(),
			)

			// 处理函数可能已经写入了部分响应体，此时只能中断连接，
			// 再追加信封会造成"半个 JSON 拼上另一个 JSON"的畸形响应。
			if c.Writer.Written() {
				c.Abort()
				return
			}

			response.WriteFail(c, response.CodeInternalError, "")
			c.Abort()
		}()

		c.Next()
	}
}

// stackTrace 返回当前 goroutine 的调用栈快照。
// 单独抽成函数是为了让 Recovery 的主体保持"恢复 → 记日志 → 写响应"三步可读。
// 缓冲区不够时 runtime.Stack 会把内容截断，这里接受截断——
// 定位 panic 通常只需栈顶若干帧。
func stackTrace() string {
	const size = 4096
	buf := make([]byte, size)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}
