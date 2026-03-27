package commands

import (
	"context"

	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"
	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/ports"
)

const (
	maxBatchCount   = 1000
	maxBizTagLength = 128
)

type SegmentCommandService struct {
	allocator ports.SegmentAllocator
}

func NewSegmentCommandService(allocator ports.SegmentAllocator) SegmentCommandService {
	return SegmentCommandService{allocator: allocator}
}

func (s SegmentCommandService) GenerateSegmentID(ctx context.Context, bizTag string) (int64, error) {
	if err := validateBizTag(bizTag); err != nil {
		return 0, err
	}

	ids, err := s.allocator.AllocateSegmentIDs(ctx, bizTag, 1)
	if err != nil {
		return 0, err
	}
	if len(ids) != 1 {
		return 0, domain.NewInvalidArgument("Segment allocator must return exactly one ID")
	}

	return ids[0], nil
}

func (s SegmentCommandService) GenerateBatchSegmentIDs(ctx context.Context, bizTag string, count int) ([]int64, error) {
	if err := validateBizTag(bizTag); err != nil {
		return nil, err
	}
	if err := validateBatchCount(count); err != nil {
		return nil, err
	}

	return s.allocator.AllocateSegmentIDs(ctx, bizTag, count)
}

func ValidateBatchCountError() error {
	return domain.NewInvalidArgument("Count must be greater than 0")
}

func validateBizTag(bizTag string) error {
	if bizTag == "" {
		return domain.NewInvalidArgument("BizTag cannot be null or empty")
	}
	if len(bizTag) > maxBizTagLength {
		return domain.NewInvalidArgument("BizTag length must not exceed 128 characters")
	}
	return nil
}

func validateBatchCount(count int) error {
	if count <= 0 {
		return domain.NewInvalidArgument("Count must be greater than 0")
	}
	if count > maxBatchCount {
		return domain.NewInvalidArgument("Count must not exceed 1000")
	}
	return nil
}
