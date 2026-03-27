package commands

import (
	"context"
	"errors"
	"sync"

	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/app/queries"
	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"
	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/ports"
)

type CachedSegmentAllocator struct {
	repository ports.SegmentRangeRepository
	launch     func(func())

	mu      sync.Mutex
	buffers map[string]*domain.SegmentBuffer
}

func NewCachedSegmentAllocator(repository ports.SegmentRangeRepository, launch func(func())) *CachedSegmentAllocator {
	if launch == nil {
		launch = func(fn func()) { go fn() }
	}

	return &CachedSegmentAllocator{
		repository: repository,
		launch:     launch,
		buffers:    make(map[string]*domain.SegmentBuffer),
	}
}

func (a *CachedSegmentAllocator) AllocateSegmentIDs(ctx context.Context, bizTag string, count int) ([]int64, error) {
	buffer, err := a.getOrInitializeBuffer(ctx, bizTag)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, 0, count)
	for i := 0; i < count; i++ {
		id, err := a.nextID(ctx, bizTag, buffer)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, nil
}

func (a *CachedSegmentAllocator) getOrInitializeBuffer(ctx context.Context, bizTag string) (*domain.SegmentBuffer, error) {
	a.mu.Lock()
	buffer := a.buffers[bizTag]
	if buffer == nil {
		buffer = domain.NewSegmentBuffer(bizTag)
		a.buffers[bizTag] = buffer
	}
	a.mu.Unlock()

	if buffer.IsInitialized() {
		return buffer, nil
	}

	allocation, err := a.repository.LoadSegmentRange(ctx, bizTag)
	if err != nil {
		return nil, err
	}
	buffer.InitializeCurrent(allocation)
	return buffer, nil
}

func (a *CachedSegmentAllocator) nextID(ctx context.Context, bizTag string, buffer *domain.SegmentBuffer) (int64, error) {
	result, err := buffer.NextID()
	if err == nil {
		if result.ShouldLoadNext {
			a.preloadNext(ctx, bizTag, buffer)
		}
		return result.ID, nil
	}

	if !errors.Is(err, domain.ErrSegmentsNotReady) {
		return 0, err
	}

	allocation, loadErr := a.repository.LoadSegmentRange(ctx, bizTag)
	if loadErr != nil {
		return 0, loadErr
	}
	buffer.StoreNext(allocation)

	result, err = buffer.NextID()
	if err != nil {
		return 0, err
	}
	if result.ShouldLoadNext {
		a.preloadNext(ctx, bizTag, buffer)
	}
	return result.ID, nil
}

func (a *CachedSegmentAllocator) preloadNext(ctx context.Context, bizTag string, buffer *domain.SegmentBuffer) {
	if !buffer.StartLoadingNext() {
		return
	}

	a.launch(func() {
		allocation, err := a.repository.LoadSegmentRange(ctx, bizTag)
		if err != nil {
			buffer.FinishLoadingNext()
			return
		}
		buffer.StoreNext(allocation)
	})
}

func (a *CachedSegmentAllocator) GetSegmentCacheInfo(bizTag string) (queries.SegmentCacheInfoView, bool) {
	a.mu.Lock()
	buffer := a.buffers[bizTag]
	a.mu.Unlock()
	if buffer == nil {
		return queries.SegmentCacheInfoView{}, false
	}

	snapshot := buffer.Snapshot()
	return queries.SegmentCacheInfoView{
		BizTag:             snapshot.BizTag,
		Initialized:        true,
		Cached:             true,
		BufferInitialized:  boolPtr(snapshot.Initialized),
		CurrentPos:         intPtr(snapshot.CurrentPos),
		NextReady:          boolPtr(snapshot.NextReady),
		LoadingNextSegment: boolPtr(snapshot.LoadingNextSegment),
		MinStep:            intPtr(snapshot.MinStep),
		UpdateTimestamp:    int64Ptr(snapshot.UpdateTimestamp),
		CurrentSegment:     queries.ToSegmentStateView(snapshot.CurrentSegment),
		NextSegment:        queries.ToSegmentStateView(snapshot.NextSegment),
	}, true
}

func boolPtr(v bool) *bool { return &v }

func intPtr(v int) *int { return &v }

func int64Ptr(v int64) *int64 { return &v }
