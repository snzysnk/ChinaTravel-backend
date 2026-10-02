package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// 跨域响应头名称。集中定义以便测试引用，避免字符串写死后与实现脱节。
const (
	// HeaderAllowOrigin 声明允许访问的来源。
	HeaderAllowOrigin = "Access-Control-Allow-Origin"
	// HeaderAllowMethods 声明预检允许的请求方法。
	HeaderAllowMethods = "Access-Control-Allow-Methods"
	// HeaderAllowHeaders 声明预检允许的请求头。
	HeaderAllowHeaders = "Access-Control-Allow-Headers"
	// HeaderAllowCredentials 声明允许携带凭据。
	HeaderAllowCredentials = "Access-Control-Allow-Credentials"
	// HeaderMaxAge 声明预检结果的有效期（秒）。
	HeaderMaxAge = "Access-Control-Max-Age"
	// HeaderVary 声明响应随 Origin 变化，供中间缓存正确区分来源。
	HeaderVary = "Vary"
)

// 预检允许的请求方法与请求头。
const (
	// allowedMethods 是允许的请求方法集合，以逗号分隔写入响应头。
	allowedMethods = "GET, POST, PUT, PATCH, DELETE, OPTIONS"
	// allowedHeaders 是允许的请求头集合。`Content-Type` 为 JSON 请求体所必需，
	// `Authorization` 为后续引入认证预留。
	allowedHeaders = "Content-Type, Authorization"
	// preflightMaxAgeSeconds 是预检结果的有效期（秒），取值 12 小时。
	// 取值偏大是为了减少前端每次请求前都要多打一次预检的开销。
	preflightMaxAgeSeconds = "43200"
	// varyValue 是 Vary 响应头的取值。
	varyValue = "Origin"
)

// CORS 返回按白名单校验来源的跨域中间件。
//
// allowOrigins 是完整来源（scheme + host + port）白名单，来自配置，
// 新增前端来源只需改配置，不必动本文件。
//
// 为什么允许凭据时来源不能是通配符 `*`：
// 携带凭据（Cookie、Authorization）的跨域请求，浏览器会拒绝接受
// `Access-Control-Allow-Origin: *` 的响应——通配符意味着"任何站点都能带着
// 用户凭据读这个接口"，浏览器正是为堵这个洞才这么规定。因此这里对每个来源
// 回显其具体值，并在白名单为空或来源不匹配时一个跨域头都不写。
//
// 与信封契约的关系：跨域校验发生在业务处理之前，被拒绝的来源拿到的是一个
// 不带跨域头的响应，浏览器会阻止页面脚本读取它——响应体本身是否合规不是重点，
// 重点是这个响应根本到不了脚本手里。
func CORS(allowOrigins []string) gin.HandlerFunc {
	// 先归一化白名单：去掉空白项与首尾空格，避免配置里多写一个空格就静默失效。
	allowSet := make(map[string]struct{}, len(allowOrigins))
	for _, origin := range allowOrigins {
		if trimmed := strings.TrimSpace(origin); trimmed != "" {
			allowSet[trimmed] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// 同源请求（含服务端直接调用、curl）不带 Origin：直接放行，
		// 且不添加任何跨域头——给同源响应加跨域头会让缓存与调试都变得难以理解。
		if origin == "" {
			c.Next()
			return
		}

		// 无论是否放行都要声明 Vary，使缓存不会把某个来源的响应复用给另一个来源。
		c.Header(HeaderVary, varyValue)

		if _, ok := allowSet[origin]; !ok {
			// 白名单之外的来源：照常处理请求，但不授予跨域读取权限。
			c.Next()
			return
		}

		c.Header(HeaderAllowOrigin, origin)
		c.Header(HeaderAllowCredentials, "true")

		if c.Request.Method == http.MethodOptions {
			// 预检请求只需回答"允不允许"，不进入业务处理，也不返回业务信封。
			c.Header(HeaderAllowMethods, allowedMethods)
			c.Header(HeaderAllowHeaders, allowedHeaders)
			c.Header(HeaderMaxAge, preflightMaxAgeSeconds)
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
