// Command server 是 ChinaTravel 后端的进程入口。
//
// 本文件只承担进程生命周期相关的职责：加载配置、装配应用、绑定端口启动、
// 等待终止信号并优雅关闭。它不构造任何领域对象——那是 bootstrap.Build 的事。
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/snzysnk/ChinaTravel-backend/internal/bootstrap"
	"github.com/snzysnk/ChinaTravel-backend/internal/platform/config"
)

// exitCodeFailure 是进程异常退出时使用的退出码。
const exitCodeFailure = 1

func main() {
	if err := run(); err != nil {
		// 这里使用 stderr 直写而非 slog：run 在日志出口建立之前就可能失败
		// （例如配置文件格式非法），此时还没有可用的统一日志出口。
		fmt.Fprintf(os.Stderr, "服务启动失败: %v\n", err)
		os.Exit(exitCodeFailure)
	}
}

// run 完成一次完整的服务生命周期，返回错误表示启动阶段失败。
//
// 优雅关闭流程：
//  1. 用 signal.NotifyContext 监听 SIGINT / SIGTERM，收到信号即取消 ctx；
//  2. 主 goroutine 阻塞在 select 上，直到监听失败或 ctx 被取消；
//  3. 取消后调用 http.Server.Shutdown，停止接受新连接并等待进行中的请求；
//  4. Shutdown 的 context 带超时上限（来自配置），超时则不再等待直接返回，
//     避免个别卡死的请求把进程永久挂住。
func run() error {
	cfg, err := config.Load("")
	if err != nil {
		return err
	}

	handler := bootstrap.Build(cfg, os.Stdout)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: handler,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 监听放在独立 goroutine：主 goroutine 需要腾出来等待信号。
	// 正常关闭时 ListenAndServe 返回 ErrServerClosed，不属于故障。
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("监听 %s 失败: %w", srv.Addr, err)
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		// 收到终止信号：先恢复信号的默认行为，使第二次 Ctrl-C 能强制结束，
		// 避免"优雅关闭卡住时连强杀都做不到"。
		stop()
	}

	shutdownTimeout := time.Duration(cfg.Server.ShutdownTimeoutSeconds) * time.Second
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		// 超时或出错都说明未能干净退出，如实上报而不是假装成功。
		return fmt.Errorf("优雅关闭未在 %s 内完成: %w", shutdownTimeout, err)
	}
	return nil
}
