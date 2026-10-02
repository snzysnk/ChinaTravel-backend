// Package bootstrap 负责把一个可运行的 HTTP 应用装配出来。
//
// 装配（谁 new 谁、中间件顺序如何、路由挂在哪）集中在本包，
// 进程入口只负责加载配置、调用 Build、启动监听与处理信号。
// 这样依赖变多时膨胀的是本包，而 main.go 始终只有进程生命周期那点逻辑。
package bootstrap

import (
	"io"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/snzysnk/ChinaTravel-backend/internal/destination"
	"github.com/snzysnk/ChinaTravel-backend/internal/health"
	"github.com/snzysnk/ChinaTravel-backend/internal/platform/config"
	"github.com/snzysnk/ChinaTravel-backend/internal/platform/logger"
	"github.com/snzysnk/ChinaTravel-backend/internal/platform/middleware"
	"github.com/snzysnk/ChinaTravel-backend/internal/platform/response"
)

// apiPrefix 是所有业务接口共用的路径前缀。
const apiPrefix = "/api"

// Build 按配置装配出完整的 HTTP 处理器。
//
// 返回的处理器**不绑定任何端口**，端口绑定与监听交给进程入口，
// 这样测试可以直接对返回值发请求而不占用真实端口。
//
// out 是日志输出目标，进程入口传 os.Stdout，测试可传内存缓冲以断言日志。
//
// 中间件顺序（自外向内）：恢复 → 访问日志 → 跨域。
//   - 恢复在最外层：业务处理、跨域校验乃至访问日志自身发生 panic 时都能兜住，
//     使 panic 处在一个"必定被记录"的位置；
//   - 访问日志在跨域之内：记到的是请求真正进入处理链之后的结果，
//     且被跨域中断的预检不会混进访问日志；
//   - 跨域在最内层：白名单不匹配时请求照常被处理，只是响应不带授权头。
func Build(cfg *config.Config, out io.Writer) *gin.Engine {
	if out == nil {
		out = os.Stdout
	}

	// Gin 默认以 debug 模式启动，会向标准输出打印带 [GIN-debug] 前缀的
	// 路由表与警告行——这是与 slog 格式不一致的第二路日志输出，
	// 正是 observability-baseline 规格所禁止的。因此这里显式切到 release 模式。
	gin.SetMode(gin.ReleaseMode)

	log := logger.New(cfg, out)

	// 使用 gin.New() 而非 gin.Default()：后者的 Logger 中间件会打印
	// 框架自定义格式的日志行，与本项目的统一日志出口冲突。
	r := gin.New()
	r.Use(
		middleware.Recovery(log),
		middleware.AccessLog(log),
		middleware.CORS(cfg.CORS.AllowOrigins),
	)

	registerRoutes(r)

	return r
}

// registerRoutes 完成各领域的路由注册。
//
// 这是"新增一个领域时唯一需要改动的既有文件"（见 http-server-bootstrap 规格）：
// 新的领域包只需在此处构造依赖并调用它的 Register。
func registerRoutes(r *gin.Engine) {
	api := r.Group(apiPrefix)

	// health 领域：无依赖，直接构造。
	health.NewHandler().Register(api)

	// destination 领域：装配顺序为 仓储 -> 服务 -> 处理函数，
	// 依赖全部以接口形式注入，替换数据来源只需改这一行。
	destination.NewHandler(
		destination.NewService(
			destination.NewMemoryRepository(),
		),
	).Register(api)

	// 未匹配任何路由时返回信封化的 404，而不是框架默认的空响应体，
	// 以免前端在路由写错时拿到无法解析的 body。
	r.NoRoute(func(c *gin.Context) {
		response.WriteFail(c, response.CodeNotFound, "接口不存在")
	})
}
