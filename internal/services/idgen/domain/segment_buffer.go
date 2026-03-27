package domain

import (
	"sync"
	"time"
)

const loadThresholdRatio = 0.9

var ErrSegmentsNotReady = &Error{
	Code:    ErrorSegmentsNotReady,
	Message: "Both segments are not ready",
}

type SegmentState struct {
	next int64
	max  int64
	step int
}

type NextIDResult struct {
	ID              int64
	SwitchedSegment bool
	ShouldLoadNext  bool
}

type SegmentStateSnapshot struct {
	Value int64
	Max   int64
	Step  int
	Idle  int64
}

type SegmentCacheSnapshot struct {
	BizTag             string
	Initialized        bool
	CurrentPos         int
	NextReady          bool
	LoadingNextSegment bool
	MinStep            int
	UpdateTimestamp    int64
	CurrentSegment     SegmentStateSnapshot
	NextSegment        SegmentStateSnapshot
}

type SegmentBuffer struct {
	bizTag string

	mu          sync.Mutex
	segments    [2]SegmentState
	currentPos  int
	nextReady   bool
	initialized bool
	loadingNext bool
	disabled    bool
	minStep     int
	updateTs    int64
}

func NewSegmentBuffer(bizTag string) *SegmentBuffer {
	return &SegmentBuffer{bizTag: bizTag}
}

func (b *SegmentBuffer) InitializeCurrent(allocation SegmentAllocation) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.disabled {
		return
	}

	b.segments[b.currentPos] = SegmentState{
		next: allocation.StartID(),
		max:  allocation.MaxID,
		step: allocation.Step,
	}
	b.initialized = true
	b.minStep = allocation.Step
	b.updateTs = time.Now().UnixMilli()
}

func (b *SegmentBuffer) StoreNext(allocation SegmentAllocation) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.disabled {
		return
	}

	nextPos := (b.currentPos + 1) % 2
	b.segments[nextPos] = SegmentState{
		next: allocation.StartID(),
		max:  allocation.MaxID,
		step: allocation.Step,
	}
	if b.minStep == 0 {
		b.minStep = allocation.Step
	}
	b.nextReady = true
	b.loadingNext = false
	b.updateTs = time.Now().UnixMilli()
}

func (b *SegmentBuffer) StartLoadingNext() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.disabled || b.nextReady || b.loadingNext {
		return false
	}
	b.loadingNext = true
	return true
}

func (b *SegmentBuffer) FinishLoadingNext() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.loadingNext = false
}

func (b *SegmentBuffer) IsInitialized() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.initialized
}

func (b *SegmentBuffer) NextID() (NextIDResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.disabled {
		return NextIDResult{}, NewBizTagNotExists(b.bizTag)
	}
	if !b.initialized {
		return NextIDResult{}, ErrSegmentsNotReady
	}

	current := &b.segments[b.currentPos]
	if current.next >= current.max {
		if !b.nextReady {
			return NextIDResult{}, ErrSegmentsNotReady
		}

		b.currentPos = (b.currentPos + 1) % 2
		b.nextReady = false
		current = &b.segments[b.currentPos]
		if current.next >= current.max {
			return NextIDResult{}, ErrSegmentsNotReady
		}

		id := current.next
		current.next++
		return NextIDResult{
			ID:              id,
			SwitchedSegment: true,
			ShouldLoadNext:  b.shouldLoadNextLocked(*current),
		}, nil
	}

	id := current.next
	current.next++
	return NextIDResult{
		ID:              id,
		SwitchedSegment: false,
		ShouldLoadNext:  b.shouldLoadNextLocked(*current),
	}, nil
}

func (b *SegmentBuffer) shouldLoadNextLocked(current SegmentState) bool {
	if b.nextReady || b.loadingNext {
		return false
	}
	idle := current.max - current.next
	return float64(idle) < float64(current.step)*loadThresholdRatio
}

func (b *SegmentBuffer) Snapshot() SegmentCacheSnapshot {
	b.mu.Lock()
	defer b.mu.Unlock()

	current := b.segments[b.currentPos]
	next := b.segments[(b.currentPos+1)%2]

	return SegmentCacheSnapshot{
		BizTag:             b.bizTag,
		Initialized:        b.initialized,
		CurrentPos:         b.currentPos,
		NextReady:          b.nextReady,
		LoadingNextSegment: b.loadingNext,
		MinStep:            b.minStep,
		UpdateTimestamp:    b.updateTs,
		CurrentSegment:     snapshotSegmentState(current),
		NextSegment:        snapshotSegmentState(next),
	}
}

func (b *SegmentBuffer) Deactivate() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.disabled = true
	b.initialized = false
	b.nextReady = false
	b.loadingNext = false
	b.segments = [2]SegmentState{}
}

func snapshotSegmentState(segment SegmentState) SegmentStateSnapshot {
	idle := int64(0)
	if segment.next < segment.max {
		idle = segment.max - segment.next
	}
	return SegmentStateSnapshot{
		Value: segment.next,
		Max:   segment.max,
		Step:  segment.step,
		Idle:  idle,
	}
}
