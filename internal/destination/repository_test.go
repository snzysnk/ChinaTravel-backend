package destination

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
)

// TestMemoryRepositoryListReturnsAllInOrder 验证 List 返回全部条目且顺序稳定。
// 顺序断言是必要的：map 的遍历顺序随机，若实现漏掉 order 维护，
// 该用例会以"内容相同但顺序漂移"的方式暴露出来。
func TestMemoryRepositoryListReturnsAllInOrder(t *testing.T) {
	repo := NewMemoryRepository()

	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List 返回错误: %v", err)
	}
	if len(got) != len(sampleDestinations()) {
		t.Fatalf("条目数 = %d, 期望 %d", len(got), len(sampleDestinations()))
	}
	for i, want := range sampleDestinations() {
		if got[i] != want {
			t.Errorf("第 %d 项 = %+v, 期望 %+v", i, got[i], want)
		}
	}
}

// TestMemoryRepositoryGetByID 验证按标识查询命中预期条目。
func TestMemoryRepositoryGetByID(t *testing.T) {
	repo := NewMemoryRepository()
	want := sampleDestinations()[0]

	got, err := repo.GetByID(context.Background(), want.ID)
	if err != nil {
		t.Fatalf("GetByID 返回错误: %v", err)
	}
	if got != want {
		t.Errorf("GetByID = %+v, 期望 %+v", got, want)
	}
}

// TestMemoryRepositoryGetByIDNotFound 验证查不到时返回可被 errors.Is 识别的
// ErrNotFound，使服务层能可靠地区分"不存在"与"其他故障"。
func TestMemoryRepositoryGetByIDNotFound(t *testing.T) {
	repo := NewMemoryRepository()

	_, err := repo.GetByID(context.Background(), "not-a-real-id")
	if err == nil {
		t.Fatal("查询不存在的标识应返回错误")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("错误应可用 errors.Is 匹配 ErrNotFound, 实际: %v", err)
	}
}

// TestMemoryRepositoryListReturnsNonNilSlice 验证空数据时返回空切片而非 nil，
// 避免序列化出 `null` 让前端多做一次判空。
func TestMemoryRepositoryListReturnsNonNilSlice(t *testing.T) {
	repo := &MemoryRepository{items: map[string]Destination{}}

	got, err := repo.List(context.Background())
	if err != nil {
		t.Fatalf("List 返回错误: %v", err)
	}
	if got == nil {
		t.Fatal("List 不应返回 nil 切片")
	}

	// 顺带验证它在 JSON 下序列化为 `[]` 而不是 `null`。
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if bytes.Equal(encoded, []byte("null")) {
		t.Errorf("空集合被序列化为 null: %s", encoded)
	}
}
