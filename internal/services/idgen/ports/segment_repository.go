package ports

import (
	"context"

	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"
)

type SegmentAllocator interface {
	AllocateSegmentIDs(ctx context.Context, bizTag string, count int) ([]int64, error)
}

type SegmentRangeRepository interface {
	LoadSegmentRange(ctx context.Context, bizTag string) (domain.SegmentAllocation, error)
}

type BizTagReader interface {
	ListBizTags(ctx context.Context) ([]string, error)
}
