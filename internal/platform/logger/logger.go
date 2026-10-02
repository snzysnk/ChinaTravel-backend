// Package logger 提供应用唯一的日志构造入口。
//
// 约定：全项目只允许通过本包获取日志句柄，底层使用标准库 log/slog。
// 业务代码不得直接使用 fmt.Print / log.Print 等无结构输出，也不得引入
// 第二套日志库——多路日志会破坏"所有日志格式一致、落在同一输出流"的约定，
// 使排查问题时需要在多种格式之间来回切换。
package logger

import (
	"io"
	"log/slog"

	"github.com/snzysnk/ChinaTravel-backend/internal/platform/config"
)

// New 按配置构造日志句柄。
//
// 日志级别取自 cfg.SlogLevel()（非法级别已在配置层回落到 info）。
// 输出目标由 w 指定，进程入口传入 os.Stdout；测试可传入内存缓冲以便断言。
func New(cfg *config.Config, w io.Writer) *slog.Logger {
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: cfg.SlogLevel(),
	})
	return slog.New(handler)
}
