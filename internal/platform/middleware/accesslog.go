// Package middleware 提供跨领域复用的 HTTP 中间件。
//
// 三个中间件各司其职：访问日志（accesslog.go）、跨域校验（cors.go）、
// panic 恢复（recovery.go）。它们都只依赖标准库与本项目 platform 包，
// 不反向依赖任何业务领域。
package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// 访问日志中使用的字段名。集中在此处是为了让字段名可被测试引用，
// 避免测试里写死字符串后与实现悄悄脱节。
const (
	// FieldMethod 是请求方法字段名。
	FieldMethod = "method"
	// FieldPath 是请求路径字段名。
	FieldPath = "path"
	// FieldStatus 是 HTTP 状态码字段名。
	FieldStatus = "status"
	// FieldDurationMS 是请求耗时（毫秒）字段名。
	FieldDurationMS = "duration_ms"
	// FieldClientIP 是客户端来源地址字段名。
	FieldClientIP = "client_ip"
)

// AccessLogMsg 是访问日志的固定消息文本。
const AccessLogMsg = "http_access"

// AccessLog 返回记录访问日志的中间件。
//
// 为什么不使用框架自带的日志中间件：
// gin 默认的 Logger 把日志写到自己的 writer，格式是 gin 自定的文本行，
// 与应用日志（slog）格式不一致，会形成 `observability-baseline` 规格所禁止的
// "多路日志输出"——排查问题时得在两种格式之间来回对照。这里改为自写中间件，
// 让访问日志与应用日志共用同一个 slog 出口、同一套字段格式。
//
// 每个请求恰好产生一条日志：计数与耗时在 c.Next() 返回后一次性记录，
// 因此即使处理函数中途写了响应，也不会多出一条。
func AccessLog(log *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 先放行后续处理，再记录日志，这样能拿到最终的状态码与总耗时。
		c.Next()

		attrs := []any{
			FieldMethod, c.Request.Method,
			FieldPath, c.Request.URL.Path,
			FieldStatus, c.Writer.Status(),
			FieldDurationMS, time.Since(start).Milliseconds(),
			FieldClientIP, c.ClientIP(),
		}

		log.Info(AccessLogMsg, attrs...)
	}
}
