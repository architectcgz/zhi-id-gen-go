package commands

import (
	"context"
	"testing"

	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"
)

type stubSegmentRangeRepository struct {
	loadRange func(ctx context.Context, bizTag string) (domain.SegmentAllocation, error)
}

func (s stubSegmentRangeRepository) LoadSegmentRange(ctx context.Context, bizTag string) (domain.SegmentAllocation, error) {
	return s.loadRange(ctx, bizTag)
}

func TestCachedSegmentAllocator_CachesCurrentSegmentAndPreloadsNext(t *testing.T) {
	var calls int
	allocator := NewCachedSegmentAllocator(
		stubSegmentRangeRepository{
			loadRange: func(_ context.Context, bizTag string) (domain.SegmentAllocation, error) {
				calls++
				if bizTag != "order" {
					t.Fatalf("unexpected bizTag: %s", bizTag)
				}
				switch calls {
				case 1:
					return domain.SegmentAllocation{BizTag: bizTag, MaxID: 10, Step: 10}, nil
				case 2:
					return domain.SegmentAllocation{BizTag: bizTag, MaxID: 20, Step: 10}, nil
				default:
					return domain.SegmentAllocation{BizTag: bizTag, MaxID: 30, Step: 10}, nil
				}
			},
		},
		func(fn func()) { fn() },
	)

	ids, err := allocator.AllocateSegmentIDs(context.Background(), "order", 12)
	if err != nil {
		t.Fatalf("AllocateSegmentIDs returned error: %v", err)
	}

	if len(ids) != 12 {
		t.Fatalf("len(ids) = %d, want 12", len(ids))
	}
	if ids[0] != 0 || ids[9] != 9 || ids[10] != 10 || ids[11] != 11 {
		t.Fatalf("unexpected ids: %v", ids)
	}
	if calls != 3 {
		t.Fatalf("loadRange calls = %d, want 3", calls)
	}
}

func TestCachedSegmentAllocator_WarmupMarksBizTagAsCachedBeforeFirstAllocation(t *testing.T) {
	allocator := NewCachedSegmentAllocator(
		stubSegmentRangeRepository{
			loadRange: func(_ context.Context, _ string) (domain.SegmentAllocation, error) {
				t.Fatal("LoadSegmentRange should not be called during warmup snapshot")
				return domain.SegmentAllocation{}, nil
			},
		},
		func(fn func()) { fn() },
	)

	allocator.Warmup([]string{"order"})

	info, ok := allocator.GetSegmentCacheInfo("order")
	if !ok {
		t.Fatal("expected warmup bizTag to exist in cache")
	}
	if !allocator.IsInitialized() {
		t.Fatal("allocator should be initialized after warmup")
	}
	if !info.Cached {
		t.Fatal("cached = false, want true")
	}
	if info.BufferInitialized == nil || *info.BufferInitialized {
		t.Fatalf("bufferInitialized = %v, want false", info.BufferInitialized)
	}
}

func TestCachedSegmentAllocator_SyncBizTagsRemovesDeletedTagsFromCacheView(t *testing.T) {
	allocator := NewCachedSegmentAllocator(
		stubSegmentRangeRepository{
			loadRange: func(_ context.Context, bizTag string) (domain.SegmentAllocation, error) {
				return domain.SegmentAllocation{BizTag: bizTag, MaxID: 10, Step: 10}, nil
			},
		},
		func(fn func()) { fn() },
	)

	allocator.Warmup([]string{"order", "user"})
	allocator.SyncBizTags([]string{"order"})

	tags, err := allocator.ListBizTags(context.Background())
	if err != nil {
		t.Fatalf("ListBizTags returned error: %v", err)
	}
	if len(tags) != 1 || tags[0] != "order" {
		t.Fatalf("tags = %v, want [order]", tags)
	}
	if _, ok := allocator.GetSegmentCacheInfo("user"); ok {
		t.Fatal("deleted bizTag should not remain in cache view")
	}
}

func TestCachedSegmentAllocator_DoesNotCacheUnknownBizTagAfterLoadFailure(t *testing.T) {
	allocator := NewCachedSegmentAllocator(
		stubSegmentRangeRepository{
			loadRange: func(_ context.Context, bizTag string) (domain.SegmentAllocation, error) {
				return domain.SegmentAllocation{}, domain.NewBizTagNotExists(bizTag)
			},
		},
		func(fn func()) { fn() },
	)

	if _, err := allocator.AllocateSegmentIDs(context.Background(), "missing", 1); err == nil {
		t.Fatal("AllocateSegmentIDs returned nil error, want bizTag not exists")
	}

	tags, err := allocator.ListBizTags(context.Background())
	if err != nil {
		t.Fatalf("ListBizTags returned error: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("tags = %v, want empty", tags)
	}
}
