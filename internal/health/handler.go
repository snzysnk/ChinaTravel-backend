// Package health 提供服务的存活探测接口。
package health

import (
	"github.com/gin-gonic/gin"

	"github.com/snzysnk/ChinaTravel-backend/internal/platform/response"
)

// statusOK 是健康响应中 data.status 的取值。
const statusOK = "ok"

// Handler 承载健康检查的处理函数。
// 该领域无需仓储与服务层——它只回答"进程活着、路由可达"这一个问题。
type Handler struct{}

// NewHandler 构造健康检查处理函数。
func NewHandler() *Handler {
	return &Handler{}
}

// Register 把健康检查路由注册到给定路由组。
//
// 接口用途：给运维、负载均衡与前端页面一个不触碰业务数据的探针，
// 判断后端进程是否存活；前端首页据此展示"服务可用/不可用"。
func (h *Handler) Register(rg *gin.RouterGroup) {
	rg.GET("/health", h.Check)
}

// Check 处理 GET /api/health，返回成功信封与最简状态信息。
//
// data 中刻意只放状态字段，不掺入版本号、依赖探测等易变信息，
// 使该接口的契约长期稳定、可被探针无条件依赖。
func (h *Handler) Check(c *gin.Context) {
	response.WriteOK(c, gin.H{
		"status": statusOK,
	})
}
