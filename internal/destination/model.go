package destination

// Destination 是旅行目的地。
//
// 注意：本包当前的数据全部来自内存仓储中的**示例数据，非持久化，
// 进程重启即丢失**。它在本变更中只承担"打通 handler -> service -> repository
// 全链路"的载体作用，不代表最终的业务模型；字段与取值随后续业务变更演进。
type Destination struct {
	// ID 是目的地的稳定标识，用于按标识查询与前端列表项的 key。
	ID string `json:"id"`
	// Name 是目的地名称。
	Name string `json:"name"`
	// Province 是所属省级行政区。
	Province string `json:"province"`
	// Summary 是一句话简介，用于前端列表项的辅助说明。
	Summary string `json:"summary"`
}
