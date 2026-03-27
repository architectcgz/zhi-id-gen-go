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
