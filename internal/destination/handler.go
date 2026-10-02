package destination

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/snzysnk/ChinaTravel-backend/internal/platform/response"
)

// Handler 承载目的地领域的 HTTP 处理函数。
type Handler struct {
	// svc 是业务层入口，由装配模块注入。
	svc *Service
}

// NewHandler 构造目的地处理函数。
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register 把本领域的路由注册到给定路由组（路由组已带 /api 前缀）。
func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/destinations", h.List)
	rg.GET("/destinations/:id", h.GetByID)
}

// List 处理 GET /api/destinations，返回全部目的地的成功信封。
// 数据全部来自仓储层，处理函数不硬编码任何条目。
func (h *Handler) List(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		response.WriteFail(c, response.CodeInternalError, "查询目的地列表失败")
		return
	}
	response.WriteOK(c, items)
}

// GetByID 处理 GET /api/destinations/:id。
//
// 目标不存在时返回 **HTTP 200 + 404xx 业务码 + data 为 null**：
// 按信封契约，业务失败不改 HTTP 状态码，前端只看 code，
// 因此这里刻意不调用 c.JSON(http.StatusNotFound, ...)。
//
// 关于「空标识」：gin 的路径匹配不会把空片段送进 :id——请求 `/api/destinations/`
// 会先被 RedirectTrailingSlash 以 301 重定向到 `/api/destinations`，
// 因此这里不存在"标识为空"的分支，也不需要为它定义一个用不到的 400xx 码。
func (h *Handler) GetByID(c *gin.Context) {
	id := c.Param("id")

	d, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.WriteFail(c, response.CodeNotFound, "目的地 "+id+" 不存在")
			return
		}
		response.WriteFail(c, response.CodeInternalError, "查询目的地失败")
		return
	}
	response.WriteOK(c, d)
}
