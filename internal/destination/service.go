package destination

import (
	"context"
	"errors"
	"fmt"
)

// Service 是目的地的业务逻辑层。
//
// 它只依赖 Repository 接口，不感知数据究竟来自内存还是数据库，
// 也不感知 HTTP——因此本层可以在没有网络与数据库的情况下被单独替换与测试。
type Service struct {
	// repo 是数据访问抽象，由装配模块注入。
	repo Repository
}

// NewService 构造目的地服务。
// 参数类型刻意声明为接口而非具体实现，由装配模块决定注入哪种仓储，
// 使"换成数据库实现"成为一次构造替换，而非一次跨层修改。
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// List 返回全部目的地。
// 仓储报错时原样上抛并附带本层上下文，由处理函数统一翻译成信封。
func (s *Service) List(ctx context.Context) ([]Destination, error) {
	items, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("查询目的地列表失败: %w", err)
	}
	return items, nil
}

// GetByID 按标识返回单个目的地。
// 目标不存在时返回包装了 ErrNotFound 的错误，供处理函数翻译成 404xx 业务码。
func (s *Service) GetByID(ctx context.Context, id string) (Destination, error) {
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return Destination{}, fmt.Errorf("目的地 %s 不存在: %w", id, ErrNotFound)
		}
		return Destination{}, fmt.Errorf("查询目的地 %s 失败: %w", id, err)
	}
	return d, nil
}
