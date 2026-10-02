package destination

import (
	"context"
	"errors"
)

// ErrNotFound 表示按标识查询时目标不存在。
// 服务层据此把它翻译成信封中的 404xx 业务码，而不是让它冒泡成 500。
var ErrNotFound = errors.New("目的地不存在")

// Repository 是目的地的数据访问抽象。
//
// service 只依赖本接口、不依赖具体实现，因此将来接入数据库时，
// 只需新增一个实现同一接口的类型并在装配处替换构造，
// handler 与 service 一行都不用改。
type Repository interface {
	// List 返回全部目的地。数据为空时返回空切片而非 nil，
	// 以免序列化出 `null` 让前端多做一次判空。
	List(ctx context.Context) ([]Destination, error)

	// GetByID 按标识返回单个目的地。
	// 目标不存在时返回包装了 ErrNotFound 的错误。
	GetByID(ctx context.Context, id string) (Destination, error)
}

// MemoryRepository 是基于内存表的 Repository 实现。
//
// 示例数据，非持久化，进程重启即丢失。
// 它存在的意义是让 handler -> service -> repository 这条链路在本变更内
// 被真实跑通，而不是留下两层未被验证的空壳。
type MemoryRepository struct {
	// items 是内存中的目的地表，以 ID 为键。
	items map[string]Destination
	// order 记录插入顺序，使 List 的输出稳定可预期（map 遍历顺序是随机的）。
	order []string
}

// NewMemoryRepository 构造内存仓储，并装载一组示例数据。
func NewMemoryRepository() *MemoryRepository {
	repo := &MemoryRepository{items: make(map[string]Destination)}
	for _, d := range sampleDestinations() {
		repo.items[d.ID] = d
		repo.order = append(repo.order, d.ID)
	}
	return repo
}

// List 返回全部目的地，按示例数据的登记顺序排列。
func (r *MemoryRepository) List(_ context.Context) ([]Destination, error) {
	result := make([]Destination, 0, len(r.order))
	for _, id := range r.order {
		result = append(result, r.items[id])
	}
	return result, nil
}

// GetByID 按标识返回单个目的地；不存在时返回包装了 ErrNotFound 的错误，
// 供上层用 errors.Is 判定并翻译成 404xx 业务码。
func (r *MemoryRepository) GetByID(_ context.Context, id string) (Destination, error) {
	d, ok := r.items[id]
	if !ok {
		return Destination{}, errors.Join(ErrNotFound, errors.New("id="+id))
	}
	return d, nil
}

// sampleDestinations 返回内存仓储装载的示例数据（**非持久化**）。
// 抽取成函数是为了让"仓库构造"与"数据内容"两件事在阅读时分开。
func sampleDestinations() []Destination {
	return []Destination{
		{
			ID:       "beijing",
			Name:     "北京",
			Province: "北京市",
			Summary:  "故宫、长城与胡同，适合首次到访中国的外国旅客。",
		},
		{
			ID:       "xian",
			Name:     "西安",
			Province: "陕西省",
			Summary:  "兵马俑与明城墙，十三朝古都的历史轴线。",
		},
		{
			ID:       "guilin",
			Name:     "桂林",
			Province: "广西壮族自治区",
			Summary:  "漓江山水与阳朔田园，适合慢节奏的自然风光行程。",
		},
	}
}
